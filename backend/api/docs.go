// Package api serves the OpenAPI description of the REST API and an
// interactive Swagger UI for it.
package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var spec []byte

// Swagger UI from a pinned CDN build, pointed at the embedded spec.
const docsPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Agentic RFP API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="docs"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#docs", persistAuthorization: true })</script>
</body>
</html>`

// RegisterRoutes wires the docs. They are public, like the spec they describe.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(spec)
	})
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(docsPage))
	})
}
