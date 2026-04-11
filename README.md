# DeepBlue

Frontend is currently organized around a Go-served dashboard shell and a small set of HTMX-friendly component partials.

Theme decisions live in [themes.md](/Users/fuzi_x_muzi/Documents/Hackathon/deepblue/themes.md) and should be referenced during frontend changes.

## Recommended Structure

```text
client/templates/base.html   Shared HTML document shell
client/templates/index.html  Main dashboard page shell
client/templates/components/ Server-rendered UI partials for HTMX swaps
client/static/css/style.css  Root stylesheet that imports layout/component CSS
client/static/css/layout/    Page-level layout styles
client/static/css/components/ Card and section styling
client/static/js/app.js      Frontend bootstrap
client/static/js/map/        Leaflet-specific map setup and helpers
cmd/server/                  Go entrypoint and HTTP wiring
internal/handlers/           HTTP handlers for template rendering
```

## Frontend Notes

- Keep the four main dashboard elements as template partials: header, map, upload, alerts.
- Let HTMX own HTML swaps for stats, upload results, and alerts.
- Keep Leaflet logic isolated under `client/static/js/map` so map state does not get tangled with HTMX responses.
- Let the Go server render from `client/templates` and serve assets from `client/static`.
- If we later add a detail drawer, it should become another template partial rather than expanding the page shell.

## Local Run

```bash
go run ./cmd/server
```

Then open `http://localhost:8080`.
