package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateSemanticSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		emb1     []float32
		emb2     []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			emb1:     []float32{1.0, 0.0, 0.0},
			emb2:     []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			emb1:     []float32{1.0, 0.0, 0.0},
			emb2:     []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			emb1:     []float32{1.0, 0.0, 0.0},
			emb2:     []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			emb1:     []float32{1.0, 0.0},
			emb2:     []float32{1.0, 0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "empty vectors",
			emb1:     []float32{},
			emb2:     []float32{},
			expected: 0.0,
		},
		{
			name:     "normalized vectors cosine",
			emb1:     []float32{0.5, 0.5, 0.5, 0.5}, // length = 1
			emb2:     []float32{0.5, 0.5, 0.5, 0.5},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSemanticSimilarity(tt.emb1, tt.emb2)
			assert.InDelta(t, tt.expected, got, 0.0001)
		})
	}
}

