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
	CollectionFace = "face_embeddings"
	CollectionName = "name_embeddings"
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
		logger.Error().Err(err).Msg("Failed to initialize milvus collections")
	} else {
		logger.Info().Str("face_collection", CollectionFace).Str("name_collection", CollectionName).Msg("Milvus collections initialized and loaded")
	}

	return mc, nil
}

func (m *MilvusClient) initCollection(ctx context.Context) error {
	// Initialize Face Collection
	hasFace, err := m.client.HasCollection(ctx, CollectionFace)
	if err != nil {
		return fmt.Errorf("has face collection check failed: %w", err)
	}

	if !hasFace {
		m.logger.Info().Str("name", CollectionFace).Msg("Creating face collection")
		schema := &entity.Schema{
			CollectionName: CollectionFace,
			Description:    "KYC Face Biometrics",
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
			return fmt.Errorf("create face collection failed: %w", err)
		}

		m.logger.Info().Str("collection", CollectionFace).Msg("Creating face index")
		idx, err := entity.NewIndexHNSW(entity.L2, 16, 200)
		if err != nil {
			return fmt.Errorf("failed to create face index entity: %w", err)
		}
		if err := m.client.CreateIndex(ctx, CollectionFace, "face_embedding", idx, false); err != nil {
			return fmt.Errorf("create face index failed: %w", err)
		}
	}

	// Initialize Name Collection
	hasName, err := m.client.HasCollection(ctx, CollectionName)
	if err != nil {
		return fmt.Errorf("has name collection check failed: %w", err)
	}

	if !hasName {
		m.logger.Info().Str("name", CollectionName).Msg("Creating name collection")
		schema := &entity.Schema{
			CollectionName: CollectionName,
			Description:    "KYC Name Semantic Embeddings",
			AutoID:         true,
			Fields: []*entity.Field{
				{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: true},
				{Name: "transaction_id", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}},
				{Name: "name_embedding", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": fmt.Sprintf("%d", DimName)}},
			},
		}

		if err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return fmt.Errorf("create name collection failed: %w", err)
		}

		m.logger.Info().Str("collection", CollectionName).Msg("Creating name index")
		idx, err := entity.NewIndexHNSW(entity.L2, 16, 200)
		if err != nil {
			return fmt.Errorf("failed to create name index entity: %w", err)
		}
		if err := m.client.CreateIndex(ctx, CollectionName, "name_embedding", idx, false); err != nil {
			return fmt.Errorf("create name index failed: %w", err)
		}
	}

	m.logger.Info().Str("name", CollectionFace).Msg("Loading face collection")
	if err := m.client.LoadCollection(ctx, CollectionFace, false); err != nil {
		return fmt.Errorf("load face collection failed: %w", err)
	}

	m.logger.Info().Str("name", CollectionName).Msg("Loading name collection")
	if err := m.client.LoadCollection(ctx, CollectionName, false); err != nil {
		return fmt.Errorf("load name collection failed: %w", err)
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

	// Insert into Face Collection
	_, err := m.client.Insert(ctx, CollectionFace, "", idCol, hashCol, nameCol, blindCol, dobCol, genderCol, faceCol)
	if err != nil {
		return fmt.Errorf("face insert failed: %w", err)
	}

	// Insert into Name Collection
	nameSigs := [][]float32{record.NameEmbedding}
	nameIDCol := entity.NewColumnVarChar("transaction_id", txnIds)
	nameEmbCol := entity.NewColumnFloatVector("name_embedding", DimName, nameSigs)

	_, err = m.client.Insert(ctx, CollectionName, "", nameIDCol, nameEmbCol)
	if err != nil {
		return fmt.Errorf("name insert failed: %w", err)
	}

	// Flush to ensure searchability
	if err := m.client.Flush(ctx, CollectionFace, false); err != nil {
		return fmt.Errorf("face flush failed: %w", err)
	}
	if err := m.client.Flush(ctx, CollectionName, false); err != nil {
		return fmt.Errorf("name flush failed: %w", err)
	}

	return nil
}

func (m *MilvusClient) SearchSimilar(ctx context.Context, faceEmbedding []float32, nameEmbedding []float32, topK int) ([]*domain.IdentityRecord, error) {
	sp, _ := entity.NewIndexHNSWSearchParam(74)

	var searchResult []client.SearchResult
	var err error

	// Search primarily by face
	for i := 0; i < 2; i++ {
		searchResult, err = m.client.Search(ctx, CollectionFace, []string{}, "", []string{"transaction_id", "demographic_hash", "name", "dob", "gender"},
			[]entity.Vector{entity.FloatVector(faceEmbedding)}, "face_embedding", entity.L2, topK, sp)

		if err == nil {
			break
		}

		if i == 0 {
			m.logger.Warn().Err(err).Msg("Milvus search failed, attempting to reload face collection")
			_ = m.client.LoadCollection(ctx, CollectionFace, true)
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}

	if err != nil {
		return nil, err
	}

	var records []*domain.IdentityRecord
	var txnIDs []string

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
			txnIDs = append(txnIDs, fmt.Sprintf("\"%s\"", txnID))
		}
	}

	if len(txnIDs) == 0 {
		return records, nil
	}

	// Fetch name embeddings from the second collection
	expr := fmt.Sprintf("transaction_id in [%s]", joinStrings(txnIDs, ","))
	nameResults, err := m.client.Query(ctx, CollectionName, []string{}, expr, []string{"transaction_id", "name_embedding"})
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to fetch name embeddings for search results")
		return records, nil // Return what we have, though name matching will be disabled
	}

	// Map name embeddings back to records
	nameMap := make(map[string][]float32)
	var txnCol *entity.ColumnVarChar
	var embCol *entity.ColumnFloatVector

	for _, f := range nameResults {
		if f.Name() == "transaction_id" {
			txnCol = f.(*entity.ColumnVarChar)
		} else if f.Name() == "name_embedding" {
			embCol = f.(*entity.ColumnFloatVector)
		}
	}

	if txnCol != nil && embCol != nil {
		for i := 0; i < txnCol.Len(); i++ {
			tid, _ := txnCol.ValueByIdx(i)
			nameMap[tid] = embCol.Data()[i]
		}
	}

	for _, rec := range records {
		if emb, ok := nameMap[rec.TransactionID]; ok {
			rec.NameEmbedding = emb
		}
	}

	return records, nil
}

func joinStrings(parts []string, sep string) string {
	res := ""
	for i, p := range parts {
		if i > 0 {
			res += sep
		}
		res += p
	}
	return res
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

	queryResult, err := m.client.Query(ctx, CollectionFace, []string{}, expr, []string{"transaction_id", "demographic_hash", "name", "dob", "gender"})
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
