package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Mock Vector Store
// ===========================================================================

type mockVectorStore struct {
	docs        map[string][]Document
	collections map[string]bool
}

func newMockVectorStore() *mockVectorStore {
	return &mockVectorStore{
		docs:        make(map[string][]Document),
		collections: make(map[string]bool),
	}
}

func (m *mockVectorStore) AddDocuments(ctx context.Context, collection string, docs []Document) error {
	m.docs[collection] = append(m.docs[collection], docs...)
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	var results []SearchResult
	for _, doc := range m.docs[req.Collection] {
		if req.Filter != nil {
			match := true
			for k, v := range req.Filter {
				if doc.Metadata[k] != fmt.Sprintf("%v", v) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		// Simple mock scoring based on content match (substring or word match)
		score := float32(0.5)
		if strings.Contains(strings.ToLower(doc.Content), strings.ToLower(req.Query)) || doc.Content == req.Query {
			score = 1.0
		} else {
			// Check if any word from query is in content
			queryWords := strings.Fields(strings.ToLower(req.Query))
			contentLower := strings.ToLower(doc.Content)
			matches := 0
			for _, word := range queryWords {
				if strings.Contains(contentLower, word) {
					matches++
				}
			}
			if matches > 0 {
				score = float32(0.6) + float32(matches)*0.1
			}
		}
		if score >= req.MinScore {
			results = append(results, SearchResult{
				Document: doc,
				Score:    score,
			})
		}
	}
	return results, nil
}

func (m *mockVectorStore) DeleteCollection(ctx context.Context, collection string) error {
	delete(m.collections, collection)
	delete(m.docs, collection)
	return nil
}

func (m *mockVectorStore) CreateCollection(ctx context.Context, collection string, dim int) error {
	m.collections[collection] = true
	return nil
}

func (m *mockVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	var names []string
	for name := range m.collections {
		names = append(names, name)
	}
	return names, nil
}

// ===========================================================================
// Retriever Tests
// ===========================================================================

func TestRetriever_Retrieve(t *testing.T) {
	store := newMockVectorStore()
	embedder := NewSimpleEmbeddingProvider(384)
	retriever := NewRetriever(store, embedder, WithDefaultK(5), WithMinScore(0.5))

	ctx := context.Background()
	docs := []Document{
		{ID: "1", Content: "how to restart nginx pod", Metadata: map[string]string{"category": "ops"}},
		{ID: "2", Content: "scale deployment to 3 replicas", Metadata: map[string]string{"category": "ops"}},
	}

	err := retriever.AddDocuments(ctx, "runbooks", docs)
	require.NoError(t, err)

	results, err := retriever.Retrieve(ctx, "restart nginx", "runbooks")
	require.NoError(t, err)
	assert.NotNil(t, results)
}

func TestRetriever_RetrieveWithFilter(t *testing.T) {
	store := newMockVectorStore()
	embedder := NewSimpleEmbeddingProvider(384)
	retriever := NewRetriever(store, embedder)

	ctx := context.Background()
	docs := []Document{
		{ID: "1", Content: "nginx restart", Metadata: map[string]string{"category": "networking"}},
		{ID: "2", Content: "redis failover", Metadata: map[string]string{"category": "database"}},
	}

	err := retriever.AddDocuments(ctx, "runbooks", docs)
	require.NoError(t, err)

	results, err := retriever.RetrieveWithFilter(ctx, "failover", "runbooks", map[string]interface{}{"category": "database"})
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// ===========================================================================
// Runbook RAG Tests
// ===========================================================================

func TestRunbookRAG_IndexAndSearch(t *testing.T) {
	store := newMockVectorStore()
	embedder := NewSimpleEmbeddingProvider(384)
	retriever := NewRetriever(store, embedder)
	runbookRAG := NewRunbookRAG(retriever)

	ctx := context.Background()
	runbook := RunbookDocument{
		ID:          "rb-1",
		Title:       "Nginx Pod Restart",
		Description: "How to restart nginx pods safely",
		Content:     "1. Check pod status\n2. Delete pod\n3. Wait for recreation",
		Tags:        []string{"nginx", "restart", "pod"},
		Category:    "networking",
		K8sCompat:   "v1.28+",
	}

	err := runbookRAG.IndexRunbook(ctx, runbook)
	require.NoError(t, err)

	results, err := runbookRAG.SearchRunbooks(ctx, "nginx restart", 5)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// ===========================================================================
// ChromaDB Client Tests
// ===========================================================================

func TestChromaDBClient_CreateCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/collections", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	err := client.CreateCollection(context.Background(), "test-collection", 384)
	require.NoError(t, err)
}

func TestChromaDBClient_ListCollections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/collections", r.URL.Path)
		collections := []chromaCollection{
			{ID: "1", Name: "runbooks"},
			{ID: "2", Name: "docs"},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(collections)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	names, err := client.ListCollections(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"runbooks", "docs"}, names)
}

func TestChromaDBClient_AddDocuments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/collections/test-collection/add", r.URL.Path)
		var req chromaAddRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Len(t, req.IDs, 2)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	docs := []Document{
		{ID: "1", Content: "doc1", Embedding: []float32{0.1, 0.2, 0.3}},
		{ID: "2", Content: "doc2", Embedding: []float32{0.4, 0.5, 0.6}},
	}
	err := client.AddDocuments(context.Background(), "test-collection", docs)
	require.NoError(t, err)
}

func TestChromaDBClient_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/collections/test-collection/query", r.URL.Path)

		var req chromaQueryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, []string{"nginx restart"}, req.QueryTexts)
		assert.Equal(t, 3, req.NResults)

		resp := chromaQueryResponse{
			IDs:       [][]string{{"1", "2"}},
			Documents: [][]string{{"how to restart nginx", "nginx troubleshooting"}},
			Metadatas: [][]map[string]interface{}{{{"category": "ops"}, {"category": "ops"}}},
			Distances: [][]float32{{0.1, 0.3}},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	results, err := client.Search(context.Background(), SearchRequest{
		Query:      "nginx restart",
		TopK:       3,
		MinScore:   0.6,
		Collection: "test-collection",
	})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "1", results[0].Document.ID)
	assert.InDelta(t, 0.9, results[0].Score, 0.01)
}

