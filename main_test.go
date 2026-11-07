package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gorilla/sessions"
	"github.com/mdbot/wiki/config"
	"github.com/mdbot/wiki/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()

	// The backend lower cases full paths, so it can't use t.TempDir (which contains the test name).
	dir, err := os.MkdirTemp("", "wiki")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	backend, err := NewGitBackend(dir)
	require.NoError(t, err)
	require.NoError(t, backend.PutPage("testpage", []byte("Fixture page body"), "tester", "Add fixture"))

	store := config.NewStore(backend, "")
	userManager, err := config.NewUserManager(store)
	require.NoError(t, err)
	siteConfig, err := config.LoadSite(store)
	require.NoError(t, err)

	pm := &PermissionChecker{}
	templates := &Templates{
		fs:              os.DirFS("resources/templates"),
		siteConfig:      siteConfig,
		checker:         pm,
		sidebarProvider: func() string { return "" },
	}
	static := fstest.MapFS{
		"Mixed.css": {Data: []byte("fixture stylesheet")},
	}

	logs := &bytes.Buffer{}
	return newHandler(
		templates,
		pm,
		backend,
		markdown.NewRenderer(backend, false, "monokai"),
		userManager,
		siteConfig,
		sessions.NewCookieStore(bytes.Repeat([]byte{1}, 32)),
		static,
		logs,
	), logs
}

