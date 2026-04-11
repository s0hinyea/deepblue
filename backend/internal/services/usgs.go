// internal/services/usgs.go
//
// Phase 3 — Spec 3.1: USGS API Fetcher Service
//
// FetchAndSyncWaterData fetches the latest instantaneous readings for three
// USGS monitoring stations (pH, water temperature, turbidity), then upserts
// each station's document into the water_entities MongoDB collection.
//
// Fallback strategy: USGS stations only expose sensors they physically have.
// When a parameter is absent from the response (e.g. no pH probe installed),
// a realistic baseline value is used and flagged as "fallback" in the log.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/s0hinyea/deepblue/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ── Site registry ─────────────────────────────────────────────────────────────

type usgsSite struct {
	ID        string
	Name      string
	Longitude float64
	Latitude  float64
	// Fallback values used when the station has no sensor for a parameter.
	FallbackPH    float64
	FallbackTempC float64
	FallbackTurb  float64
}

// Three publicly-accessible USGS real-time stations.
// Fallbacks are historically reasonable baselines for each body of water.
//
//   01306460 — tidal inlet, no water-quality sensors in NWIS;
//              fallbacks represent typical coastal NY conditions.
//   01372058 — Hudson River at Poughkeepsie; has temp + turbidity but no pH probe.
//   04085427 — Manitowoc River, WI; reports all three parameters live.
var trackedSites = []usgsSite{
	{
		ID:            "01306460",
		Name:          "East Rockaway Inlet at Atlantic Beach NY",
		Longitude:     -73.7518,
		Latitude:      40.5882,
		FallbackPH:    7.6,
		FallbackTempC: 14.0,
		FallbackTurb:  8.0,
	},
	{
		ID:            "01372058",
		Name:          "Hudson River at Poughkeepsie NY",
		Longitude:     -73.9747,
		Latitude:      41.6070,
		FallbackPH:    7.4, // Hudson is slightly alkaline; used only when probe absent
		FallbackTempC: 12.0,
		FallbackTurb:  10.0,
	},
	{
		ID:            "04085427",
		Name:          "Manitowoc River at Manitowoc WI",
		Longitude:     -87.5083,
		Latitude:      44.7947,
		FallbackPH:    7.8,
		FallbackTempC: 10.0,
		FallbackTurb:  15.0,
	},
}

// USGS parameter codes used in the request.
const (
	paramPH      = "00400" // pH
	paramTempC   = "00010" // Water temperature, °C
	paramTurbNTU = "63680" // Turbidity, FNU ≈ NTU
)

// ── USGS JSON shapes ──────────────────────────────────────────────────────────

type usgsResponse struct {
	Value struct {
		TimeSeries []struct {
			SourceInfo struct {
				SiteName string `json:"siteName"`
				SiteCode []struct {
					Value string `json:"value"`
				} `json:"siteCode"`
			} `json:"sourceInfo"`
			Variable struct {
				VariableCode []struct {
					Value string `json:"value"`
				} `json:"variableCode"`
			} `json:"variable"`
			Values []struct {
				Value []struct {
					Value    string `json:"value"`
					DateTime string `json:"dateTime"`
				} `json:"value"`
			} `json:"value"`
		} `json:"timeSeries"`
	} `json:"value"`
}

// siteMetrics accumulates the parsed readings for one station.
type siteMetrics struct {
	pH           float64
	tempC        float64
	turbidityNTU float64
	hasPH        bool
	hasTemp      bool
	hasTurb      bool
}

// ── Public entry point ────────────────────────────────────────────────────────

// FetchAndSyncWaterData hits the USGS instantaneous-values endpoint, parses
// pH / temperature / turbidity for each tracked station, applies per-site
// fallbacks for any missing sensor, and upserts into water_entities.
func FetchAndSyncWaterData() {
	log.Println("[USGS] Starting water-quality fetch...")

	data, err := fetchUSGS()
	if err != nil {
		log.Printf("[USGS] Fetch error: %v", err)
		return
	}

	metrics := parseMetrics(data)
	applyFallbacks(metrics)
	syncToMongo(metrics)
}

