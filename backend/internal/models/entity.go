package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type OfficialMetrics struct {
	PH          float64   `bson:"ph"            json:"ph"`
	TempC       float64   `bson:"temp_c"        json:"temp_c"`
	TurbidityNTU float64  `bson:"turbidity_ntu" json:"turbidity_ntu"`
	LastUpdated time.Time `bson:"last_updated"  json:"last_updated"`
}

// GeoJSON Point as expected by MongoDB 2dsphere index.
type GeoPoint struct {
	Type        string     `bson:"type"        json:"type"`
	Coordinates [2]float64 `bson:"coordinates" json:"coordinates"` // [longitude, latitude]
}

type WaterEntity struct {
	ID             bson.ObjectID `bson:"_id,omitempty"    json:"id"`
	Name           string             `bson:"name"             json:"name"`
	Location       GeoPoint           `bson:"location"         json:"location"`
	OfficialMetrics OfficialMetrics   `bson:"official_metrics" json:"official_metrics"`
	SafetyScore    float64            `bson:"safety_score"     json:"safety_score"`
	SafetyLabel    string             `bson:"safety_label"     json:"safety_label"`
	ReportCount    int                `bson:"report_count"     json:"report_count"`
	LastReportAt   time.Time          `bson:"last_report_at"   json:"last_report_at"`
}

// SafetyLabel derives a label string from a numeric score.
func SafetyLabel(score float64) string {
	switch {
	case score < 0.3:
		return "safe"
	case score < 0.6:
		return "moderate"
	default:
		return "dangerous"
	}
}

// ComputeSafetyScore calculates the Safety Index using sensor metrics and an
// optional community visual-risk average (pass 0.0 when no reports exist yet).
//
//   S = (W_ph × ΔpH_norm) + (W_turb × T_norm) + (W_vis × V_avg)
//
// Weights: W_ph=0.3, W_turb=0.3, W_vis=0.4
// ΔpH_norm  = abs(ph - 7.0) / 7.0          (max deviation from neutral, clamped 0–1)
// T_norm    = turbidity_ntu / 25.0          (EPA recreational threshold ≈ 25 NTU, clamped 0–1)
// V_avg     = average AI confidence from last 5 community reports (0–1)
// ComputeSafetyScore calculates a 0–1 risk score using only the metrics that
// were actually measured (non-zero). Missing sensors are excluded and their
// weight is redistributed so the score is never inflated by absent data.
func ComputeSafetyScore(ph, turbidityNTU, vAvg float64) float64 {
	score, totalWeight := 0.0, 0.0

	if ph != 0 {
		phNorm := abs(ph-7.0) / 7.0
		if phNorm > 1.0 {
			phNorm = 1.0
		}
		score += 0.3 * phNorm
		totalWeight += 0.3
	}

	if turbidityNTU != 0 {
		tNorm := turbidityNTU / 25.0
		if tNorm > 1.0 {
			tNorm = 1.0
		}
		score += 0.3 * tNorm
		totalWeight += 0.3
	}

	if vAvg != 0 {
		score += 0.4 * vAvg
		totalWeight += 0.4
	}

	if totalWeight == 0 {
		return 0
	}
	// Normalise so a station with only one sensor isn't artificially low-scored.
	return score / totalWeight
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
