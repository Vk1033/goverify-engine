package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/embedding"
	"github.com/vk1033/goverify-engine/internal/observability"
	"github.com/vk1033/goverify-engine/internal/vectordb"
	"github.com/vk1033/goverify-engine/pkg/crypto"
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
	WeightFace        = 0.70 // 70% Face Biometric
	WeightName        = 0.20 // 20% Name Similarity
	WeightDemographic = 0.10 // 10% Demographic Hash Match

	// Thresholds
	ThresholdMatch   = 0.85
	ThresholdPartial = 0.70
)

type serviceImpl struct {
	embeddings embedding.Service
	milvus     vectordb.Client
	cfg        *config.Config
	logger     *zerolog.Logger
}

func NewKYCService(e embedding.Service, m vectordb.Client, cfg *config.Config, logger *zerolog.Logger) KYCService {
	return &serviceImpl{
		embeddings: e,
		milvus:     m,
		cfg:        cfg,
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

	// Encrypt PII
	encName, err := crypto.Encrypt(req.Name, s.cfg.Security.AESKey)
	if err != nil {
		return fmt.Errorf("name encryption failed: %w", err)
	}
	encDOB, err := crypto.Encrypt(req.DOB, s.cfg.Security.AESKey)
	if err != nil {
		return fmt.Errorf("dob encryption failed: %w", err)
	}

	// Generate blind index for searching (SHA-256 of normalized name)
	nameHash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(req.Name))))
	blindIndex := hex.EncodeToString(nameHash[:])

	record := &domain.IdentityRecord{
		TransactionID:   txnID,
		Name:            encName,
		NameBlindIndex:  blindIndex,
		DOB:             encDOB,
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

	var bestMatch *domain.IdentityRecord
	var bestDemographicMatch bool

	for _, res := range results {
		// Decrypt name and dob for matching/logic if needed
		decName, err := crypto.Decrypt(res.Name, s.cfg.Security.AESKey)
		if err != nil {
			s.logger.Error().Err(err).Str("txnID", txnID).Msg("Failed to decrypt name from DB")
			continue
		}
		res.Name = decName

		decDOB, err := crypto.Decrypt(res.DOB, s.cfg.Security.AESKey)
		if err != nil {
			s.logger.Error().Err(err).Str("txnID", txnID).Msg("Failed to decrypt DOB from DB")
			continue
		}
		res.DOB = decDOB

		match, err := hash.CompareDemographicHash(req.DOB, req.Gender, res.DemographicHash)
		if err == nil && match {
			bestMatch = res
			bestDemographicMatch = true
			break
		}
	}

	if bestMatch == nil {
		bestMatch = results[0]
	}

	// Calculate similarities from L2 distance
	// For L2 distance on normalized vectors: dist^2 = 2 - 2*cos_sim => cos_sim = 1 - (dist^2 / 2)
	dist := float64(bestMatch.Score)
	faceSimilarity := 1.0 - (dist * dist / 2.0)
	if faceSimilarity < 0 {
		faceSimilarity = 0
	}

	// Real string similarity for names (Levenshtein-based or simple overlap)
	nameSimilarity := calculateStringSimilarity(req.Name, bestMatch.Name)

	demoScore := 0.0
	if bestDemographicMatch {
		demoScore = 1.0
	}

	finalScore := (faceSimilarity * WeightFace) + (nameSimilarity * WeightName) + (demoScore * WeightDemographic)

	// Biometric Safety Threshold: If face similarity is extremely low, it's a mismatch regardless of other data.
	if faceSimilarity < 0.4 {
		finalScore *= 0.5 // Heavy penalty for poor biometric match
	}

	status := domain.StatusNoMatch
	if finalScore >= ThresholdMatch {
		status = domain.StatusMatched
	} else if finalScore >= ThresholdPartial {
		status = domain.StatusPartial
	}

	explanation := fmt.Sprintf(
		"Biometric Match: %.2f (Face: %.2f * %.1f + Name: %.2f * %.1f + Demo: %.1f * %.1f). Match Threshold: %.2f",
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
	
	searchName := name
	if name != "" {
		hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(name))))
		searchName = hex.EncodeToString(hash[:])
	}

	results, err := s.milvus.QueryIdentities(ctx, searchName, gender)
	if err != nil {
		return nil, err
	}

	// Decrypt results
	for _, res := range results {
		decName, err := crypto.Decrypt(res.Name, s.cfg.Security.AESKey)
		if err == nil {
			res.Name = decName
		}
		decDOB, err := crypto.Decrypt(res.DOB, s.cfg.Security.AESKey)
		if err == nil {
			res.DOB = decDOB
		}
	}

	return results, nil
}

// calculateStringSimilarity returns a value between 0 and 1 using Levenshtein Distance.
func calculateStringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	// Levenshtein Distance
	d := make([][]int, len(s1)+1)
	for i := range d {
		d[i] = make([]int, len(s2)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			d[i][j] = min3(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}

	dist := d[len(s1)][len(s2)]
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	return 1.0 - float64(dist)/float64(maxLen)
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}
