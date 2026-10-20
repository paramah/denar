package spa

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	indexHTML = `<!doctype html><html lang="pl"><title>Denar</title><div id="root"></div></html>`
	appJS     = `console.log("denar")`

	wantCSP = "default-src 'self'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; " +
		"img-src 'self'; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; " +
		"form-action 'self'; frame-ancestors 'none'"
	immutable = "public, max-age=31536000, immutable"
)

func dist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":             {Data: []byte(indexHTML)},
		"favicon.svg":            {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"theme-init.js":          {Data: []byte(`document.documentElement.dataset.theme = "dark"`)},
		"assets/app-3f9a1c.js":   {Data: []byte(appJS)},
		"assets/app-77b0e2.css":  {Data: []byte(`body { margin: 0 }`)},
		"assets/inter-a1.woff2":  {Data: []byte("wOF2 not a real font")},
		".gitkeep":               {Data: []byte{}},
		"assets/.hidden":         {Data: []byte("secret")},
		"assets/data.unknownext": {Data: []byte("<html>not html</html>")},
	}
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// serve sends one request to a handler over the test files. The target is
// what a client puts in the request line, so it may hold escapes.
func serve(t *testing.T, h http.Handler, method, target string, header ...string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	res.Request = req
	return res
}

func body(t *testing.T, res *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wantHeader(t *testing.T, res *http.Response, name, want string) {
	t.Helper()
	if got := res.Header.Get(name); got != want {
		t.Errorf("%s %s: %s = %q, want %q", res.Request.Method, res.Request.URL, name, got, want)
	}
}

// wantNotHTML checks an answer that must not be taken for the panel.
func wantNotHTML(t *testing.T, res *http.Response, status int) {
	t.Helper()
	if res.StatusCode != status {
		t.Errorf("%s %s: status = %d, want %d", res.Request.Method, res.Request.URL, res.StatusCode, status)
	}
	wantHeader(t, res, "Content-Type", "text/plain; charset=utf-8")
	wantHeader(t, res, "X-Content-Type-Options", "nosniff")
	wantHeader(t, res, "Content-Security-Policy", "")
	if b := body(t, res); strings.Contains(b, "<html") {
		t.Errorf("%s %s: the body is the panel: %q", res.Request.Method, res.Request.URL, b)
	}
}

func TestPanelAddressesGetIndexHTML(t *testing.T) {
	h := Handler(dist(), discard())
	for _, target := range []string{
		"/", "/index.html", "/projects/abc", "/projects/abc/", "/issues/RZL-12/edit", "/issues/RZL-12?tab=activity",
		"//projects", "/apiary", "/eventsource", "/mcpx/y", "/assets",
	} {
		res := serve(t, h, http.MethodGet, target)
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", target, res.StatusCode)
		}
		if got := body(t, res); got != indexHTML {
			t.Errorf("GET %s: body = %q, want index.html", target, got)
		}
		wantHeader(t, res, "Content-Type", "text/html; charset=utf-8")
		wantHeader(t, res, "Cache-Control", "no-cache")
		wantHeader(t, res, "Content-Security-Policy", wantCSP)
		wantHeader(t, res, "X-Frame-Options", "DENY")
		wantHeader(t, res, "X-Content-Type-Options", "nosniff")
		wantHeader(t, res, "Referrer-Policy", "same-origin")
		wantHeader(t, res, "Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if res.Header.Get("ETag") == "" {
			t.Errorf("GET %s: no ETag", target)
		}
		if got := res.Header.Get("Last-Modified"); got != "" {
			t.Errorf("GET %s: Last-Modified = %q, want none", target, got)
		}
	}
}

