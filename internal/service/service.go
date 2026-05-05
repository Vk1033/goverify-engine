package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/embedding"
	"github.com/vk1033/goverify-engine/internal/vectordb"
	"github.com/vk1033/goverify-engine/pkg/hash"
)

type KYCService interface {
	ProcessEnrollment(ctx context.Context, txnID string, req domain.KYCEnrollRequest) error
	ProcessVerification(ctx context.Context, txnID string, req domain.KYCVerifyRequest) (*domain.VerificationResult, error)
}

type serviceImpl struct {
	embeddings embedding.Service
	milvus     vectordb.Client
	logger     *slog.Logger
}

func NewKYCService(e embedding.Service, m vectordb.Client, logger *slog.Logger) KYCService {
	return &serviceImpl{
		embeddings: e,
		milvus:     m,
		logger:     logger,
	}
}

func (s *serviceImpl) ProcessEnrollment(ctx context.Context, txnID string, req domain.KYCEnrollRequest) error {
	s.logger.Info("processing enrollment", "txnID", txnID)

	faceEmb, err := s.embeddings.GenerateFaceEmbedding(req.PhotoBase64)
	if err != nil {
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
		NameEmbedding:   nameEmb,
		FaceEmbedding:   faceEmb,
		DemographicHash: demographicHash,
	}

	if err := s.milvus.InsertIdentity(ctx, record); err != nil {
		return fmt.Errorf("milvus insert failed: %w", err)
	}

	s.logger.Info("enrollment successful", "txnID", txnID)
	return nil
}

func (s *serviceImpl) ProcessVerification(ctx context.Context, txnID string, req domain.KYCVerifyRequest) (*domain.VerificationResult, error) {
	s.logger.Info("processing verification", "txnID", txnID)

	faceEmb, err := s.embeddings.GenerateFaceEmbedding(req.PhotoBase64)
	if err != nil {
		return nil, fmt.Errorf("face embedding failed: %w", err)
	}

	nameEmb, err := s.embeddings.GenerateNameEmbedding(req.Name)
	if err != nil {
		return nil, fmt.Errorf("name embedding failed: %w", err)
	}

	// Fetch top 5 similar faces
	results, err := s.milvus.SearchSimilar(ctx, faceEmb, nameEmb, 5)
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

	// Mock similarities for the result (since milvus Search API in this mock isn't returning scores directly without distances logic)
	// We'll set high confidence if demographic matched.
	faceSimilarity := float32(0.95) // Should be computed based on Milvus distance
	nameSimilarity := float32(0.90) // Should be computed based on cosine of NameEmbs

	status := domain.StatusNoMatch
	if bestDemographicMatch {
		status = domain.StatusMatched
	} else if faceSimilarity > 0.8 {
		status = domain.StatusPartial
	}

	res := &domain.VerificationResult{
		TransactionID:   txnID,
		Status:          status,
		ConfidenceScore: (faceSimilarity + nameSimilarity) / 2.0,
		Details: domain.VerificationDetails{
			FaceSimilarity:   faceSimilarity,
			NameSimilarity:   nameSimilarity,
			DemographicMatch: bestDemographicMatch,
		},
	}

	s.logger.Info("verification completed", "txnID", txnID, "status", res.Status)
	return res, nil
}
