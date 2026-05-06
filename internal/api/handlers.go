package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/kafka"
	"github.com/vk1033/goverify-engine/internal/service"
)

type Handler struct {
	producer kafka.Producer
	service    service.KYCService
	redis      *redis.Client
	jwtManager *JWTManager
	logger     *zerolog.Logger
}

func NewHandler(p kafka.Producer, s service.KYCService, r *redis.Client, j *JWTManager, l *zerolog.Logger) *Handler {
	return &Handler{
		producer:   p,
		service:    s,
		redis:      r,
		jwtManager: j,
		logger:     l,
	}
}

// Enroll godoc
// @Summary      KYC Enrollment
// @Description  Creates a unique identity signature for the user and stores demographic metadata.
// @Tags         kyc
// @Accept       json
// @Produce      json
// @Param        request  body      domain.KYCRequest  true  "Enrollment Data"
// @Success      202      {object}  domain.AsyncResponse
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /kyc/enroll [post]
// @Security     Bearer
func (h *Handler) Enroll(c *gin.Context) {
	var req domain.KYCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnID := "txn_" + uuid.New().String()

	// Push to Kafka for async processing
	err := h.producer.PublishEnrollment(c.Request.Context(), txnID, req)
	if err != nil {
		h.logger.Error().Err(err).Str("txnID", txnID).Msg("Failed to publish enrollment")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue request"})
		return
	}

	// Save pending status to Redis
	err = h.redis.Set(c.Request.Context(), txnID, string(domain.StatusPending), 24*time.Hour).Err()
	if err != nil {
		h.logger.Error().Err(err).Str("txnID", txnID).Msg("Failed to set status in redis")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save status"})
		return
	}

	c.JSON(http.StatusAccepted, domain.AsyncResponse{
		TransactionID: txnID,
		Status:        string(domain.StatusPending),
		Message:       "Enrollment request queued",
	})
}

// Verify godoc
// @Summary      Re-KYC Verification
// @Description  Performs similarity-based lookup for returning users using facial embeddings and demographic matching.
// @Tags         kyc
// @Accept       json
// @Produce      json
// @Param        request  body      domain.KYCRequest  true  "Verification Data"
// @Success      202      {object}  domain.AsyncResponse
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /kyc/verify [post]
// @Security     Bearer
func (h *Handler) Verify(c *gin.Context) {
	var req domain.KYCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnID := "txn_" + uuid.New().String()

	err := h.producer.PublishVerification(c.Request.Context(), txnID, req)
	if err != nil {
		h.logger.Error().Err(err).Str("txnID", txnID).Msg("Failed to publish verification")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue request"})
		return
	}

	err = h.redis.Set(c.Request.Context(), txnID, string(domain.StatusPending), 24*time.Hour).Err()
	if err != nil {
		h.logger.Error().Err(err).Str("txnID", txnID).Msg("Failed to set status in redis")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save status"})
		return
	}

	c.JSON(http.StatusAccepted, domain.AsyncResponse{
		TransactionID: txnID,
		Status:        string(domain.StatusPending),
		Message:       "Verification request queued",
	})
}

// Status godoc
// @Summary      Get Transaction Status
// @Description  Retrieves the status or result of an asynchronous KYC transaction.
// @Tags         kyc
// @Produce      json
// @Param        transaction_id  path      string  true  "Transaction ID"
// @Success      200             {object}  domain.VerificationResult
// @Failure      404             {object}  map[string]string
// @Failure      500             {object}  map[string]string
// @Router       /kyc/status/{transaction_id} [get]
// @Security     Bearer
func (h *Handler) Status(c *gin.Context) {
	txnID := c.Param("transaction_id")

	val, err := h.redis.Get(c.Request.Context(), txnID).Result()
	if err == redis.Nil {
		h.logger.Warn().Str("txnID", txnID).Msg("Transaction not found in redis")
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	} else if err != nil {
		h.logger.Error().Err(err).Str("txnID", txnID).Msg("Redis error during status check")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Try to unmarshal if it's a JSON VerificationResult, else just return the raw status string
	var res domain.VerificationResult
	if err := json.Unmarshal([]byte(val), &res); err == nil {
		c.JSON(http.StatusOK, res)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction_id": txnID,
		"status":         val,
	})
}

// Search godoc
// @Summary      Search Identities
// @Description  Searches for identity records based on name and/or gender metadata.
// @Tags         kyc
// @Produce      json
// @Param        name    query     string  false  "Name"
// @Param        gender  query     string  false  "Gender"
// @Success      200     {array}   domain.IdentityRecord
// @Failure      500     {object}  map[string]string
// @Router       /kyc/search [get]
// @Security     Bearer
func (h *Handler) Search(c *gin.Context) {
	name := c.Query("name")
	gender := c.Query("gender")

	results, err := h.service.SearchIdentities(c.Request.Context(), name, gender)
	if err != nil {
		h.logger.Error().Err(err).Msg("Search failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	c.JSON(http.StatusOK, results)
}

// Login handles the POST /auth/login endpoint
// @Summary      Login
// @Description  Exchanges credentials for a JWT access token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      domain.AuthRequest  true  "Login Credentials"
// @Success      200      {object}  domain.AuthResponse
// @Failure      401      {object}  map[string]string
// @Router       /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req domain.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Simple credential check
	// In a production app, verify against a database with hashed passwords
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := h.jwtManager.Generate(req.Username)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to generate token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, domain.AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	})
}

// Health godoc
// @Summary      Health Check
// @Description  Returns the current health status of the service.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}
