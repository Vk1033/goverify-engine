package domain

import "time"

// KYCRequest represents the input for /kyc/enroll and /kyc/verify
type KYCRequest struct {
	PhotoBase64 string `json:"photo_base64" binding:"required"`
	Name        string `json:"name" binding:"required"`
	DOB         string `json:"dob" binding:"required" example:"1990-01-01"`
	Gender      string `json:"gender" binding:"required" example:"FEMALE"`
	CallbackURL string `json:"callback_url,omitempty" example:"http://client-service.local/webhook"`
}

// TransactionStatus represents the status of an async transaction
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "PENDING"
	StatusMatched   TransactionStatus = "MATCHED"
	StatusPartial   TransactionStatus = "PARTIAL_MATCH"
	StatusNoMatch   TransactionStatus = "NO_MATCH"
	StatusError     TransactionStatus = "ERROR"
	StatusSuccess   TransactionStatus = "SUCCESS"
)

// AsyncResponse is the immediate response given to the client
type AsyncResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// VerificationResult is the final output of the verification process
type VerificationResult struct {
	TransactionID   string            `json:"transaction_id"`
	Status          TransactionStatus `json:"status"`
	ConfidenceScore float32           `json:"confidence_score"`
	Details         VerificationDetails `json:"details"`
	CreatedAt       time.Time         `json:"created_at"`
}

type VerificationDetails struct {
	FaceSimilarity   float32 `json:"face_similarity"`
	NameSimilarity   float32 `json:"name_similarity"`
	DemographicMatch bool    `json:"demographic_match"`
	Explanation      string  `json:"explanation,omitempty"`
}

// IdentityRecord is what we store in the Vector DB / Metadata store
type IdentityRecord struct {
	ID              int64     `json:"-"`
	TransactionID   string    `json:"transaction_id"`
	Name            string    `json:"name"`
	NameBlindIndex  string    `json:"-"`
	DOB             string    `json:"dob"`
	Gender          string    `json:"gender"`
	NameEmbedding   []float32 `json:"-"`
	FaceEmbedding   []float32 `json:"-"`
	DemographicHash string    `json:"demographic_hash"`
	Score           float32   `json:"score,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// AuthRequest represents the login request
type AuthRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// AuthResponse represents the login response with the JWT token
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type" example:"Bearer"`
	ExpiresIn   int64  `json:"expires_in" example:"3600"`
}
