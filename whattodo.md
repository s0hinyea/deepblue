# Frontend Hand-off & Polish Checklist (What To Do)

Welcome to the frontend role! The backend architecture is 100% complete. Claude and MongoDB are literally talking to each other right now through the Go API. 

Here is what you need to focus on to drive this home for the MVP presentation:

## 1. Setup Your Environment
- Pull the latest `main` branch so you have all the new HTTP routes.
- Ask for the `.env` file (containing the MongoDB URI, S3 Bucket, and AWS Keys). You'll need this to run the server on your local machine if you are testing endpoints.
- Run `cd backend` and `go run cmd/server/main.go` to test your changes against the live database.

## 2. HTMX & Endpoint Integration
The API routes are working, now we need to bind the UI firmly to them:
- [ ] **Leaflet Pins:** Ensure your map script fetches `GET /api/entities` on load to populate the map pins, and confirm the 30-second polling is active.
- [ ] **The AI Button:** On the bottom sheet that slides up, hook up the "Load AI Analysis" button. Use an HTMX `hx-get="/api/entity/{id}/advisory"` and `hx-target="#your-advisory-div"` to elegantly slide Claude's text into the UI without page reloads.
- [ ] **Image Upload Form:** Verify that the HTMX form for community reports points to `POST /api/reports` and is correctly using `enctype="multipart/form-data"` so the image bytes reach Go successfully.

## 3. UI/UX Polish (Crucial for Hackathons)
- **Loading Spinners:** Since the AI Advisory (`GET .../advisory`) takes about 1–3 seconds to run through Vector Search and Bedrock, make sure you use an `htmx-indicator` class to show a glowing loading spinner on the button! Judges hate freezing buttons.
- **Color Coding:** Ensure the map pins physically change color based on the `safety_label` or `safety_score` returned in `GET /api/entities` (e.g., Green = Safe, Orange = Moderate, Red = Dangerous).
- **Toast Notifications:** Make sure the upload form displays a slick "Upload Successful - AI is analyzing" toast message since the AI analysis happens asynchronously.

## 4. The "MongoDB Flex" (Bonus)
Since we are going for the MongoDB track:
- Put a small badge or text in the UI (perhaps on the Advisory panel) that says: **"Live Advisory powered by Atlas Vector Search & Change Streams"**. Judges love when you call out their tech visually!
