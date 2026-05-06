package embedding

import (
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/vk1033/goverify-engine/internal/config"
)

func TestFaceEmbeddingAccuracy(t *testing.T) {
	cfg, _ := config.LoadConfig()
	// Ensure these paths are correct for the test environment
	cfg.AI.LibraryPath = "/usr/local/lib/libonnxruntime.so"
	cfg.AI.FaceModelPath = "/app/models/face.onnx"

	svc, err := NewRealService(cfg)
	if err != nil {
		t.Skipf("Skipping accuracy test because AI models are not available: %v", err)
	}
	defer svc.(interface{ Close() error }).Close()

	// 1. Load Images
	img1a := getBase64("../../person1_a.png")
	img1b := getBase64("../../person1_b.png")
	img2 := getBase64("../../person2.png")

	// 2. Generate Embeddings
	emb1a, _ := svc.GenerateFaceEmbedding(img1a)
	emb1b, _ := svc.GenerateFaceEmbedding(img1b)
	emb2, _ := svc.GenerateFaceEmbedding(img2)

	// 3. Calculate Similarities
	simPos := cosineSimilarity(emb1a, emb1b)
	simNeg := cosineSimilarity(emb1a, emb2)

	fmt.Printf("\n--- AI Accuracy Test Results ---\n")
	fmt.Printf("Positive Similarity (Same Person): %.4f\n", simPos)
	fmt.Printf("Negative Similarity (Different Person): %.4f\n", simNeg)
	fmt.Printf("Separation Margin: %.4f\n", simPos-simNeg)

	if simPos < 0.80 {
		t.Errorf("Positive similarity too low: %.4f", simPos)
	}
	if simNeg > 0.60 {
		t.Errorf("Negative similarity too high: %.4f", simNeg)
	}
	if simPos <= simNeg {
		t.Errorf("AI failed to distinguish between people!")
	}
}

func getBase64(path string) string {
	data, _ := os.ReadFile(path)
	return base64.StdEncoding.EncodeToString(data)
}

func cosineSimilarity(v1, v2 []float32) float32 {
	var dot, norm1, norm2 float32
	for i := range v1 {
		dot += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}
	return dot / (float32(math.Sqrt(float64(norm1))) * float32(math.Sqrt(float64(norm2))))
}
