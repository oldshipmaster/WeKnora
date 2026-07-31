package mathmastery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBootstrapClientCreatesKnowledgeBaseUploadsAndSeeds(t *testing.T) {
	book := filepath.Join(t.TempDir(), "一年级上册.pdf")
	require.NoError(t, os.WriteFile(book, []byte("pdf"), 0o600))
	registered := false
	seeded := false
	sourcesWritten := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			if !registered {
				http.Error(w, `{"message":"unknown user"}`, http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"token":"test-token"}`))
		case "/api/v1/auth/register":
			registered = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/api/v1/knowledge-bases":
			require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"kb-1","name":"人教版小学数学体系化掌握"}}`))
		case "/api/v1/knowledge-bases/kb-1/knowledge/file":
			require.NoError(t, r.ParseMultipartForm(1024))
			require.Contains(t, r.FormValue("process_config"), `"graph_enabled":true`)
			_, header, err := r.FormFile("file")
			require.NoError(t, err)
			require.Equal(t, "一年级上册.pdf", header.Filename)
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"knowledge-1","parse_status":"pending"}}`))
		case "/api/v1/knowledge-bases/kb-1/math-mastery/seed":
			var seed CurriculumSeed
			require.NoError(t, json.NewDecoder(r.Body).Decode(&seed))
			require.Len(t, seed.Nodes, 1)
			seeded = true
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/api/v1/knowledge-bases/kb-1/math-mastery/sources":
			var body struct {
				Sources []map[string]any `json:"sources"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "processing", body.Sources[0]["status"])
			sourcesWritten = true
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	curriculum := []byte(`{"nodes":[{"id":"count","node_type":"concept","grade":1,"term":1,"domain":"number","title":"数一数"}],"edges":[]}`)
	manifest := Manifest{Entries: []ManifestEntry{{
		TargetID: "rj-g1-s1-textbook", MaterialID: "material-1", Kind: MaterialTextbook,
		Status: StatusFound, Title: "一年级上册", Edition: "人教版", Grade: 1, Term: 1, Path: book,
	}}}

	report, err := NewBootstrapClient(server.URL, server.Client()).Run(context.Background(), BootstrapConfig{
		Email: "math@example.local", Password: "secret-pass", KnowledgeBaseName: "人教版小学数学体系化掌握",
		Curriculum: curriculum, Manifest: manifest,
	})
	require.NoError(t, err)
	require.True(t, registered)
	require.True(t, seeded)
	require.True(t, sourcesWritten)
	require.Equal(t, "kb-1", report.KnowledgeBaseID)
	require.Equal(t, 1, report.UploadedTextbooks)
	require.False(t, strings.Contains(report.KnowledgeBaseID, "test-token"))
}

func TestUploadTextbookReusesDuplicateAndMapsCompletedToReady(t *testing.T) {
	book := filepath.Join(t.TempDir(), "一年级上册.pdf")
	require.NoError(t, os.WriteFile(book, []byte("pdf"), 0o600))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"success":false,"data":{"id":"knowledge-existing","parse_status":"completed"}}`))
	}))
	defer server.Close()

	client := NewBootstrapClient(server.URL, server.Client())
	client.token = "token"
	id, status, duplicate, err := client.uploadTextbook(context.Background(), "kb-1", ManifestEntry{
		TargetID: "book-1", Kind: MaterialTextbook, Status: StatusFound, Title: "一年级上册", Path: book,
	})
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, "knowledge-existing", id)
	require.Equal(t, StatusReady, status)
}
