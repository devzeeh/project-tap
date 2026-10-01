package docs

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed openapi.json
var openAPISpec []byte

// RegisterRoutes registers the documentation endpoints on the provided http.ServeMux.
func RegisterRoutes(mux *http.ServeMux) {
	// Serve raw OpenAPI 3.0 specification
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(openAPISpec)
	})

	// Serve Scalar API Reference UI
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(scalarHTML))
	})
	mux.HandleFunc("GET /docs/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs", http.StatusMovedPermanently)
	})

	// Serve Swagger UI
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerHTML))
	})
	mux.HandleFunc("GET /swagger/", func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimRight(r.URL.Path, "/") == "/swagger" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(swaggerHTML))
			return
		}
		http.NotFound(w, r)
	})
}

const scalarHTML = `<!doctype html>
<html lang="en">
  <head>
    <title>Project TAP - Interactive API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%236366f1'><path d='M21 16.5c0 .38-.21.71-.53.88l-7.9 4.44c-.16.12-.36.18-.57.18s-.41-.06-.57-.18l-7.9-4.44A.991.991 0 0 1 3 16.5v-9c0-.38.21-.71.53-.88l7.9-4.44c.16-.12.36-.18.57-.18s.41.06.57.18l7.9 4.44c.32.17.53.5.53.88v9z'/></svg>" />
    <style>
      body {
        margin: 0;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
      }
      .docs-nav-bar {
        background: #0f172a;
        color: #f8fafc;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 10px 20px;
        font-size: 14px;
        border-bottom: 1px solid #1e293b;
      }
      .docs-nav-brand {
        display: flex;
        align-items: center;
        gap: 10px;
        font-weight: 700;
        letter-spacing: -0.01em;
      }
      .docs-nav-brand span.badge {
        background: #4f46e5;
        color: white;
        padding: 2px 8px;
        border-radius: 9999px;
        font-size: 11px;
        font-weight: 600;
      }
      .docs-nav-links {
        display: flex;
        align-items: center;
        gap: 14px;
      }
      .docs-nav-links a {
        color: #94a3b8;
        text-decoration: none;
        padding: 5px 12px;
        border-radius: 6px;
        transition: all 0.2s ease;
        font-weight: 500;
      }
      .docs-nav-links a:hover {
        color: #ffffff;
        background: #1e293b;
      }
      .docs-nav-links a.active {
        color: #ffffff;
        background: #334155;
      }
    </style>
  </head>
  <body>
    <div class="docs-nav-bar">
      <div class="docs-nav-brand">
        <span>💳 Project TAP API</span>
        <span class="badge">v1.0.0</span>
      </div>
      <div class="docs-nav-links">
        <a href="/docs" class="active">Scalar Docs</a>
        <a href="/swagger">Swagger UI</a>
        <a href="/openapi.json" target="_blank">OpenAPI JSON</a>
      </div>
    </div>

    <script
      id="api-reference"
      data-url="/openapi.json"
      data-configuration='{"theme": "deepSpace", "hideModels": false, "showSidebar": true}'
    ></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@latest/dist/browser/standalone.min.js"></script>
  </body>
</html>
`

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Project TAP - Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%236366f1'><path d='M21 16.5c0 .38-.21.71-.53.88l-7.9 4.44c-.16.12-.36.18-.57.18s-.41-.06-.57-.18l-7.9-4.44A.991.991 0 0 1 3 16.5v-9c0-.38.21-.71.53-.88l7.9-4.44c.16-.12.36-.18.57-.18s.41.06.57.18l7.9 4.44c.32.17.53.5.53.88v9z'/></svg>" />
  <style>
    html {
      box-sizing: border-box;
      overflow-y: scroll;
    }
    *, *:before, *:after {
      box-sizing: inherit;
    }
    body {
      margin: 0;
      background: #fafafa;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    }
    .docs-nav-bar {
      background: #1b1b1b;
      color: #f8fafc;
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 10px 20px;
      font-size: 14px;
      border-bottom: 1px solid #333;
    }
    .docs-nav-brand {
      display: flex;
      align-items: center;
      gap: 10px;
      font-weight: 700;
      letter-spacing: -0.01em;
    }
    .docs-nav-brand span.badge {
      background: #4f46e5;
      color: white;
      padding: 2px 8px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 600;
    }
    .docs-nav-links {
      display: flex;
      align-items: center;
      gap: 14px;
    }
    .docs-nav-links a {
      color: #94a3b8;
      text-decoration: none;
      padding: 5px 12px;
      border-radius: 6px;
      transition: all 0.2s ease;
      font-weight: 500;
    }
    .docs-nav-links a:hover {
      color: #ffffff;
      background: #333;
    }
    .docs-nav-links a.active {
      color: #ffffff;
      background: #4f46e5;
    }
    .swagger-ui .topbar {
      display: none !important;
    }
  </style>
</head>
<body>
  <div class="docs-nav-bar">
    <div class="docs-nav-brand">
      <span>💳 Project TAP API</span>
      <span class="badge">v1.0.0</span>
    </div>
    <div class="docs-nav-links">
      <a href="/docs">Scalar Docs</a>
      <a href="/swagger" class="active">Swagger UI</a>
      <a href="/openapi.json" target="_blank">OpenAPI JSON</a>
    </div>
  </div>

  <div id="swagger-ui"></div>

  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
  window.onload = function() {
    window.ui = SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: '#swagger-ui',
      deepLinking: true,
      persistAuthorization: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIStandalonePreset
      ],
      plugins: [
        SwaggerUIBundle.plugins.DownloadUrl
      ],
      layout: "StandaloneLayout"
    });
  };
  </script>
</body>
</html>
`