func TestFilesAreServedWithTheirTypeAndCachePolicy(t *testing.T) {
	h := Handler(dist(), discard())
	for _, tt := range []struct {
		target      string
		contentType string
		cache       string
		etag        bool
	}{
		{"/assets/app-3f9a1c.js", "text/javascript; charset=utf-8", immutable, false},
		{"/assets/app-77b0e2.css", "text/css; charset=utf-8", immutable, false},
		{"/assets/inter-a1.woff2", "font/woff2", immutable, false},
		{"/assets/data.unknownext", "application/octet-stream", immutable, false},
		{"/favicon.svg", "image/svg+xml", "no-cache", true},
		{"/theme-init.js", "text/javascript; charset=utf-8", "no-cache", true},
	} {
		res := serve(t, h, http.MethodGet, tt.target)
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", tt.target, res.StatusCode)
			continue
		}
		wantHeader(t, res, "Content-Type", tt.contentType)
		wantHeader(t, res, "Cache-Control", tt.cache)
		wantHeader(t, res, "X-Content-Type-Options", "nosniff")
		for _, onlyHTML := range []string{"Content-Security-Policy", "X-Frame-Options", "Referrer-Policy", "Permissions-Policy"} {
			wantHeader(t, res, onlyHTML, "")
		}
		if got := res.Header.Get("ETag") != ""; got != tt.etag {
			t.Errorf("GET %s: ETag = %q, want one: %v", tt.target, res.Header.Get("ETag"), tt.etag)
		}
		want, _ := dist().ReadFile(strings.TrimPrefix(tt.target, "/"))
		if got := body(t, res); got != string(want) {
			t.Errorf("GET %s: body = %q, want %q", tt.target, got, want)
		}
	}
}

func TestWhatIsNotAPanelAddressIsNotFound(t *testing.T) {
	h := Handler(dist(), discard())
	for _, target := range []string{
		// Missing files: the last segment has an extension.
		"/assets/nie-ma.js", "/x.png", "/projects/abc.json", "/index.htm",
		// Names starting with a dot, existing or not.
		"/.gitkeep", "/assets/.hidden", "/.well-known/security.txt", "/projects/.env",
		// Ways out of the directory.
		"/..", "/../index.html", "/assets/../index.html", "/..%2f", "/..%2findex.html", "/assets/..%2f..%2findex.html",
		"/%2e%2e/index.html", "/a/./b",
		// Characters no panel address has.
		`/assets\app-3f9a1c.js`, "/assets%5capp-3f9a1c.js", "/a%00b", "/index.html%00",
		// Addresses of the other adapters.
		"/api", "/api/", "/api/x", "/api/issues/RZL-1", "/healthz", "/healthz/x", "/mcp", "/mcp/x", "/events", "/events/x",
		"//api/x", "/./api/x",
	} {
		wantNotHTML(t, serve(t, h, http.MethodGet, target), http.StatusNotFound)
	}
}

func TestOnlyGetAndHeadAreAllowed(t *testing.T) {
	h := Handler(dist(), discard())
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		for _, target := range []string{"/", "/projects/abc", "/assets/app-3f9a1c.js"} {
			res := serve(t, h, method, target)
			wantNotHTML(t, res, http.StatusMethodNotAllowed)
			wantHeader(t, res, "Allow", "GET, HEAD")
		}
	}
}

func TestHeadGivesTheHeadersWithoutTheBody(t *testing.T) {
	h := Handler(dist(), discard())
	for target, length := range map[string]int{"/": len(indexHTML), "/issues/RZL-12": len(indexHTML), "/assets/app-3f9a1c.js": len(appJS)} {
		get := serve(t, h, http.MethodGet, target)
		head := serve(t, h, http.MethodHead, target)
		if head.StatusCode != http.StatusOK {
			t.Errorf("HEAD %s: status = %d, want 200", target, head.StatusCode)
		}
		if b := body(t, head); b != "" {
			t.Errorf("HEAD %s: body = %q, want none", target, b)
		}
		wantHeader(t, head, "Content-Length", strconv.Itoa(length))
		for _, name := range []string{"Content-Type", "Cache-Control", "ETag", "Content-Security-Policy", "X-Content-Type-Options"} {
			wantHeader(t, head, name, get.Header.Get(name))
		}
	}
	wantNotHTML(t, serve(t, h, http.MethodHead, "/x.png"), http.StatusNotFound)
}

