// internal/services/rag_service.go
//
// Phase 5 — Spec 5.2: Vector RAG Service
//
// GetRelevantGuidelines embeds a query string derived from live sensor metrics,
// runs a MongoDB $vectorSearch against the pre-seeded knowledge_chunks
// collection, and returns the top 3 matching EPA/WHO paragraphs.
package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ── Titan Embed shapes ────────────────────────────────────────────────────────

type titanEmbedRequest struct {
	InputText string `json:"inputText"`
}

type titanEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// ── Public entry point ────────────────────────────────────────────────────────

// GetRelevantGuidelines builds a natural-language search query from the
// entity's current sensor readings, embeds it with Titan, then performs a
// $vectorSearch on knowledge_chunks.
//
// Prerequisites: an Atlas Vector Search index named "vector_index" must exist
// on the `embedding` field of the knowledge_chunks collection.
func GetRelevantGuidelines(ctx context.Context, metrics models.OfficialMetrics) ([]string, error) {
	query := fmt.Sprintf(
		"Water safety guidelines for pH %.1f, turbidity %.1f NTU, temperature %.1f°C",
		metrics.PH, metrics.TurbidityNTU, metrics.TempC,
	)

	embedding, err := embedText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: "vector_index"},
			{Key: "path", Value: "embedding"},
			{Key: "queryVector", Value: embedding},
			{Key: "numCandidates", Value: int32(50)},
			{Key: "limit", Value: int32(3)},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "content", Value: 1},
			{Key: "_id", Value: 0},
		}}},
	}

	cursor, err := db.KnowledgeCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vectorSearch aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var paragraphs []string
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if content, ok := doc["content"].(string); ok {
			paragraphs = append(paragraphs, content)
		}
	}

	return paragraphs, nil
}

// ── Titan Embed helper ────────────────────────────────────────────────────────

func embedText(ctx context.Context, text string) ([]float32, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	brc := bedrockruntime.NewFromConfig(awsCfg)

	body, err := json.Marshal(titanEmbedRequest{InputText: text})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	out, err := brc.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     strPtr("amazon.titan-embed-text-v2:0"),
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("InvokeModel Titan: %w", err)
	}

	var resp titanEmbedResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return resp.Embedding, nil
}
