# DeepBlue

Real-time water safety intelligence for New York State. DeepBlue fuses live USGS sensor data with community photo reports and AI analysis to give the public an up-to-date picture of water quality at 180+ monitoring stations.

---

## What it does

- **Live map** of NY water stations with safety labels (Safe / Moderate / Dangerous)
- **Community reports** — upload a photo of a water body and Claude AI analyzes it for algae, pollution, turbidity, and other risk signals
- **AI Advisory** — click any station to generate a plain-English safety advisory grounded in EPA/WHO guidelines and recent community submissions
- **Automatic updates** — safety scores update in real-time as new photos are submitted

---

## Stack

- **Backend** — Go, MongoDB Atlas, AWS Bedrock (Claude 3 Haiku + Titan Embeddings), AWS S3
- **Frontend** — Leaflet.js, HTMX, vanilla JS
- **Data** — USGS Water Services API (live, refreshes every 15 min)

---

## Running locally

**Prerequisites:** Go 1.22+, a MongoDB Atlas cluster, AWS account with Bedrock + S3 access.

**1. Clone the repo**
```bash
git clone https://github.com/s0hinyea/deepblue.git
cd deepblue
```

**2. Create `backend/.env`**
```
MONGODB_URI=your_atlas_connection_string
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret
AWS_REGION=us-east-1
S3_BUCKET_NAME=your_bucket
```

**3. Run**
```bash
cd backend
go run cmd/server/main.go
```

Open [http://localhost:8080](http://localhost:8080)

---

## Using the app

1. The map loads with live NY water stations — click any marker or sidebar card to see its metrics
2. Hit **+ Report** on a station to upload a photo — the AI pipeline runs automatically
3. After ~5 seconds the station's safety label updates on the map
4. Click **✦ AI Advisory** for a detailed safety assessment with two sections: one from sensor data + guidelines, one from community reports