func TestIfNoneMatchGivesNotModified(t *testing.T) {
	h := Handler(dist(), discard())
	etags := map[string]string{}
	for _, target := range []string{"/", "/projects/abc", "/favicon.svg", "/theme-init.js"} {
		etag := serve(t, h, http.MethodGet, target).Header.Get("ETag")
		if etag == "" {
			t.Fatalf("GET %s: no ETag", target)
		}
		etags[target] = etag

		res := serve(t, h, http.MethodGet, target, "If-None-Match", etag)
		if res.StatusCode != http.StatusNotModified {
			t.Errorf("GET %s with its ETag: status = %d, want 304", target, res.StatusCode)
		}
		if b := body(t, res); b != "" {
			t.Errorf("GET %s with its ETag: body = %q, want none", target, b)
		}
		wantHeader(t, res, "ETag", etag)
		wantHeader(t, res, "Cache-Control", "no-cache")

		res = serve(t, h, http.MethodGet, target, "If-None-Match", `"another"`)
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s with another ETag: status = %d, want 200", target, res.StatusCode)
		}
	}
	if etags["/"] != etags["/projects/abc"] {
		t.Errorf("index.html has the ETags %s and %s", etags["/"], etags["/projects/abc"])
	}
	if etags["/"] == etags["/favicon.svg"] || etags["/favicon.svg"] == etags["/theme-init.js"] {
		t.Errorf("different files share an ETag: %v", etags)
	}

	// The ETag follows the content, as there are no modification times.
	changed := dist()
	changed["index.html"] = &fstest.MapFile{Data: []byte(indexHTML + "<!-- v2 -->")}
	if got := serve(t, Handler(changed, discard()), http.MethodGet, "/").Header.Get("ETag"); got == etags["/"] {
		t.Errorf("a changed index.html kept the ETag %s", got)
	}
	if got := serve(t, Handler(dist(), discard()), http.MethodGet, "/").Header.Get("ETag"); got != etags["/"] {
		t.Errorf("the same index.html got the ETag %s, then %s", etags["/"], got)
	}
}

func TestRangeRequestsGetAPart(t *testing.T) {
	res := serve(t, Handler(dist(), discard()), http.MethodGet, "/assets/app-3f9a1c.js", "Range", "bytes=0-6")
	if res.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", res.StatusCode)
	}
	if got := body(t, res); got != appJS[:7] {
		t.Errorf("body = %q, want %q", got, appJS[:7])
	}
}

func TestWithoutIndexHTMLAPlaceholderIsServed(t *testing.T) {
	files := dist()
	delete(files, "index.html")
	var log bytes.Buffer
	h := Handler(files, slog.New(slog.NewTextHandler(&log, nil)))

	if !strings.Contains(log.String(), "task web:build") {
		t.Errorf("log = %q, want a note that names task web:build", log.String())
	}
	for _, target := range []string{"/", "/households/abc", "/index.html"} {
		res := serve(t, h, http.MethodGet, target)
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", target, res.StatusCode)
		}
		wantHeader(t, res, "Content-Type", "text/html; charset=utf-8")
		wantHeader(t, res, "X-Content-Type-Options", "nosniff")
		if res.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("GET %s: no CSP", target)
		}
		if b := body(t, res); !strings.Contains(b, "Denar") || !strings.Contains(b, "/api") {
			t.Errorf("GET %s: body = %q, want the placeholder", target, b)
		}
	}
	// The rest keeps working.
	if res := serve(t, h, http.MethodGet, "/favicon.svg"); res.StatusCode != http.StatusOK {
		t.Errorf("GET /favicon.svg: status = %d, want 200", res.StatusCode)
	}
	wantNotHTML(t, serve(t, h, http.MethodGet, "/x.png"), http.StatusNotFound)
	wantNotHTML(t, serve(t, h, http.MethodGet, "/api/x"), http.StatusNotFound)

	log.Reset()
	Handler(dist(), slog.New(slog.NewTextHandler(&log, nil)))
	if log.Len() != 0 {
		t.Errorf("a built panel logs %q, want nothing", log.String())
	}
}

func TestEmptyFileSystemOnlyHasGitkeep(t *testing.T) {
	h := Handler(fstest.MapFS{".gitkeep": {Data: []byte{}}}, discard())
	if res := serve(t, h, http.MethodGet, "/"); res.StatusCode != http.StatusOK {
		t.Errorf("GET /: status = %d, want the placeholder", res.StatusCode)
	}
	wantNotHTML(t, serve(t, h, http.MethodGet, "/.gitkeep"), http.StatusNotFound)
}
