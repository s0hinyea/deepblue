// internal/services/ai_service.go
//
// Phase 5 — Spec 5.1: Claude Multi-modal & Text AI Services
package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/s0hinyea/deepblue/internal/models"
)

const claudeModelID = "anthropic.claude-3-haiku-20240307-v1:0"

// ── Bedrock Messages API shapes ───────────────────────────────────────────────

type claudeRequest struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	Messages         []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type   string       `json:"type"`
	Source *imageSource `json:"source,omitempty"`
	Text   string       `json:"text,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"` // "base64"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// ── Spec 5.1a: Image Analysis ─────────────────────────────────────────────────

// AnalyzeImageLabels sends the image at imageURL to Claude 3.5 Sonnet and
// returns a slice of water-risk tags (e.g. ["algae", "turbid"]).
//
// The uploaded S3 object must be publicly readable so the app can fetch the
// bytes and forward them to Bedrock as a base64 image block.
func AnalyzeImageLabels(ctx context.Context, imageURL string) ([]string, error) {
	brc, err := newBedrockClient(ctx)
	if err != nil {
		return nil, err
	}

	image, err := fetchImageSource(ctx, imageURL)
	if err != nil {
		return nil, err
	}

	prompt := `You are a water safety analyst reviewing a photo of a water body.
Identify any visible safety risks and return ONLY a valid JSON array of tags from this set:
  algae, debris, pollution, discoloration, foam, turbid, clear

Use "clear" only when the water shows no visible hazards.
Respond with the JSON array only — no markdown, no explanation.
Example: ["algae","turbid"]`

	req := claudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        128,
		Messages: []claudeMessage{{
			Role: "user",
			Content: []contentBlock{
				{Type: "image", Source: image},
				{Type: "text", Text: prompt},
			},
		}},
	}

	raw, err := invokeClaudeRaw(ctx, brc, req)
	if err != nil {
		return nil, err
	}

	var tags []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &tags); err != nil {
		return nil, fmt.Errorf("parse tags JSON %q: %w", raw, err)
	}
	return tags, nil
}

// ── Spec 5.1b: Advisory Generation ───────────────────────────────────────────

// GenerateAdvisory calls Claude 3.5 Sonnet with the water entity's live metrics
// plus the top RAG paragraphs from the EPA/WHO knowledge base, and returns a
// 2-sentence plain-English safety advisory.
// AdvisoryResult holds the two sections of the generated advisory.
type AdvisoryResult struct {
	SensorAdvisory    string
	CommunityAdvisory string
}

func GenerateAdvisory(ctx context.Context, entity models.WaterEntity, manualParagraphs []string, communityContext string) (AdvisoryResult, error) {
	brc, err := newBedrockClient(ctx)
	if err != nil {
		return AdvisoryResult{}, err
	}

	guidelines := strings.Join(manualParagraphs, "\n\n---\n\n")

	phLine := "pH:          not measured at this station"
	if entity.OfficialMetrics.PH != 0 {
		phLine = fmt.Sprintf("pH:          %.2f  (safe range: 6.5–8.5)", entity.OfficialMetrics.PH)
	}
	tempLine := "Temperature: not measured at this station"
	if entity.OfficialMetrics.TempC != 0 {
		tempLine = fmt.Sprintf("Temperature: %.1f°C", entity.OfficialMetrics.TempC)
	}
	turbLine := "Turbidity:   not measured at this station"
	if entity.OfficialMetrics.TurbidityNTU != 0 {
		turbLine = fmt.Sprintf("Turbidity:   %.2f NTU  (EPA recreational threshold: 25 NTU)", entity.OfficialMetrics.TurbidityNTU)
	}

	prompt := fmt.Sprintf(`You are a public water safety expert writing advisories for swimmers and recreators.

Location:     %s
%s
%s
%s
Safety Score: %.2f / 1.0  (0 = safe, 1 = dangerous)

%s

Relevant EPA / WHO guidelines:
%s

Respond with valid JSON only — no markdown, no explanation. Use exactly this shape:
{
  "sensor_advisory": "1–2 sentences based only on the sensor readings and EPA/WHO guidelines above. Only reference metrics that were actually measured. State whether the location is safe for contact recreation.",
  "community_advisory": "1–2 sentences based only on the community visual reports above. If no reports were submitted, write: No community reports have been submitted for this station yet."
}`,
		entity.Name,
		phLine,
		tempLine,
		turbLine,
		entity.SafetyScore,
		communityContext,
		guidelines,
	)

	req := claudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        512,
		Messages: []claudeMessage{{
			Role:    "user",
			Content: []contentBlock{{Type: "text", Text: prompt}},
		}},
	}

	raw, err := invokeClaudeRaw(ctx, brc, req)
	if err != nil {
		return AdvisoryResult{}, err
	}

	var result AdvisoryResult
	var parsed struct {
		SensorAdvisory    string `json:"sensor_advisory"`
		CommunityAdvisory string `json:"community_advisory"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		// Fallback: put the raw response in sensor_advisory if JSON parsing fails.
		return AdvisoryResult{SensorAdvisory: raw}, nil
	}
	result.SensorAdvisory = parsed.SensorAdvisory
	result.CommunityAdvisory = parsed.CommunityAdvisory
	return result, nil
}

// ── shared helpers ─────────────────────────────────────────────────────────────

func invokeClaudeRaw(ctx context.Context, brc *bedrockruntime.Client, req claudeRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	out, err := brc.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     strPtr(claudeModelID),
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        body,
	})
	if err != nil {
		return "", fmt.Errorf("InvokeModel: %w", err)
	}

	var resp claudeResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return strings.TrimSpace(resp.Content[0].Text), nil
}

func fetchImageSource(ctx context.Context, imageURL string) (*imageSource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build image request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: unexpected HTTP status %d", resp.StatusCode)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("read image: empty body")
	}

	mediaType := resp.Header.Get("Content-Type")
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = http.DetectContentType(imageBytes)
	}

	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		return nil, fmt.Errorf("unsupported image media type %q", mediaType)
	}

	return &imageSource{
		Type:      "base64",
		MediaType: mediaType,
		Data:      base64.StdEncoding.EncodeToString(imageBytes),
	}, nil
}

func newBedrockClient(ctx context.Context) (*bedrockruntime.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return bedrockruntime.NewFromConfig(awsCfg), nil
}

func strPtr(s string) *string { return &s }
