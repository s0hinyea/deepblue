# DeepBlue: The Full Technical Architecture

DeepBlue is a full-stack, real-time water safety platform built using Go, HTMX, MongoDB Atlas, and AWS Bedrock. 

This document explains the end-to-end flow of how the system works.

## 1. The Knowledge Base (RAG Preparation)
To ensure our AI doesn't hallucinate, we pre-seeded a MongoDB collection (`knowledge_chunks`) with official EPA and WHO water safety guidelines. 
Each paragraph was converted into a 1024-dimensional vector using AWS Bedrock's *Titan Embed v2* model and stored with an Atlas Vector Search index.

## 2. Ingestion: The Background Metronome
Our Go server runs an infinite background loop (`FetchAndSyncWaterData`). Every 15 minutes, it reaches out to the USGS API to fetch live sensor data (pH, turbidity, temperature) for our monitored lakes and rivers in NY and WI. 
It uses a MongoDB `$set` with `upsert: true` to silently update the `water_entities` collection so our map always has live data without blocking the web server.

## 3. The Frontend Viewer
When a user visits our site (`GET /`), they are served a dark-themed Leaflet.js map. 
The frontend hits `GET /api/entities` to pull the latest coordinates and safety scores from MongoDB, dropping colored pins on the map. The sidebar automatically polls for updates every 30 seconds.

## 4. Community Reports
If a user is standing by a lake and sees something gross (like green algae), they use the HTMX form in the sidebar to upload a photo and select the location.
This hits `POST /api/reports`:
1. Go receives the image file.
2. Go uploads the image byte-stream to AWS S3.
3. AWS S3 returns a public URL for the image.
4. Go inserts a shiny new document containing that S3 URL into the `community_reports` MongoDB collection.

## 5. The Reactive AI "Watcher" (Change Streams)
This is the magic phase. We do **not** make the user wait for the AI to finish analyzing their photo while they look at a loading spinner.
Instead, we have a Go goroutine running `WatchReports()`. This uses **MongoDB Change Streams** to constantly listen to the `community_reports` collection.
1. The exact millisecond the new report is saved, the Watcher triggers.
2. It grabs the S3 URL and passes it to Claude 3.5 Haiku via AWS Bedrock.
3. Claude analyzes the image and returns visual danger tags (e.g., `["algae", "turbid"]`).
4. Go takes those tags, recalculates the lake's `safety_score`, and updates the parent `water_entities` document in MongoDB.

## 6. Localized AI Advisory Generation
When a user clicks on a map pin to learn more, a bottom sheet slides up, and they can click "Load AI Analysis". This hits `GET /api/entity/{id}/advisory`.
1. Go fetches the live sensor data for that lake.
2. It uses Titan Embed v2 to convert the sensor data into a mathematical query.
3. It performs a `$vectorSearch` on MongoDB to find the relevant EPA/WHO safety rules.
4. It passes the raw sensor data AND the EPA rules to Claude 3.5 Haiku.
5. Claude generates a strictly accurate, context-aware, 2-sentence public safety advisory, which HTMX injects straight into the UI.