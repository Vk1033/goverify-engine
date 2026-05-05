package api

import (

	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/kafka"
)

type Handler struct {
	producer kafka.Producer
	redis    *redis.Client
}

func NewHandler(p kafka.Producer, r *redis.Client) *Handler {
	return &Handler{
		producer: p,
		redis:    r,
	}
}

// Enroll handles the POST /kyc/enroll endpoint
func (h *Handler) Enroll(c *gin.Context) {
	var req domain.KYCEnrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnID := "txn_" + uuid.New().String()

	// Push to Kafka
	err := h.producer.PublishEnrollment(c.Request.Context(), txnID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue request"})
		return
	}

	// Save pending status to Redis
	h.redis.Set(c.Request.Context(), txnID, domain.StatusPending, 24*time.Hour)

	c.JSON(http.StatusAccepted, domain.AsyncResponse{
		TransactionID: txnID,
		Status:        string(domain.StatusPending),
		Message:       "Enrollment request queued",
	})
}

// Verify handles the POST /kyc/verify endpoint
func (h *Handler) Verify(c *gin.Context) {
	var req domain.KYCVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnID := "txn_" + uuid.New().String()

	err := h.producer.PublishVerification(c.Request.Context(), txnID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue request"})
		return
	}

	h.redis.Set(c.Request.Context(), txnID, domain.StatusPending, 24*time.Hour)

	c.JSON(http.StatusAccepted, domain.AsyncResponse{
		TransactionID: txnID,
		Status:        string(domain.StatusPending),
		Message:       "Verification request queued",
	})
}

// Status handles the GET /kyc/status/:transaction_id endpoint
func (h *Handler) Status(c *gin.Context) {
	txnID := c.Param("transaction_id")

	val, err := h.redis.Get(c.Request.Context(), txnID).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	} else if err != nil {
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
