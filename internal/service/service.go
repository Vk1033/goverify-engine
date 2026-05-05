package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/embedding"
	"github.com/vk1033/goverify-engine/internal/observability"
	"github.com/vk1033/goverify-engine/internal/vectordb"
	"github.com/vk1033/goverify-engine/pkg/hash"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("kyc-service")

type KYCService interface {
	ProcessEnrollment(ctx context.Context, txnID string, req domain.KYCRequest) error
	ProcessVerification(ctx context.Context, txnID string, req domain.KYCRequest) (*domain.VerificationResult, error)
	SearchIdentities(ctx context.Context, name string, gender string) ([]*domain.IdentityRecord, error)
}

const (
	// Multi-modal Scoring Weights
	WeightFace        = 0.50 // 50% Face Biometric
	WeightName        = 0.30 // 30% Name Embedding
	WeightDemographic = 0.20 // 20% Demographic Hash Match

	// Thresholds
	ThresholdMatch   = 0.85
	ThresholdPartial = 0.70
)

type serviceImpl struct {
	embeddings embedding.Service
	milvus     vectordb.Client
	logger     *zerolog.Logger
}

func NewKYCService(e embedding.Service, m vectordb.Client, logger *zerolog.Logger) KYCService {
	return &serviceImpl{
		embeddings: e,
		milvus:     m,
		logger:     logger,
	}
}

func (s *serviceImpl) ProcessEnrollment(ctx context.Context, txnID string, req domain.KYCRequest) error {
	ctx, span := tracer.Start(ctx, "KYCService.ProcessEnrollment")
	defer span.End()
	span.SetAttributes(attribute.String("txn_id", txnID), attribute.String("user_name", req.Name))

	s.logger.Info().Ctx(ctx).Str("txnID", txnID).Msg("processing enrollment")

	faceEmb, err := s.embeddings.GenerateFaceEmbedding(req.PhotoBase64)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("face embedding failed: %w", err)
	}

	nameEmb, err := s.embeddings.GenerateNameEmbedding(req.Name)
	if err != nil {
		return fmt.Errorf("name embedding failed: %w", err)
	}

	demographicHash, err := hash.GenerateDemographicHash(req.DOB, req.Gender, hash.DefaultConfig)
	if err != nil {
		return fmt.Errorf("hashing failed: %w", err)
	}

	record := &domain.IdentityRecord{
		TransactionID:   txnID,
		Name:            req.Name,
		DOB:             req.DOB,
		Gender:          req.Gender,
		NameEmbedding:   nameEmb,
		FaceEmbedding:   faceEmb,
		DemographicHash: demographicHash,
	}

	if err := s.milvus.InsertIdentity(ctx, record); err != nil {
		return fmt.Errorf("milvus insert failed: %w", err)
	}

	s.logger.Info().Ctx(ctx).Str("txnID", txnID).Msg("enrollment successful")
	return nil
}

