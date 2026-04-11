// scripts/seed_knowledge.go
//
// Phase 2 — RAG Knowledge Pre-seeding (Specs 2.1 + 2.2)
//
// Standalone script. Run from the backend/ directory:
//
//	go run ./scripts/seed_knowledge.go
//
// What it does:
//  1. Loads credentials from the environment (.env + AWS_* vars).
//  2. For each EPA/WHO text paragraph, calls Bedrock's
//     amazon.titan-embed-text-v2:0 model to produce a 1536-dim float32 vector.
//  3. Inserts { source, section, content, embedding, created_at } into the
//     MongoDB `knowledge_chunks` collection.
//
// Prerequisites:
//   - MONGODB_URI in .env
//   - AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION in environment
//   - Bedrock model access granted for amazon.titan-embed-text-v2:0 in your AWS region
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── Bedrock request / response shapes ─────────────────────────────────────────

type titanRequest struct {
	InputText string `json:"inputText"`
}

type titanResponse struct {
	Embedding           []float32 `json:"embedding"`
	InputTextTokenCount int       `json:"inputTextTokenCount"`
}

// ── MongoDB document shape ────────────────────────────────────────────────────

type knowledgeChunk struct {
	Source    string    `bson:"source"`
	Section   string    `bson:"section"`
	Content   string    `bson:"content"`
	Embedding []float32 `bson:"embedding"`
	CreatedAt time.Time `bson:"created_at"`
}

// ── Hardcoded EPA / WHO knowledge paragraphs ──────────────────────────────────

type paragraph struct {
	source  string
	section string
	content string
}

