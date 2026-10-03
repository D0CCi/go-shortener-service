package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/D0CCi/go-shortener-service/internal/shortener"
	"github.com/D0CCi/go-shortener-service/internal/storage"
)

const testBaseURL = "http://short.test"

// fakeService returns preset values and remembers what it was called with.
type fakeService struct {
	code    string
	link    string
	created bool
	err     error

	gotURL  string
	gotCode string
}

func (f *fakeService) Shorten(_ context.Context, rawURL string) (string, string, bool, error) {
	f.gotURL = rawURL
	return f.code, f.link, f.created, f.err
}

func (f *fakeService) Resolve(_ context.Context, code string) (string, error) {
	f.gotCode = code
	return f.link, f.err
}

// do sends one request to the API backed by svc and returns the recorded response.
func do(svc Shortener, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	New(svc, testBaseURL, slog.New(slog.DiscardHandler)).ServeHTTP(rec, req)
	return rec
}

func TestAPI(t *testing.T) {
	const okBody = `{"url":"http://a.ru","short_url":"http://short.test/aaaaaaaaaa"}`
	errInternal := errors.New("pq: password authentication failed") // must not reach the client

	tests := []struct {
		name       string
		svc        fakeService
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string // empty means not checked (mux plain-text replies)
	}{
		// POST /
		{name: "shorten new", svc: fakeService{code: "aaaaaaaaaa", link: "http://a.ru", created: true},
			method: "POST", path: "/", body: `{"url":"a.ru"}`, wantStatus: http.StatusCreated, wantBody: okBody},
		{name: "shorten existing", svc: fakeService{code: "aaaaaaaaaa", link: "http://a.ru"},
			method: "POST", path: "/", body: `{"url":"a.ru"}`, wantStatus: http.StatusOK, wantBody: okBody},
		{name: "broken json", method: "POST", path: "/", body: `{"url":`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":"invalid json"}`},
		{name: "body too large", method: "POST", path: "/", body: `{"url":"` + strings.Repeat("a", maxBodyBytes) + `"}`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":"invalid json"}`},
		{name: "invalid url", svc: fakeService{err: fmt.Errorf("%w: missing host", shortener.ErrInvalidURL)},
			method: "POST", path: "/", body: `{"url":"https://"}`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":"invalid url: missing host"}`},
		{name: "shorten internal error hidden", svc: fakeService{err: errInternal},
			method: "POST", path: "/", body: `{"url":"a.ru"}`,
			wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal error"}`},

		// GET /{code}
		{name: "resolve", svc: fakeService{link: "http://a.ru"},
			method: "GET", path: "/aaaaaaaaaa", wantStatus: http.StatusOK, wantBody: okBody},
		{name: "invalid code", svc: fakeService{err: shortener.ErrInvalidCode},
			method: "GET", path: "/abc", wantStatus: http.StatusBadRequest, wantBody: `{"error":"invalid code"}`},
		{name: "not found", svc: fakeService{err: storage.ErrNotFound},
			method: "GET", path: "/aaaaaaaaaa", wantStatus: http.StatusNotFound, wantBody: `{"error":"not found"}`},
		{name: "resolve internal error hidden", svc: fakeService{err: errInternal},
			method: "GET", path: "/aaaaaaaaaa", wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal error"}`},

		// routing
		{name: "post with path", method: "POST", path: "/aaaaaaaaaa", wantStatus: http.StatusMethodNotAllowed},
		{name: "delete", method: "DELETE", path: "/aaaaaaaaaa", wantStatus: http.StatusMethodNotAllowed},
		{name: "get root", method: "GET", path: "/", wantStatus: http.StatusMethodNotAllowed},
		{name: "nested path", method: "GET", path: "/a/b", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(&tt.svc, tt.method, tt.path, tt.body)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody == "" {
				return
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("body = %s, want %s", got, tt.wantBody)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}

// The handlers must pass the client's input to the service untouched.
func TestAPI_PassesInput(t *testing.T) {
	svc := &fakeService{code: "aaaaaaaaaa", link: "http://a.ru"}

	do(svc, "POST", "/", `{"url":" пример.рф/путь "}`)
	if svc.gotURL != " пример.рф/путь " {
		t.Errorf("Shorten got %q", svc.gotURL)
	}

	do(svc, "GET", "/bbbbbbbbbb", "")
	if svc.gotCode != "bbbbbbbbbb" {
		t.Errorf("Resolve got %q", svc.gotCode)
	}
}
