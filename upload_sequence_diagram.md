# DeepBlue Upload Sequence Diagram

This Mermaid sequence diagram shows the full path from a user uploading an image to the frontend showing a changed safety label and marker color.

You can paste the Mermaid block below into [Mermaid Live Editor](https://mermaid.live) to preview it and export a PNG or SVG.

```mermaid
sequenceDiagram
    title DeepBlue: Image Upload -> Async AI Processing -> Client UI Update

    actor User
    participant DOM as Browser DOM / HTMX Form
    participant Server as Go HTTP Server
    participant Reports as ReportsHandler
    participant S3Service as S3 Upload Service
    participant S3 as AWS S3 Bucket
    participant ReportsDB as MongoDB community_reports
    participant Watcher as WatchReports Change Stream
    participant AI as AI Service
    participant Bedrock as AWS Bedrock Claude
    participant EntitiesDB as MongoDB water_entities
    participant Poll as 30s entities poll

    rect rgb(238, 246, 255)
        Note over User,ReportsDB: Upload Path

        User->>DOM: Select station, choose image, click Submit Report
        activate DOM

        DOM->>Server: POST /api/reports (multipart/form-data)
        activate Server

        Server->>Reports: ReportsHandler(r)
        activate Reports

        Reports->>S3Service: UploadImage(file bytes, filename)
        activate S3Service
        S3Service->>S3: PutObject(bucket, key, body)
        activate S3
        S3-->>S3Service: Public image URL
        deactivate S3
        S3Service-->>Reports: imageURL
        deactivate S3Service

        Reports->>ReportsDB: InsertOne({ site_id, image_url, created_at })
        activate ReportsDB
        ReportsDB-->>Reports: Insert acknowledged
        deactivate ReportsDB

        Reports-->>Server: 200 OK + "Report received - AI analysis starting..."
        deactivate Reports
        Server-->>DOM: HTMX swaps status area and resets form
        deactivate Server
        deactivate DOM
    end

    rect rgb(244, 255, 244)
        Note over ReportsDB,EntitiesDB: Async AI Processing Path

        Note over Watcher,ReportsDB: The watcher is event-driven, not polled.<br/>It reacts as soon as MongoDB emits the insert event.

        ReportsDB->>Watcher: Change stream insert event (fullDocument)
        activate Watcher

        Watcher->>Watcher: Extract report_id, site_id, image_url

        Watcher->>AI: AnalyzeImageLabels(imageURL)
        activate AI

        AI->>S3: HTTP GET imageURL
        activate S3
        S3-->>AI: Image bytes
        deactivate S3

        AI->>AI: Detect media type and base64-encode image

        AI->>Bedrock: InvokeModel(base64 image + prompt)
        activate Bedrock
        Bedrock-->>AI: JSON tags (for example ["algae", "turbid"])
        deactivate Bedrock

        AI-->>Watcher: ai_tags[]
        deactivate AI

        Watcher->>ReportsDB: UpdateOne(report_id, set ai_tags)
        activate ReportsDB
        ReportsDB-->>Watcher: Update acknowledged
        deactivate ReportsDB

        Watcher->>EntitiesDB: FindOne({ site_id })
        activate EntitiesDB
        EntitiesDB-->>Watcher: Water entity + official metrics

        Watcher->>Watcher: Compute visualRiskScore(tags)
        Watcher->>Watcher: Compute safety_score and safety_label

        Watcher->>EntitiesDB: UpdateOne({ site_id }, set safety_score, safety_label, last_report_at, inc report_count)
        EntitiesDB-->>Watcher: Update acknowledged
        deactivate EntitiesDB

        Watcher-->>Watcher: Log completion
        deactivate Watcher
    end

    rect rgb(255, 248, 236)
        Note over DOM,Poll: UI Refresh Path

        loop Every 30 seconds
            DOM->>Poll: fetchEntities()
            activate Poll
            Poll->>Server: GET /api/entities
            activate Server
            Server->>EntitiesDB: Find(all entities)
            activate EntitiesDB
            EntitiesDB-->>Server: Entity list
            deactivate EntitiesDB
            Server-->>Poll: 200 OK + entities JSON
            deactivate Server
            Poll-->>DOM: Updated entities array
            deactivate Poll

            DOM->>DOM: Re-render sidebar cards and labels
            DOM->>DOM: Re-render map marker colors and icons
        end

        Note over DOM: User sees the updated label and map dot coloring on the next entities poll after the watcher updates water_entities.
    end
```
