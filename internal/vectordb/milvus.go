package vectordb

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/domain"
)

const (
	CollectionName = "kyc_identities_v2"
	DimFace        = 512
	DimName        = 768
)

type Client interface {
	InsertIdentity(ctx context.Context, record *domain.IdentityRecord) error
	SearchSimilar(ctx context.Context, faceEmbedding []float32, nameEmbedding []float32, topK int) ([]*domain.IdentityRecord, error)
	Close() error
}

type MilvusClient struct {
	client client.Client
	logger *slog.Logger
}

func NewMilvusClient(cfg *config.Config, logger *slog.Logger) (Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := client.NewClient(ctx, client.Config{
		Address: cfg.Milvus.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to milvus: %w", err)
	}

	mc := &MilvusClient{
		client: c,
		logger: logger,
	}

	if err := mc.initCollection(ctx); err != nil {
		logger.Error("Failed to initialize milvus collection", "error", err)
		// Return the client anyway so the application can start, maybe milvus is down temporarily
	}

	return mc, nil
}

func (m *MilvusClient) initCollection(ctx context.Context) error {
	has, err := m.client.HasCollection(ctx, CollectionName)
	if err != nil {
		return err
	}
	if has {
		return nil
	}

	schema := &entity.Schema{
		CollectionName: CollectionName,
		Description:    "KYC Identity Records",
		AutoID:         true,
		Fields: []*entity.Field{
			{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: true},
			{Name: "transaction_id", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}},
			{Name: "demographic_hash", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}},
			{Name: "face_embedding", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": fmt.Sprintf("%d", DimFace)}},
		},
	}

	if err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return err
	}

	// Create indices
	idxFace, err := entity.NewIndexHNSW(entity.COSINE, 16, 200)
	if err != nil {
		return err
	}
	if err := m.client.CreateIndex(ctx, CollectionName, "face_embedding", idxFace, false); err != nil {
		return err
	}

	return m.client.LoadCollection(ctx, CollectionName, false)
}

func (m *MilvusClient) InsertIdentity(ctx context.Context, record *domain.IdentityRecord) error {
	txnIds := []string{record.TransactionID}
	hashes := []string{record.DemographicHash}
	faces := [][]float32{record.FaceEmbedding}

	idCol := entity.NewColumnVarChar("transaction_id", txnIds)
	hashCol := entity.NewColumnVarChar("demographic_hash", hashes)
	faceCol := entity.NewColumnFloatVector("face_embedding", DimFace, faces)

	_, err := m.client.Insert(ctx, CollectionName, "", idCol, hashCol, faceCol)
	if err != nil {
		return err
	}
	
	// Flush to ensure it's searchable
	return m.client.Flush(ctx, CollectionName, false)
}

func (m *MilvusClient) SearchSimilar(ctx context.Context, faceEmbedding []float32, nameEmbedding []float32, topK int) ([]*domain.IdentityRecord, error) {
	// For simplicity in the hackathon, we'll do a vector search on the face_embedding first,
	// then we can refine or just retrieve the corresponding name_embedding and hash.
	sp, _ := entity.NewIndexHNSWSearchParam(74)
	
	searchResult, err := m.client.Search(ctx, CollectionName, []string{}, "", []string{"transaction_id", "demographic_hash"}, 
		[]entity.Vector{entity.FloatVector(faceEmbedding)}, "face_embedding", entity.COSINE, topK, sp)
		
	if err != nil {
		return nil, err
	}

	var results []*domain.IdentityRecord
	for _, res := range searchResult {
		for i := 0; i < res.ResultCount; i++ {
			// Extract fields
			var txnID string
			var hash string
			
			// We iterate through Fields to find our output fields
			for _, field := range res.Fields {
				if field.Name() == "transaction_id" {
					if vCol, ok := field.(*entity.ColumnVarChar); ok {
						txnID, _ = vCol.ValueByIdx(i)
					}
				}
				if field.Name() == "demographic_hash" {
					if vCol, ok := field.(*entity.ColumnVarChar); ok {
						hash, _ = vCol.ValueByIdx(i)
					}
				}
			}

			// Note: Milvus scores for COSINE are distances if using older versions or similarities depending on index.
			// Let's just collect the matched records. We can refine logic in the service layer.
			results = append(results, &domain.IdentityRecord{
				TransactionID:   txnID,
				DemographicHash: hash,
				// Note: face embedding and name embedding aren't returned by default unless requested, 
				// we could request them but for verification we just need the hash for exact match.
			})
		}
	}

	return results, nil
}

func (m *MilvusClient) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
