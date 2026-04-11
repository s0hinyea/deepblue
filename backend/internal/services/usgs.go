// internal/services/usgs.go
//
// Phase 3 — Spec 3.1: USGS API Fetcher Service
//
// FetchAndSyncWaterData queries the USGS instantaneous-values API for every
// active stream/lake monitoring station in New York State that reports at
// least one of pH, water temperature, or turbidity. Sites are discovered
// dynamically — no hardcoded list required.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/s0hinyea/deepblue/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// USGS instantaneous-values endpoint filtered to New York streams and lakes.
const usgsNYURL = "https://waterservices.usgs.gov/nwis/iv/" +
	"?format=json&stateCd=ny&parameterCd=00400,00010,63680&siteType=ST,LK"

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
				GeoLocation struct {
					GeogLocation struct {
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
					} `json:"geogLocation"`
				} `json:"geoLocation"`
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

// siteInfo holds the geographic metadata for one station.
type siteInfo struct {
	ID   string
	Name string
	Lat  float64
	Lng  float64
}

// siteMetrics holds the latest parsed readings for one station.
type siteMetrics struct {
	pH, tempC, turbNTU      float64
	hasPH, hasTemp, hasTurb bool
}

// ── Public entry point ────────────────────────────────────────────────────────

// FetchAndSyncWaterData discovers all active NY monitoring stations from the
// USGS API, parses their latest readings, and upserts every station that has
// at least one valid measurement into the water_entities collection.
func FetchAndSyncWaterData() {
	log.Println("[USGS] Fetching NY water quality data...")

	data, err := fetchUSGS()
	if err != nil {
		log.Printf("[USGS] Fetch error: %v", err)
		return
	}

	sites, metrics := parseAll(data)
	syncToMongo(sites, metrics)
}

// ── Fetch ─────────────────────────────────────────────────────────────────────

func fetchUSGS() (*usgsResponse, error) {
	resp, err := http.Get(usgsNYURL) //nolint:gosec
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

// parseAll walks every time series in the response, registers each site's
// metadata on first encounter, and accumulates the latest reading for each
// parameter. USGS sentinel values (≤ -900) are silently skipped.
func parseAll(data *usgsResponse) (map[string]*siteInfo, map[string]*siteMetrics) {
	sites   := make(map[string]*siteInfo)
	metrics := make(map[string]*siteMetrics)

	for _, ts := range data.Value.TimeSeries {
		if len(ts.SourceInfo.SiteCode) == 0 || len(ts.Variable.VariableCode) == 0 {
			continue
		}

		siteID    := ts.SourceInfo.SiteCode[0].Value
		paramCode := ts.Variable.VariableCode[0].Value

		// Register site metadata on first encounter.
		if _, ok := sites[siteID]; !ok {
			geo := ts.SourceInfo.GeoLocation.GeogLocation
			sites[siteID] = &siteInfo{
				ID:   siteID,
				Name: ts.SourceInfo.SiteName,
				Lat:  geo.Latitude,
				Lng:  geo.Longitude,
			}
			metrics[siteID] = &siteMetrics{}
		}

		// Skip if no recent value exists.
		if len(ts.Values) == 0 || len(ts.Values[0].Value) == 0 {
			continue
		}
		raw := ts.Values[0].Value[0].Value
		val, err := strconv.ParseFloat(raw, 64)
		if err != nil || val <= -900 { // USGS uses -999999 for missing/ice/etc.
			continue
		}

		m := metrics[siteID]
		switch paramCode {
		case paramPH:
			m.pH, m.hasPH = val, true
		case paramTempC:
			m.tempC, m.hasTemp = val, true
		case paramTurbNTU:
			m.turbNTU, m.hasTurb = val, true
		}
	}

	return sites, metrics
}

// ── Sync ──────────────────────────────────────────────────────────────────────

func syncToMongo(sites map[string]*siteInfo, metrics map[string]*siteMetrics) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	synced := 0
	for id, site := range sites {
		m := metrics[id]
		if !m.hasPH && !m.hasTemp && !m.hasTurb {
			continue // no usable data for this site
		}

		setFields := bson.M{
			"site_id": id,
			"name":    site.Name,
			"location": bson.M{
				"type":        "Point",
				"coordinates": bson.A{site.Lng, site.Lat},
			},
			"official_metrics.last_updated": time.Now().UTC(),
		}
		if m.hasPH   { setFields["official_metrics.ph"] = m.pH }
		if m.hasTemp { setFields["official_metrics.temp_c"] = m.tempC }
		if m.hasTurb { setFields["official_metrics.turbidity_ntu"] = m.turbNTU }

		_, err := db.EntitiesCollection.UpdateOne(ctx,
			bson.M{"site_id": id},
			bson.M{"$set": setFields},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			log.Printf("[USGS] Upsert failed for site %s: %v", id, err)
			continue
		}
		synced++
	}

	log.Printf("[USGS] Sync complete — %d NY sites upserted.", synced)
}
