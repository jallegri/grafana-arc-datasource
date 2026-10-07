package plugin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/sync/semaphore"
)

func queryJSONTestSettings(server *httptest.Server) *ArcInstanceSettings {
	return &ArcInstanceSettings{
		settings:         ArcDataSourceSettings{URL: server.URL},
		client:           server.Client(),
		sem:              semaphore.NewWeighted(1),
		maxResponseBytes: 1024,
	}
}

func TestQueryJSONRejectsTruncatedResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("request path = %q, want /api/v1/query", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"columns":["value"],"data":[[42]],"truncated":true,"truncation_reason":"row limit"}`)
	}))
	defer server.Close()

	frame, err := queryJSON(context.Background(), queryJSONTestSettings(server), "SELECT 42 AS value")
	if err == nil {
		t.Fatal("queryJSON returned a partial frame without an error for a truncated result")
	}
	if frame != nil {
		t.Fatalf("queryJSON returned a partial frame for a truncated result: %#v", frame)
	}
	if !strings.Contains(err.Error(), "truncated") || !strings.Contains(err.Error(), "row limit") {
		t.Fatalf("queryJSON error = %q, want truncation reason", err)
	}
}

func TestQueryJSONAcceptsCompleteLegacyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"columns":["value"],"data":[[42]]}`)
	}))
	defer server.Close()

	frame, err := queryJSON(context.Background(), queryJSONTestSettings(server), "SELECT 42 AS value")
	if err != nil {
		t.Fatalf("queryJSON returned an error for a complete legacy response: %v", err)
	}
	if frame == nil || len(frame.Fields) != 1 {
		t.Fatalf("queryJSON frame = %#v, want one value field containing 42", frame)
	}
	value, ok := frame.Fields[0].At(0).(*float64)
	if !ok || value == nil || *value != 42 {
		t.Fatalf("queryJSON field value = %#v, want a numeric value of 42", frame.Fields[0].At(0))
	}
}
