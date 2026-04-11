# DeepBlue API Documentation

Welcome to the Frontend! The backend integration is fully wired. Here are the endpoints and data contracts you need to hook up HTMX and Leaflet.js.

## 1. Get Safety Advisory (The AI Brain)

Triggers the RAG pipeline to generate a real-time, localized safety advisory using live sensor data and the EPA/WHO knowledge base.

*   **URL:** `/api/entity/{id}/advisory`
*   **Method:** `GET`
*   **Response Format:** `JSON`

### Example Request
```bash
curl -s http://localhost:8080/api/entity/04085427/advisory
```

### Example Response
```json
{
    "entity": "Manitowoc River at Manitowoc WI",
    "advisory": "The Manitowoc River currently has a water pH of 7.80, which is within the safe range... caution is advised due to the elevated turbidity levels."
}
```

### Frontend Integration Tips (HTMX):
For the side-drawer UI, you can easily wire this up with an HTMX `hx-get` on your Leaflet map markers. 
```html
<button hx-get="/api/entity/04085427/advisory" hx-target="#advisory-panel">
   Load AI Analysis
</button>
```

---

## 2. Community Photo Uploads & Watcher Trigger

Right now, image uploads go straight from Go to S3, but we also rely on **MongoDB Change Streams**. 

When you build the photo upload form, you just need to POST the image file to the backend. The backend will update MongoDB, and the `Watcher` will immediately trigger Claude Vision to analyze the photo in the background.

*As a frontend developer, you do not need to wait for the AI analysis to finish on upload. The background watcher handles it asynchronously!*
