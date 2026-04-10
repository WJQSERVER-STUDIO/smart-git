package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infinite-iroha/touka"
)

func TestServiceRPCErrorShouldNotReturnJSONToGitClients(t *testing.T) {
	t.Helper()

	r := touka.Default()
	r.POST("/:user/git-upload-pack", serviceRPC(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/octocat/git-upload-pack", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		t.Fatalf("expected non-JSON git error response, got Content-Type %q", contentType)
	}

	if !strings.Contains(recorder.Body.String(), "400 Bad Request") {
		t.Fatalf("expected plain HTTP error body, got %q", recorder.Body.String())
	}
}