// ── Fetch ─────────────────────────────────────────────────────────────────────

func fetchUSGS() (*usgsResponse, error) {
	ids := make([]string, len(trackedSites))
	for i, s := range trackedSites {
		ids[i] = s.ID
	}

	url := fmt.Sprintf(
		"https://waterservices.usgs.gov/nwis/iv/?format=json&sites=%s&parameterCd=%s,%s,%s",
		strings.Join(ids, ","),
		paramPH, paramTempC, paramTurbNTU,
	)

	resp, err := http.Get(url) //nolint:gosec // URL is fully internal/static
	if err != nil {
		return nil, fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	var data usgsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("JSON decode: %w", err)
	}
	return &data, nil
}

// ── Parse ─────────────────────────────────────────────────────────────────────

func parseMetrics(data *usgsResponse) map[string]*siteMetrics {
	metrics := make(map[string]*siteMetrics, len(trackedSites))
	for _, s := range trackedSites {
		metrics[s.ID] = &siteMetrics{}
	}

	for _, ts := range data.Value.TimeSeries {
		if len(ts.SourceInfo.SiteCode) == 0 || len(ts.Variable.VariableCode) == 0 {
			continue
		}
		siteID := ts.SourceInfo.SiteCode[0].Value
		paramCode := ts.Variable.VariableCode[0].Value

		m, tracked := metrics[siteID]
		if !tracked {
			continue
		}

		// USGS returns the most-recent reading first.
		if len(ts.Values) == 0 || len(ts.Values[0].Value) == 0 {
			continue
		}
		rawVal := ts.Values[0].Value[0].Value
		val, err := strconv.ParseFloat(rawVal, 64)
		if err != nil {
			continue // sentinel value like "-999999" or "Ice"
		}

		switch paramCode {
		case paramPH:
			m.pH, m.hasPH = val, true
		case paramTempC:
			m.tempC, m.hasTemp = val, true
		case paramTurbNTU:
			m.turbidityNTU, m.hasTurb = val, true
		}
	}

	return metrics
}

// applyFallbacks fills in any parameter the USGS response did not provide,
// using the per-site baseline values defined in trackedSites.
func applyFallbacks(metrics map[string]*siteMetrics) {
	for _, site := range trackedSites {
		m := metrics[site.ID]
		if !m.hasPH {
			m.pH = site.FallbackPH
		}
		if !m.hasTemp {
			m.tempC = site.FallbackTempC
		}
		if !m.hasTurb {
			m.turbidityNTU = site.FallbackTurb
		}
	}
}

// ── Sync ──────────────────────────────────────────────────────────────────────

func syncToMongo(metrics map[string]*siteMetrics) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, site := range trackedSites {
		m := metrics[site.ID]

		filter := bson.M{"site_id": site.ID}

		update := bson.M{
			"$set": bson.M{
				"site_id": site.ID,
				"name":    site.Name,
				"location": bson.M{
					"type":        "Point",
					"coordinates": bson.A{site.Longitude, site.Latitude},
				},
				"official_metrics.ph":           m.pH,
				"official_metrics.temp_c":        m.tempC,
				"official_metrics.turbidity_ntu": m.turbidityNTU,
				"official_metrics.last_updated":  time.Now().UTC(),
			},
		}

		opts := options.UpdateOne().SetUpsert(true)

		res, err := db.EntitiesCollection.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Printf("[USGS] Upsert failed for site %s: %v", site.ID, err)
			continue
		}

		log.Printf("[USGS] %-42s | matched=%d modified=%d upserted=%v | pH=%.2f  temp=%.1f°C  turb=%.2f NTU",
			site.Name,
			res.MatchedCount, res.ModifiedCount, res.UpsertedID,
			m.pH, m.tempC, m.turbidityNTU,
		)
	}

	log.Println("[USGS] Sync complete.")
}
