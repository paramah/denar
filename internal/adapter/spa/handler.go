// Package spa serves the built web panel: its files, and index.html for
// every address the panel's own router handles.
package spa

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"
)

const (
	indexName    = "index.html"
	assetsPrefix = "assets/" // files named after their content by the build

	htmlType = "text/html; charset=utf-8"

	// contentSecurityPolicy lets the panel load only what the instance
	// serves. Inline style attributes are allowed, style and script
	// elements are not.
	contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; " +
		"img-src 'self'; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; " +
		"form-action 'self'; frame-ancestors 'none'"
	permissionsPolicy = "camera=(), microphone=(), geolocation=()"
)

// reserved are the first path segments that belong to the other adapters.
// The root mux does not route them here; refusing them keeps a mistake
// there from answering an API client with the panel.
var reserved = []string{"api", "healthz", "mcp", "events"}

// contentTypes are set by hand, because the table of the mime package
// depends on the system the server runs on.
var contentTypes = map[string]string{
	".html":        htmlType,
	".js":          "text/javascript; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".css":         "text/css; charset=utf-8",
	".json":        "application/json",
	".map":         "application/json",
	".webmanifest": "application/manifest+json",
	".txt":         "text/plain; charset=utf-8",
	".svg":         "image/svg+xml",
	".png":         "image/png",
	".jpg":         "image/jpeg",
	".jpeg":        "image/jpeg",
	".gif":         "image/gif",
	".webp":        "image/webp",
	".avif":        "image/avif",
	".ico":         "image/x-icon",
	".woff2":       "font/woff2",
	".woff":        "font/woff",
	".ttf":         "font/ttf",
	".wasm":        "application/wasm",
}

// placeholder is the page served while web/dist holds no build.
var placeholder = []byte(`<!doctype html>
<html lang="pl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Denar</title>
</head>
<body style="font-family: system-ui, sans-serif; max-width: 40rem; margin: 3rem auto; padding: 0 1rem; line-height: 1.5">
<h1>Denar</h1>
<p>Serwer działa. Interfejs WWW nie jest jeszcze zbudowany (powstaje w etapie 007).</p>
<p>API REST jest dostępne pod <code>/api</code>, np. <a href="/api/auth/config">/api/auth/config</a>;
stan serwera: <a href="/healthz">/healthz</a>.</p>
</body>
</html>
`)

type file struct {
	content     []byte
	contentType string
	etag        string // quoted SHA-256 of the content
}

type handler struct {
	files map[string]file // by slash-separated name, without names starting with a dot
}

// Handler serves the files of dist, the output of the web build, and
// answers the addresses of the panel's router with index.html. It reads
// dist once. Without an index.html in it, as before the web UI of stage
// 007 is built, it serves a built-in placeholder page instead and logs so.
// A nil logger means slog.Default.
func Handler(dist fs.FS, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handler{files: map[string]file{}}
	err := fs.WalkDir(dist, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		content, err := fs.ReadFile(dist, name)
		if err != nil {
			return err
		}
		contentType, ok := contentTypes[strings.ToLower(path.Ext(name))]
		if !ok {
			contentType = "application/octet-stream"
		}
		sum := sha256.Sum256(content)
		h.files[name] = file{content: content, contentType: contentType, etag: `"` + hex.EncodeToString(sum[:]) + `"`}
		return nil
	})
	if err != nil {
		logger.Error("reading the web panel files", slog.Any("error", err))
	}
	if _, ok := h.files[indexName]; !ok {
		logger.Info("the web UI is not built; serving the placeholder page (run task web:build and build the server again)")
		sum := sha256.Sum256(placeholder)
		h.files[indexName] = file{content: placeholder, contentType: htmlType, etag: `"` + hex.EncodeToString(sum[:]) + `"`}
	}
	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("X-Content-Type-Options", "nosniff")

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		header.Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, ok := fileName(r.URL.Path)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, found := h.files[name]
	if !found {
		// A missing file is not found; any other address is the router's.
		if name != indexName && path.Ext(name) != "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		name = indexName
		f = h.files[name]
	}

	header.Set("Content-Type", f.contentType)
	if strings.HasPrefix(name, assetsPrefix) {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "no-cache")
		header.Set("ETag", f.etag)
	}
	if f.contentType == htmlType {
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "same-origin")
		header.Set("Permissions-Policy", permissionsPolicy)
	}
	// Embedded files have no modification time, so there is no
	// Last-Modified; the ETag validates.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(f.content))
}

// fileName turns the path of a request into the name of the file it asks
// for, index.html for the root. It refuses paths no panel address has:
// with a backslash, a NUL byte or a segment starting with a dot, which
// covers "..", and the paths of the other adapters.
func fileName(urlPath string) (name string, ok bool) {
	if strings.ContainsAny(urlPath, "\\\x00") {
		return "", false
	}
	for _, segment := range strings.Split(urlPath, "/") {
		if strings.HasPrefix(segment, ".") {
			return "", false
		}
	}
	name = strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if first, _, _ := strings.Cut(name, "/"); slices.Contains(reserved, first) {
		return "", false
	}
	if name == "" {
		name = indexName
	}
	return name, true
}