func (s *serviceImpl) ProcessVerification(ctx context.Context, txnID string, req domain.KYCRequest) (*domain.VerificationResult, error) {
	start := time.Now()
	defer func() {
		observability.KycVerifyLatencyMs.Observe(float64(time.Since(start).Milliseconds()))
	}()

	ctx, span := tracer.Start(ctx, "KYCService.ProcessVerification")
	defer span.End()
	span.SetAttributes(attribute.String("txn_id", txnID), attribute.String("user_name", req.Name))

	s.logger.Info().Ctx(ctx).Str("txnID", txnID).Msg("processing verification")

	faceEmb, err := s.embeddings.GenerateFaceEmbedding(req.PhotoBase64)
	if err != nil {
		return nil, fmt.Errorf("face embedding failed: %w", err)
	}

	nameEmb, err := s.embeddings.GenerateNameEmbedding(req.Name)
	if err != nil {
		return nil, fmt.Errorf("name embedding failed: %w", err)
	}

	// Fetch top 5 similar faces
	searchStart := time.Now()
	results, err := s.milvus.SearchSimilar(ctx, faceEmb, nameEmb, 5)
	observability.VectorSearchLatencyMs.Observe(float64(time.Since(searchStart).Milliseconds()))
	if err != nil {
		return nil, fmt.Errorf("milvus search failed: %w", err)
	}

	if len(results) == 0 {
		return &domain.VerificationResult{
			TransactionID: txnID,
			Status:        domain.StatusNoMatch,
			Details:       domain.VerificationDetails{},
		}, nil
	}

	// For simplicity in the hackathon, we assume the top result is our target if it passes thresholds.
	// In reality we should compare all results to find the best match that ALSO matches demographic hash.
	var bestMatch *domain.IdentityRecord
	var bestDemographicMatch bool

	for _, res := range results {
		match, err := hash.CompareDemographicHash(req.DOB, req.Gender, res.DemographicHash)
		if err == nil && match {
			bestMatch = res
			bestDemographicMatch = true
			break
		}
	}

	if bestMatch == nil {
		bestMatch = results[0] // fallback to highest face similarity if demographic doesn't match
	}

	// Calculate similarities from L2 distance (Milvus returns L2 squared for some indexes, or just L2)
	// For HNSW with L2 on normalized vectors: dist = 2*(1-cos)
	// So cos = 1 - (dist/2)
	faceSimilarity := 1.0 - (float64(bestMatch.Score) / 2.0)
	if faceSimilarity < 0 {
		faceSimilarity = 0
	}

	// Since we are using a combined vector for search, the 'Score' is the combined distance.
	// However, for "Explainable" scoring, we want to show individual contributions.
	// In a real system, we'd perform separate searches or rerank. 
	// For this hackathon, we'll treat the search result score as the primary face similarity
	// and assume name similarity is roughly the same for the top match (or slightly lower).
	nameSimilarity := faceSimilarity * 0.95 // heuristic for mock
	if !bestDemographicMatch {
		nameSimilarity = faceSimilarity * 0.5 // penalize if demographic doesn't match
	}

	// Calculate Weighted Score
	demoScore := 0.0
	if bestDemographicMatch {
		demoScore = 1.0
	}

	finalScore := (faceSimilarity * WeightFace) + (nameSimilarity * WeightName) + (demoScore * WeightDemographic)

	status := domain.StatusNoMatch
	if finalScore >= ThresholdMatch {
		status = domain.StatusMatched
	} else if finalScore >= ThresholdPartial {
		status = domain.StatusPartial
	}

	explanation := fmt.Sprintf(
		"Combined Score: %.2f (Face: %.2f * %.1f + Name: %.2f * %.1f + Demo: %.1f * %.1f). Threshold for MATCH: %.2f",
		finalScore, faceSimilarity, WeightFace, nameSimilarity, WeightName, demoScore, WeightDemographic, ThresholdMatch,
	)

	res := &domain.VerificationResult{
		TransactionID:   txnID,
		Status:          status,
		ConfidenceScore: float32(finalScore),
		Details: domain.VerificationDetails{
			FaceSimilarity:   float32(faceSimilarity),
			NameSimilarity:   float32(nameSimilarity),
			DemographicMatch: bestDemographicMatch,
			Explanation:      explanation,
		},
		CreatedAt: time.Now(),
	}
	s.logger.Info().Ctx(ctx).Str("txnID", txnID).Str("status", string(res.Status)).Float64("final_score", finalScore).Msg("verification completed")
	return res, nil
}

func (s *serviceImpl) SearchIdentities(ctx context.Context, name string, gender string) ([]*domain.IdentityRecord, error) {
	ctx, span := tracer.Start(ctx, "KYCService.SearchIdentities")
	defer span.End()
	span.SetAttributes(attribute.String("name", name), attribute.String("gender", gender))

	s.logger.Info().Ctx(ctx).Str("name", name).Str("gender", gender).Msg("searching identities")
	return s.milvus.QueryIdentities(ctx, name, gender)
}
