package vectordb

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/domain"
)

const (
	CollectionName = "kyc_identities_v17"
	DimFace        = 512
	DimName        = 768
)

type Client interface {
	InsertIdentity(ctx context.Context, record *domain.IdentityRecord) error
	SearchSimilar(ctx context.Context, faceEmbedding []float32, nameEmbedding []float32, topK int) ([]*domain.IdentityRecord, error)
	QueryIdentities(ctx context.Context, name string, gender string) ([]*domain.IdentityRecord, error)
	Close() error
}

type MilvusClient struct {
	client client.Client
	logger *zerolog.Logger
}

func NewMilvusClient(cfg *config.Config, logger *zerolog.Logger) (Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		logger.Error().Err(err).Msg("Failed to initialize milvus collection")
	} else {
		logger.Info().Str("collection", CollectionName).Msg("Milvus collection initialized and loaded")
	}

	return mc, nil
}

func (m *MilvusClient) initCollection(ctx context.Context) error {
	has, err := m.client.HasCollection(ctx, CollectionName)
	if err != nil {
		return fmt.Errorf("has collection check failed: %w", err)
	}

	if !has {
		m.logger.Info().Str("name", CollectionName).Msg("Creating collection")
		schema := &entity.Schema{
			CollectionName: CollectionName,
			Description:    "KYC Identity Records v11",
			AutoID:         true,
			Fields: []*entity.Field{
				{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: true},
				{Name: "transaction_id", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}},
				{Name: "demographic_hash", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}},
				{Name: "name", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "512"}},
				{Name: "name_blind_index", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}},
				{Name: "dob", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}},
				{Name: "gender", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "16"}},
				{Name: "face_embedding", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": fmt.Sprintf("%d", DimFace)}},
			},
		}

		if err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return fmt.Errorf("create collection failed: %w", err)
		}

		m.logger.Info().Str("collection", CollectionName).Msg("Creating face index")
		idx, err := entity.NewIndexHNSW(entity.L2, 16, 200)
		if err != nil {
			return fmt.Errorf("failed to create index entity: %w", err)
		}
		if err := m.client.CreateIndex(ctx, CollectionName, "face_embedding", idx, false); err != nil {
			return fmt.Errorf("create face index failed: %w", err)
		}
	}

	m.logger.Info().Str("name", CollectionName).Msg("Loading collection")
	if err := m.client.LoadCollection(ctx, CollectionName, false); err != nil {
		return fmt.Errorf("load collection failed: %w", err)
	}

	return nil
}

func (m *MilvusClient) InsertIdentity(ctx context.Context, record *domain.IdentityRecord) error {
	txnIds := []string{record.TransactionID}
	hashes := []string{record.DemographicHash}
	names := []string{record.Name}
	blindIndexes := []string{record.NameBlindIndex}
	dobs := []string{record.DOB}
	genders := []string{record.Gender}
	
	faceSigs := [][]float32{record.FaceEmbedding}

	idCol := entity.NewColumnVarChar("transaction_id", txnIds)
	hashCol := entity.NewColumnVarChar("demographic_hash", hashes)
	nameCol := entity.NewColumnVarChar("name", names)
	blindCol := entity.NewColumnVarChar("name_blind_index", blindIndexes)
	dobCol := entity.NewColumnVarChar("dob", dobs)
	genderCol := entity.NewColumnVarChar("gender", genders)
	faceCol := entity.NewColumnFloatVector("face_embedding", DimFace, faceSigs)

	_, err := m.client.Insert(ctx, CollectionName, "", idCol, hashCol, nameCol, blindCol, dobCol, genderCol, faceCol)
	if err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}

	// Flush to ensure it's searchable
	if err := m.client.Flush(ctx, CollectionName, false); err != nil {
		return fmt.Errorf("flush failed: %w", err)
	}

	return nil
}

