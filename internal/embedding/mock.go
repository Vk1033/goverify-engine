package embedding

import (
	"crypto/sha256"
	"math/rand"
	"time"
)

type MockService struct{}

func NewMockService() Service {
	return &MockService{}
}

// GenerateFaceEmbedding generates a 512D deterministic-ish embedding based on the hash of the photo.
func (s *MockService) GenerateFaceEmbedding(photoBase64 string) ([]float32, error) {
	time.Sleep(100 * time.Millisecond) // Simulate processing time

	hash := sha256.Sum256([]byte(photoBase64))
	r := rand.New(rand.NewSource(int64(hash[0]) | int64(hash[1])<<8 | int64(hash[2])<<16))

	embedding := make([]float32, 512)
	for i := 0; i < 512; i++ {
		embedding[i] = r.Float32()
	}

	return normalize(embedding), nil
}

// GenerateNameEmbedding generates a 768D deterministic-ish embedding based on the hash of the name.
func (s *MockService) GenerateNameEmbedding(name string) ([]float32, error) {
	time.Sleep(50 * time.Millisecond) // Simulate processing time

	hash := sha256.Sum256([]byte(name))
	r := rand.New(rand.NewSource(int64(hash[0]) | int64(hash[1])<<8 | int64(hash[2])<<16))

	embedding := make([]float32, 768)
	for i := 0; i < 768; i++ {
		embedding[i] = r.Float32()
	}

	return normalize(embedding), nil
}
