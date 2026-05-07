package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type PythonClient struct {
	baseURL string
	client  *http.Client
	logger  *zerolog.Logger
}

func NewPythonClient(baseURL string, logger *zerolog.Logger) *PythonClient {
	return &PythonClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

type EmbeddingRequest struct {
	ImageBase64 string `json:"image_base64"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (p *PythonClient) GenerateFaceEmbedding(photoBase64 string) ([]float32, error) {
	reqBody := EmbeddingRequest{
		ImageBase64: photoBase64,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := p.client.Post(fmt.Sprintf("%s/represent", p.baseURL), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("failed to call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Detail string `json:"detail"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("AI service returned error (%d): %s", resp.StatusCode, errResp.Detail)
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return normalize(result.Embedding), nil
}

func (p *PythonClient) GenerateNameEmbedding(name string) ([]float32, error) {
	// Character frequency vector as implemented in onnx.go previously
	embedding := make([]float32, 768)
	for _, char := range name {
		idx := int(char) % 768
		embedding[idx] += 1.0
	}
	return normalize(embedding), nil
}