func serve(
	t *testing.T,
	router http.Handler,
	method, target string,
	body url.Values,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != nil {
		req = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func TestRouter(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		target    string
		body      url.Values
		headers   map[string]string
		status    int
		location  string
		contains  string
		errorPage bool
	}{
		{
			name:     "root redirects to main page",
			method:   http.MethodGet,
			target:   "/",
			status:   http.StatusSeeOther,
			location: "/view/MainPage",
		},
		{
			name:     "root redirects for any method",
			method:   http.MethodPost,
			target:   "/",
			status:   http.StatusSeeOther,
			location: "/view/MainPage",
		},
		{
			name:     "view root has its trailing slash stripped first",
			method:   http.MethodGet,
			target:   "/view/",
			status:   http.StatusPermanentRedirect,
			location: "/view",
		},
		{
			name:      "view without slash is not found",
			method:    http.MethodGet,
			target:    "/view",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:     "view shows page",
			method:   http.MethodGet,
			target:   "/view/testpage",
			status:   http.StatusOK,
			contains: "Fixture page body",
		},
		{
			name:     "view keeps query when lower casing",
			method:   http.MethodGet,
			target:   "/view/TestPage?rev=x",
			status:   http.StatusPermanentRedirect,
			location: "/view/testpage?rev=x",
		},
		{
			name:   "view rejects post",
			method: http.MethodPost,
			target: "/view/testpage",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:      "view of missing page renders not found",
			method:    http.MethodGet,
			target:    "/view/missing",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:     "page trailing slash is stripped",
			method:   http.MethodGet,
			target:   "/view/testpage/",
			status:   http.StatusPermanentRedirect,
			location: "/view/testpage",
		},

		{
			name:     "edit get shows form",
			method:   http.MethodGet,
			target:   "/edit/testpage",
			status:   http.StatusOK,
			contains: "Fixture page body",
		},
		{
			name:     "edit post saves page",
			method:   http.MethodPost,
			target:   "/edit/newpage",
			body:     url.Values{"content": {"x"}},
			status:   http.StatusSeeOther,
			location: "/view/newpage",
		},
		{
			name:   "head is served by get routes",
			method: http.MethodHead,
			target: "/edit/testpage",
			status: http.StatusOK,
		},
		{
			name:   "edit rejects put",
			method: http.MethodPut,
			target: "/edit/testpage",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:   "edit rejects put before lower casing",
			method: http.MethodPut,
			target: "/edit/TestPage",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:     "edit lower cases page",
			method:   http.MethodGet,
			target:   "/edit/TestPage",
			status:   http.StatusPermanentRedirect,
			location: "/edit/testpage",
		},
		{
			name:      "edit without slash is not found",
			method:    http.MethodGet,
			target:    "/edit",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:      "edit prefix is case sensitive",
			method:    http.MethodGet,
			target:    "/Edit/testpage",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:      "files view without slash is not found",
			method:    http.MethodGet,
			target:    "/files/view",
			status:    http.StatusNotFound,
			errorPage: true,
		},

		{
			name:     "exact route",
			method:   http.MethodGet,
			target:   "/wiki/index",
			status:   http.StatusOK,
			contains: "testpage",
		},
		{
			name:   "exact route rejects post",
			method: http.MethodPost,
			target: "/wiki/index",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:      "exact route is case sensitive",
			method:    http.MethodGet,
			target:    "/wiki/Index",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:     "exact route trailing slash is stripped",
			method:   http.MethodGet,
			target:   "/wiki/index/",
			status:   http.StatusPermanentRedirect,
			location: "/wiki/index",
		},
		{
			name:      "exact route does not match sub paths",
			method:    http.MethodGet,
			target:    "/wiki/index/more",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:   "post only route rejects get",
			method: http.MethodGet,
			target: "/wiki/login",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:     "encoded path matches decoded route",
			method:   http.MethodGet,
			target:   "/wiki/%69ndex",
			status:   http.StatusOK,
			contains: "testpage",
		},

		{
			name:     "static file served without prefix",
			method:   http.MethodGet,
			target:   "/static/Mixed.css",
			status:   http.StatusOK,
			contains: "fixture stylesheet",
		},
		{
			name:      "static missing file",
			method:    http.MethodGet,
			target:    "/static/missing.css",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:      "static without slash is not found",
			method:    http.MethodGet,
			target:    "/static",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:     "static root has its trailing slash stripped",
			method:   http.MethodGet,
			target:   "/static/",
			status:   http.StatusPermanentRedirect,
			location: "/static",
		},

		{
			name:      "unknown path is not lower cased",
			method:    http.MethodGet,
			target:    "/Unknown/Thing",
			status:    http.StatusNotFound,
			errorPage: true,
		},
		{
			name:     "unknown path with trailing slash",
			method:   http.MethodGet,
			target:   "/unknown/",
			status:   http.StatusPermanentRedirect,
			location: "/unknown",
		},

		{
			name:     "unclean path is cleaned",
			method:   http.MethodGet,
			target:   "/view/a/../testpage",
			status:   http.StatusMovedPermanently,
			location: "/view/testpage",
		},
		{
			name:     "encoded dot segments are cleaned",
			method:   http.MethodGet,
			target:   "/view/%2e%2e/testpage",
			status:   http.StatusMovedPermanently,
			location: "/testpage",
		},
		{
			name:     "double slashes are cleaned before stripping",
			method:   http.MethodGet,
			target:   "//example.com/",
			status:   http.StatusMovedPermanently,
			location: "/example.com/",
		},

		{
			name:    "cross origin protection runs before page errors",
			method:  http.MethodPost,
			target:  "/edit/testpage",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"},
			status:  http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				router, _ := newTestRouter(t)
				res := serve(t, router, tt.method, tt.target, tt.body, tt.headers)

				assert.Equal(t, tt.status, res.Code)
				assert.Equal(t, tt.location, res.Header().Get("Location"))
				if tt.contains != "" {
					assert.Contains(t, res.Body.String(), tt.contains)
				}
				if tt.errorPage {
					assert.Contains(t, res.Body.String(), "<html")
				} else if tt.status >= 400 {
					assert.NotContains(t, res.Body.String(), "<html")
				}
			},
		)
	}
}

func TestRouterLogsAfterCleaning(t *testing.T) {
	router, logs := newTestRouter(t)

	serve(t, router, http.MethodGet, "/view/a/../testpage", nil, nil)
	assert.Empty(t, logs.String())

	serve(t, router, http.MethodGet, "/view/testpage/", nil, nil)
	assert.Contains(t, logs.String(), `"GET /view/testpage/ HTTP/1.1" 308`)

	serve(t, router, http.MethodPut, "/edit/testpage", nil, nil)
	assert.Contains(t, logs.String(), `"PUT /edit/testpage HTTP/1.1" 405`)

	serve(t, router, http.MethodGet, "/nowhere", nil, nil)
	assert.Contains(t, logs.String(), `"GET /nowhere HTTP/1.1" 404`)
}