var knowledgeParagraphs = []paragraph{
	{
		source:  "EPA Recreational Water Quality Criteria 2012",
		section: "pH Standards for Freshwater Recreation",
		content: "The EPA recommends a pH range of 6.5 to 8.5 for freshwater bodies used for primary contact recreation such as swimming and wading. A pH below 6.0 can indicate acid mine drainage or industrial runoff and is corrosive to skin and mucous membranes. A pH above 9.0 frequently accompanies dense algal blooms, as photosynthesizing cyanobacteria strip dissolved CO2 from the water, driving alkalinity upward. Sustained pH readings outside the 6.5–8.5 band are grounds for issuing a precautionary advisory even in the absence of visible bloom conditions.",
	},
	{
		source:  "EPA Recreational Water Quality Criteria 2012",
		section: "Turbidity and Water Clarity Standards",
		content: "Turbidity, measured in Nephelometric Turbidity Units (NTU), quantifies the cloudiness of water caused by suspended particles including sediment, algae, and organic matter. The EPA recreational threshold for primary contact is 25 NTU; values above this level reduce underwater visibility, increasing the risk of submersion accidents and concealing hazardous debris. Turbidity exceeding 50 NTU warrants beach closure. High turbidity also attenuates UV light, reducing the natural disinfection of waterborne pathogens. Turbidity spikes following heavy rainfall often correlate with elevated fecal coliform counts and should trigger immediate retesting.",
	},
	{
		source:  "WHO Guidelines for Safe Recreational Water Environments, Volume 1: Coastal and Fresh Waters (2003)",
		section: "Cyanobacteria Alert Level Thresholds",
		content: "The World Health Organization defines three progressive alert levels for cyanobacteria in recreational freshwater. Guidance Level 1 (low probability of adverse health effects): cyanobacterial cell densities below 20,000 cells per mL; no action beyond routine monitoring required. Guidance Level 2 (moderate risk): densities of 20,000–100,000 cells/mL; post visible warning signs and advise sensitive groups (children, immunocompromised individuals) to avoid contact. Alert Level (high risk): densities exceeding 100,000 cells/mL or visible surface scum; prohibit all primary contact recreation and investigate toxin concentrations. These thresholds apply to both planktonic blooms and benthic mats.",
	},
	{
		source:  "EPA Harmful Algal Bloom (HAB) Field Identification Guidelines 2019",
		section: "Visual Identification of Algal Blooms",
		content: "Harmful algal blooms (HABs) can be tentatively identified in the field by several visual cues. Characteristic indicators include: (1) water discoloration ranging from bright green or blue-green to brown or red, depending on the dominant genus; (2) surface scum or streaks of dense biomass concentrated by wind along shorelines; (3) a paint-like, oily, or spilled-milk appearance on the water surface; (4) clumping of algal cells into floating mats that disperse and regroup; and (5) musty, earthy, or sulfurous odors produced by geosmin and other volatile metabolites. The absence of visible scum does not exclude bloom conditions—subsurface blooms can reach toxic concentrations before surfacing. Any water exhibiting two or more of these indicators should be sampled and tested for cyanotoxins before recreational use.",
	},
	{
		source:  "EPA Recommended Human Health Recreational Ambient Water Quality Criteria for Cyanotoxins 2019",
		section: "Microcystin and Recreational Guidance Values",
		content: "Microcystins are the most frequently detected cyanotoxin class in freshwater HABs worldwide. They are hepatotoxic cyclic peptides produced primarily by Microcystis, Dolichospermum, and Planktothrix genera. The EPA recreational guidance values are 8 micrograms per liter (μg/L) for adults and 6 μg/L for children. Dermal exposure during swimming can cause skin rashes and irritation at sub-guidance concentrations. Ingestion via accidental swallowing can cause nausea, vomiting, diarrhea, and, at high doses, acute liver damage. Toxin concentrations within a bloom are spatially heterogeneous and can vary by two to three orders of magnitude across a single water body, necessitating multiple sampling points before issuing an all-clear.",
	},
	{
		source:  "EPA Recreational Water Quality Criteria 2012",
		section: "Temperature, Dissolved Oxygen, and Eutrophication Indicators",
		content: "Water temperature above 25°C (77°F) accelerates metabolic rates in cyanobacteria and can trigger exponential bloom growth within 48–72 hours when combined with elevated nutrient loading. The EPA recommends dissolved oxygen (DO) concentrations above 5 mg/L for waters supporting primary contact recreation; DO below 2 mg/L (hypoxia) creates conditions lethal to fish and other aquatic organisms and indicates severe organic enrichment. Nighttime oxygen crashes following afternoon photosynthetic peaks are a hallmark of eutrophic systems on the verge of bloom formation. Monitoring programs should collect temperature and DO readings at both surface and sub-thermocline depths to detect stratification-driven hypoxic layers that do not appear in surface measurements.",
	},
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	// 1. Load .env (MONGODB_URI lives here; AWS vars are read directly from the shell)
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: no .env file found — relying on system environment")
	}

	// 2. Connect to MongoDB
	db.Connect()
	ctx := context.Background()

	// 3. Build the Bedrock client.
	//    aws-sdk-go-v2/config.LoadDefaultConfig reads, in order:
	//      a) AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY environment variables
	//      b) ~/.aws/credentials file
	//      c) EC2 instance / ECS task metadata (if running on AWS)
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	brc := bedrockruntime.NewFromConfig(awsCfg)

	// 4. Drop the collection so the script is safe to re-run
	if err := db.KnowledgeCollection.Drop(ctx); err != nil {
		log.Fatalf("Failed to drop knowledge_chunks: %v", err)
	}
	fmt.Println("Dropped knowledge_chunks collection.")

	// 5. Embed each paragraph and insert into MongoDB
	for i, p := range knowledgeParagraphs {
		fmt.Printf("[%d/%d] Embedding: %q ...\n", i+1, len(knowledgeParagraphs), p.section)

		embedding, err := embed(ctx, brc, p.content)
		if err != nil {
			log.Fatalf("Bedrock embed failed for paragraph %d: %v", i+1, err)
		}

		chunk := knowledgeChunk{
			Source:    p.source,
			Section:   p.section,
			Content:   p.content,
			Embedding: embedding,
			CreatedAt: time.Now(),
		}

		// Convert struct to bson.D so we can log the field count
		doc, err := toBsonD(chunk)
		if err != nil {
			log.Fatalf("BSON marshal failed: %v", err)
		}

		res, err := db.KnowledgeCollection.InsertOne(ctx, doc)
		if err != nil {
			log.Fatalf("InsertOne failed: %v", err)
		}

		fmt.Printf("    → inserted _id=%v  embedding_dims=%d\n", res.InsertedID, len(embedding))
	}

	fmt.Printf("\nPre-seed complete. %d knowledge chunks inserted into MongoDB.\n", len(knowledgeParagraphs))
	fmt.Println("Next step: create an Atlas Vector Search index on the `embedding` field in the MongoDB Atlas UI.")
}

// embed calls Bedrock's Titan Embed Text v2 model and returns the float32 vector.
func embed(ctx context.Context, brc *bedrockruntime.Client, text string) ([]float32, error) {
	body, err := json.Marshal(titanRequest{InputText: text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	out, err := brc.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     strPtr("amazon.titan-embed-text-v2:0"),
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("InvokeModel: %w", err)
	}

	var resp titanResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(resp.Embedding) == 0 {
		return nil, fmt.Errorf("bedrock returned empty embedding")
	}

	return resp.Embedding, nil
}

// toBsonD converts any struct to a bson.D using the bson codec.
func toBsonD(v any) (bson.D, error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}
	var doc bson.D
	if err := bson.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func strPtr(s string) *string { return &s }
