# Frontend Hand-off & Polish Checklist

Welcome! The backend is fully implemented and running. Here's everything you need to know to get up to speed and what needs fixing.

---

## 1. Getting Up to Speed

- Pull the `test_frontend` branch — that's the active branch with the latest frontend + all backend routes wired up
- Get the `.env` file from Sohil (MongoDB URI, AWS keys, S3 bucket name) — put it inside the `backend/` folder
- Run the server: `cd backend && go run cmd/server/main.go`
- Open `http://localhost:8080` — you should see a dark Leaflet map with NY water stations populating within a few seconds

**What the backend does automatically on startup:**
1. Connects to MongoDB Atlas
2. Fetches ~183 live NY water quality stations from USGS (pH, temp, turbidity) and upserts them into the DB — refreshes every 15 minutes
3. Starts a MongoDB change stream watcher — when a community report (photo) is submitted, it triggers Claude (AI) to analyze the image, compute a safety score, and update the station

---

## 2. Crucial Bug Fixes (Do These First)

### BUG 1 — Bottom sheet doesn't open on click (BLOCKER)
Clicking a map marker or a station card in the sidebar should slide up a detail panel from the bottom. It currently does nothing.
- The sheet element is `#detail-sheet` in `templates/index.html`
- The JS function `openSheet(id)` / `selectSite(id)` should be triggered on marker click and card click
- Debug by opening the browser console and checking for JS errors when you click a marker
- The sheet has CSS `transform: translateY(100%)` when closed and `translateY(0)` when open — verify the class toggle is actually firing

### BUG 2 — Remove debug logging from usgs.go (CLEANUP)
`backend/internal/services/usgs.go` has temporary debug log lines added during debugging. Clean them up:
- Remove the `eligible` counter variable and its log line
- Revert the final log line back to: `log.Printf("[USGS] Sync complete — %d NY sites upserted.", synced)`
- Remove the `| setFields=%v` from the upsert error log

---

## 3. UI/UX Polish

- **Color-coded map pins:** Change marker color based on `safety_label` from `GET /api/entities`. Green = SAFE, Orange = CAUTION, Red = UNSAFE, Grey = UNKNOWN
- **AI Advisory button:** In the bottom sheet, the "Load AI Analysis" button should fire `GET /api/entity/{id}/advisory` (HTMX or fetch). Show a loading spinner while it waits (takes 1–3 seconds). Display the returned `advisory` text in the sheet
- **Station cards:** Consider only showing stations that have at least one non-null metric in the sidebar to reduce clutter
- **MongoDB flex badge:** Add a small badge somewhere on the advisory panel: *"Powered by Atlas Vector Search + Change Streams"* — good for the MongoDB track judges

---

## 4. API Reference (Quick)

| Method | Route | Returns |
|--------|-------|---------|
| GET | `/api/entities` | JSON array of all water stations with metrics + safety_label |
| POST | `/api/reports` | Submit photo + site_id (multipart/form-data), triggers AI pipeline |
| GET | `/api/entity/{id}/advisory` | AI-generated 2-sentence water safety advisory for that station |

Each entity in `/api/entities` looks like:
```json
{
  "site_id": "01234567",
  "name": "HUDSON RIVER AT ALBANY NY",
  "location": { "type": "Point", "coordinates": [-73.75, 42.65] },
  "safety_label": "UNKNOWN",
  "safety_score": 0,
  "official_metrics": {
    "ph": 7.9,
    "temp_c": 12.5,
    "turbidity_ntu": 14.5,
    "last_updated": "2026-04-11T..."
  }
}
```
`ph`, `temp_c`, or `turbidity_ntu` may be absent if the station doesn't have that sensor — display `--` in the UI (already handled in current JS).

---

## 5. What's Already Working (Don't Break)

- Live USGS data sync every 15 min
- Community report upload → S3 → MongoDB insert → change stream → Claude image analysis → safety score update
- RAG pipeline: Titan embeddings + Atlas Vector Search over EPA/WHO knowledge base
- HTMX report form with toast feedback
- 30-second sidebar polling via `GET /api/entities`
- Dark Leaflet map with CartoDB tiles
- Bottom sheet CSS + animation (just the JS trigger is broken)
