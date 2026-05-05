package domain

import "time"

// KYCEnrollRequest represents the input for /kyc/enroll
type KYCEnrollRequest struct {
	PhotoBase64 string `json:"photo_base64" binding:"required"`
	Name        string `json:"name" binding:"required"`
	DOB         string `json:"dob" binding:"required"`    // e.g. YYYY-MM-DD
	Gender      string `json:"gender" binding:"required"` // e.g. MALE, FEMALE, OTHER
}

// KYCVerifyRequest represents the input for /kyc/verify
type KYCVerifyRequest struct {
	PhotoBase64 string `json:"photo_base64" binding:"required"`
	Name        string `json:"name" binding:"required"`
	DOB         string `json:"dob" binding:"required"`
	Gender      string `json:"gender" binding:"required"`
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
}

// IdentityRecord is what we store in the Vector DB / Metadata store
type IdentityRecord struct {
	ID              int64     `json:"-"`
	TransactionID   string    `json:"transaction_id"`
	Name            string    `json:"name"`
	DOB             string    `json:"dob"`
	Gender          string    `json:"gender"`
	NameEmbedding   []float32 `json:"-"`
	FaceEmbedding   []float32 `json:"-"`
	DemographicHash string    `json:"demographic_hash"`
	Score           float32   `json:"score,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