func TestChromaDBClient_DeleteCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/collections/test-collection", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	err := client.DeleteCollection(context.Background(), "test-collection")
	require.NoError(t, err)
}

func TestChromaDBClient_Search_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chromaQueryResponse{
			IDs:       [][]string{},
			Documents: [][]string{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	results, err := client.Search(context.Background(), SearchRequest{
		Query:      "nonexistent",
		TopK:       5,
		Collection: "test",
	})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ===========================================================================
// Simple Embedding Provider Tests
// ===========================================================================

func TestSimpleEmbeddingProvider_Embed(t *testing.T) {
	provider := NewSimpleEmbeddingProvider(10)
	embeddings, err := provider.Embed(context.Background(), []string{"hello", "world"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.Len(t, embeddings[0], 10)
	assert.Len(t, embeddings[1], 10)

	// Check normalization (length should be ~1)
	var sum float32
	for _, v := range embeddings[0] {
		sum += v * v
	}
	assert.InDelta(t, 1.0, sum, 0.01)
}

func TestSimpleEmbeddingProvider_Dimensions(t *testing.T) {
	provider := NewSimpleEmbeddingProvider(128)
	assert.Equal(t, 128, provider.Dimensions())
}

// ===========================================================================
// RAG Error Tests
// ===========================================================================

func TestRAGError(t *testing.T) {
	err := NewError(ErrCodeEmbeddingFailed, "embedding failed", nil)
	assert.Equal(t, ErrCodeEmbeddingFailed, err.Code)
	assert.Contains(t, err.Error(), "embedding failed")

	inner := fmt.Errorf("connection refused")
	err2 := NewError(ErrCodeStoreUnavailable, "store down", inner)
	assert.ErrorIs(t, err2, inner)
}

// ===========================================================================
// Document Tests
// ===========================================================================

func TestDocument(t *testing.T) {
	doc := Document{
		ID:      "doc-1",
		Content: "test content",
		Metadata: map[string]string{
			"key": "value",
		},
		Embedding: []float32{0.1, 0.2},
		Score:     0.95,
	}
	assert.Equal(t, "doc-1", doc.ID)
	assert.Equal(t, "test content", doc.Content)
	assert.Equal(t, "value", doc.Metadata["key"])
}

func TestSearchResult(t *testing.T) {
	sr := SearchResult{
		Document: Document{ID: "1", Content: "test"},
		Score:    0.85,
		Distance: 0.15,
	}
	assert.Equal(t, "1", sr.Document.ID)
	assert.InDelta(t, 0.85, sr.Score, 0.001)
}

// ===========================================================================
// Retriever Options Tests
// ===========================================================================

func TestRetrieverOptions(t *testing.T) {
	store := newMockVectorStore()
	embedder := NewSimpleEmbeddingProvider(384)
	retriever := NewRetriever(store, embedder,
		WithDefaultK(10),
		WithMinScore(0.8),
	)

	assert.Equal(t, 10, retriever.defaultK)
	assert.InDelta(t, 0.8, retriever.minScore, 0.001)
}

// ===========================================================================
// Runbook Document Tests
// ===========================================================================

func TestRunbookDocument(t *testing.T) {
	rb := RunbookDocument{
		ID:          "rb-1",
		Title:       "Test Runbook",
		Description: "A test runbook",
		Content:     "Step 1\nStep 2",
		Tags:        []string{"test", "ops"},
		Category:    "general",
		K8sCompat:   "v1.25+",
	}
	assert.Equal(t, "rb-1", rb.ID)
	assert.Equal(t, "Test Runbook", rb.Title)
	assert.Contains(t, rb.Tags, "test")
}

// ===========================================================================
// Search Request Tests
// ===========================================================================

func TestSearchRequest(t *testing.T) {
	req := SearchRequest{
		Query:      "nginx restart",
		TopK:       5,
		MinScore:   0.7,
		Collection: "runbooks",
		Filter:     map[string]interface{}{"category": "networking"},
	}
	assert.Equal(t, "nginx restart", req.Query)
	assert.Equal(t, 5, req.TopK)
}

// ===========================================================================
// ChromaDB Error Handling Tests
// ===========================================================================

func TestChromaDBClient_CreateCollection_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	err := client.CreateCollection(context.Background(), "test", 384)
	require.Error(t, err)
}

func TestChromaDBClient_AddDocuments_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	err := client.AddDocuments(context.Background(), "missing", []Document{{ID: "1"}})
	require.Error(t, err)
}

func TestChromaDBClient_Search_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	_, err := client.Search(context.Background(), SearchRequest{Collection: "test"})
	require.Error(t, err)
}

func TestChromaDBClient_ListCollections_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewChromaDBClient(WithBaseURL(server.URL))
	_, err := client.ListCollections(context.Background())
	require.Error(t, err)
}

// ===========================================================================
// VectorStore Interface Compliance
// ===========================================================================

func TestVectorStoreInterface(t *testing.T) {
	var _ VectorStore = (*ChromaDBClient)(nil)
	var _ VectorStore = (*mockVectorStore)(nil)
}

func TestEmbeddingProviderInterface(t *testing.T) {
	var _ EmbeddingProvider = (*SimpleEmbeddingProvider)(nil)
}
