package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/embedding"
	"github.com/vk1033/goverify-engine/internal/vectordb"
	"github.com/vk1033/goverify-engine/pkg/hash"
)

type KYCService interface {
	ProcessEnrollment(ctx context.Context, txnID string, req domain.KYCEnrollRequest) error
	ProcessVerification(ctx context.Context, txnID string, req domain.KYCVerifyRequest) (*domain.VerificationResult, error)
	SearchIdentities(ctx context.Context, name string, gender string) ([]*domain.IdentityRecord, error)
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

	// Calculate similarity from L2 distance. 
	// For normalized vectors, L2 squared is 2(1-cos). 
	// Here we use a simpler heuristic for the hackathon: 1.0 - (dist / max_expected_dist)
	distance := bestMatch.Score
	similarity := 1.0 - (float64(distance) / 2.0)
	if similarity < 0 {
		similarity = 0
	}

	status := domain.StatusNoMatch
	if bestDemographicMatch && similarity > 0.8 {
		status = domain.StatusMatched
	} else if similarity > 0.7 {
		status = domain.StatusPartial
	}

	res := &domain.VerificationResult{
		TransactionID:   txnID,
		Status:          status,
		ConfidenceScore: float32(similarity),
		Details: domain.VerificationDetails{
			FaceSimilarity:   float32(similarity),
			NameSimilarity:   float32(similarity),
			DemographicMatch: bestDemographicMatch,
		},
		CreatedAt: time.Now(),
	}
	s.logger.Info("verification completed", "txnID", txnID, "status", res.Status)
	return res, nil
}

func (s *serviceImpl) SearchIdentities(ctx context.Context, name string, gender string) ([]*domain.IdentityRecord, error) {
	s.logger.Info("searching identities", "name", name, "gender", gender)
	return s.milvus.QueryIdentities(ctx, name, gender)
}