func (m *MilvusClient) SearchSimilar(ctx context.Context, faceEmbedding []float32, nameEmbedding []float32, topK int) ([]*domain.IdentityRecord, error) {
	sp, _ := entity.NewIndexHNSWSearchParam(74)

	var searchResult []client.SearchResult
	var err error

	// Search primarily by face
	for i := 0; i < 2; i++ {
		searchResult, err = m.client.Search(ctx, CollectionName, []string{}, "", []string{"transaction_id", "demographic_hash", "name", "dob", "gender"},
			[]entity.Vector{entity.FloatVector(faceEmbedding)}, "face_embedding", entity.L2, topK, sp)

		if err == nil {
			break
		}

		if i == 0 {
			m.logger.Warn().Err(err).Msg("Milvus search failed, attempting to reload collection")
			_ = m.client.LoadCollection(ctx, CollectionName, true)
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}

	if err != nil {
		return nil, err
	}

	var records []*domain.IdentityRecord
	for _, res := range searchResult {
		for i := 0; i < res.ResultCount; i++ {
			var txnID, hash, name, dob, gender string

			for _, field := range res.Fields {
				switch field.Name() {
				case "transaction_id":
					if v, ok := field.(*entity.ColumnVarChar); ok {
						txnID, _ = v.ValueByIdx(i)
					}
				case "demographic_hash":
					if v, ok := field.(*entity.ColumnVarChar); ok {
						hash, _ = v.ValueByIdx(i)
					}
				case "name":
					if v, ok := field.(*entity.ColumnVarChar); ok {
						name, _ = v.ValueByIdx(i)
					}
				case "dob":
					if v, ok := field.(*entity.ColumnVarChar); ok {
						dob, _ = v.ValueByIdx(i)
					}
				case "gender":
					if v, ok := field.(*entity.ColumnVarChar); ok {
						gender, _ = v.ValueByIdx(i)
					}
				}
			}

			records = append(records, &domain.IdentityRecord{
				TransactionID:   txnID,
				DemographicHash: hash,
				Name:            name,
				DOB:             dob,
				Gender:          gender,
				Score:           res.Scores[i],
			})
		}
	}
	return records, nil
}

func (m *MilvusClient) QueryIdentities(ctx context.Context, name string, gender string) ([]*domain.IdentityRecord, error) {
	expr := ""
	if name != "" && gender != "" {
		expr = fmt.Sprintf("name_blind_index == \"%s\" && gender == \"%s\"", name, gender)
	} else if name != "" {
		expr = fmt.Sprintf("name_blind_index == \"%s\"", name)
	} else if gender != "" {
		expr = fmt.Sprintf("gender == \"%s\"", gender)
	}

	queryResult, err := m.client.Query(ctx, CollectionName, []string{}, expr, []string{"transaction_id", "demographic_hash", "name", "dob", "gender"})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var records []*domain.IdentityRecord
	rowCount := 0
	if len(queryResult) > 0 {
		rowCount = queryResult[0].Len()
	}

	for i := 0; i < rowCount; i++ {
		record := &domain.IdentityRecord{}
		for _, f := range queryResult {
			switch f.Name() {
			case "transaction_id":
				if v, ok := f.(*entity.ColumnVarChar); ok {
					record.TransactionID, _ = v.ValueByIdx(i)
				}
			case "demographic_hash":
				if v, ok := f.(*entity.ColumnVarChar); ok {
					record.DemographicHash, _ = v.ValueByIdx(i)
				}
			case "name":
				if v, ok := f.(*entity.ColumnVarChar); ok {
					record.Name, _ = v.ValueByIdx(i)
				}
			case "dob":
				if v, ok := f.(*entity.ColumnVarChar); ok {
					record.DOB, _ = v.ValueByIdx(i)
				}
			case "gender":
				if v, ok := f.(*entity.ColumnVarChar); ok {
					record.Gender, _ = v.ValueByIdx(i)
				}
			}
		}
		records = append(records, record)
	}

	return records, nil
}

func (m *MilvusClient) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
