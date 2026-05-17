package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   60 * time.Second,
		},
		logger: logger,
	}
}

type IdentityEmbeddingRequest struct {
	ImageBase64 string `json:"image_base64"`
	Name        string `json:"name"`
}

type IdentityEmbeddingResponse struct {
	FaceEmbedding []float32 `json:"face_embedding"`
	NameEmbedding []float32 `json:"name_embedding"`
}

func (p *PythonClient) GenerateIdentityEmbeddings(ctx context.Context, photoBase64 string, name string) ([]float32, []float32, error) {
	reqBody := IdentityEmbeddingRequest{
		ImageBase64: photoBase64,
		Name:        name,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/represent-identity", p.baseURL), bytes.NewBuffer(b))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call AI service (/represent-identity): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Detail string `json:"detail"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, nil, fmt.Errorf("AI service returned error (%d): %s", resp.StatusCode, errResp.Detail)
	}

	var result IdentityEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return normalize(result.FaceEmbedding), normalize(result.NameEmbedding), nil
}
