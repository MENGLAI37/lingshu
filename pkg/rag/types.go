package rag

import (
	"context"
	"fmt"
	"time"
)

// ===========================================================================
// Core Types
// ===========================================================================

// Document represents a document in the RAG store.
type Document struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	Embedding []float32         `json:"embedding,omitempty"`
	Score     float32           `json:"score,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	Document Document
	Score    float32
	Distance float32
}

// SearchRequest represents a semantic search request.
type SearchRequest struct {
	Query       string
	TopK        int
	Filter      map[string]interface{}
	MinScore    float32
	Collection  string
}

// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// VectorStore is the interface for vector database operations.
type VectorStore interface {
	AddDocuments(ctx context.Context, collection string, docs []Document) error
	Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
	DeleteCollection(ctx context.Context, collection string) error
	CreateCollection(ctx context.Context, collection string, dim int) error
	ListCollections(ctx context.Context) ([]string, error)
}

// Retriever performs semantic retrieval with optional re-ranking.
type Retriever struct {
	store     VectorStore
	embedder  EmbeddingProvider
	defaultK  int
	minScore  float32
}

// RetrieverOption configures the retriever.
type RetrieverOption func(*Retriever)

// WithDefaultK sets the default top-k.
func WithDefaultK(k int) RetrieverOption {
	return func(r *Retriever) {
		r.defaultK = k
	}
}

// WithMinScore sets the minimum similarity score.
func WithMinScore(score float32) RetrieverOption {
	return func(r *Retriever) {
		r.minScore = score
	}
}

// NewRetriever creates a new retriever.
func NewRetriever(store VectorStore, embedder EmbeddingProvider, opts ...RetrieverOption) *Retriever {
	ret := &Retriever{
		store:    store,
		embedder: embedder,
		defaultK: 5,
		minScore: 0.7,
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// Retrieve performs semantic search and returns relevant documents.
func (r *Retriever) Retrieve(ctx context.Context, query string, collection string) ([]SearchResult, error) {
	if collection == "" {
		collection = "default"
	}

	req := SearchRequest{
		Query:      query,
		TopK:       r.defaultK,
		MinScore:   r.minScore,
		Collection: collection,
	}

	return r.store.Search(ctx, req)
}

// RetrieveWithFilter performs semantic search with metadata filtering.
func (r *Retriever) RetrieveWithFilter(ctx context.Context, query string, collection string, filter map[string]interface{}) ([]SearchResult, error) {
	if collection == "" {
		collection = "default"
	}

	req := SearchRequest{
		Query:      query,
		TopK:       r.defaultK,
		MinScore:   r.minScore,
		Collection: collection,
		Filter:     filter,
	}

	return r.store.Search(ctx, req)
}

// AddDocuments adds documents to the vector store.
func (r *Retriever) AddDocuments(ctx context.Context, collection string, docs []Document) error {
	if collection == "" {
		collection = "default"
	}

	// Generate embeddings if not provided
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}

	embeddings, err := r.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	for i := range docs {
		docs[i].Embedding = embeddings[i]
	}

	return r.store.AddDocuments(ctx, collection, docs)
}

// ===========================================================================
// Runbook RAG (specialized for ops runbooks)
// ===========================================================================

// RunbookDocument represents a runbook in the RAG store.
type RunbookDocument struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	Category    string   `json:"category"`
	K8sCompat   string   `json:"k8s_compat,omitempty"`
}

// RunbookRAG provides runbook-specific retrieval.
type RunbookRAG struct {
	retriever  *Retriever
	collection string
}

// NewRunbookRAG creates a new runbook RAG retriever.
func NewRunbookRAG(retriever *Retriever) *RunbookRAG {
	return &RunbookRAG{
		retriever:  retriever,
		collection: "runbooks",
	}
}

// IndexRunbook indexes a runbook into the vector store.
func (r *RunbookRAG) IndexRunbook(ctx context.Context, runbook RunbookDocument) error {
	content := runbook.Title + "\n" + runbook.Description + "\n" + runbook.Content

	doc := Document{
		ID:      runbook.ID,
		Content: content,
		Metadata: map[string]string{
			"title":       runbook.Title,
			"category":    runbook.Category,
			"k8s_compat":  runbook.K8sCompat,
			"description": runbook.Description,
		},
	}

	// Add tags to metadata
	for i, tag := range runbook.Tags {
		doc.Metadata[fmt.Sprintf("tag_%d", i)] = tag
	}

	return r.retriever.AddDocuments(ctx, r.collection, []Document{doc})
}

// SearchRunbooks searches for relevant runbooks.
func (r *RunbookRAG) SearchRunbooks(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	req := SearchRequest{
		Query:      query,
		TopK:       topK,
		Collection: r.collection,
		MinScore:   0.6,
	}

	return r.retriever.store.Search(ctx, req)
}

// ===========================================================================
// Errors
// ===========================================================================

// RAGError represents an error from the RAG layer.
type RAGError struct {
	Code    string
	Message string
	Cause   error
}

func (e *RAGError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("rag error [%s]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("rag error [%s]: %s", e.Code, e.Message)
}

func (e *RAGError) Unwrap() error {
	return e.Cause
}

// Common error codes.
const (
	ErrCodeEmbeddingFailed  = "EMBEDDING_FAILED"
	ErrCodeStoreUnavailable = "STORE_UNAVAILABLE"
	ErrCodeSearchFailed     = "SEARCH_FAILED"
	ErrCodeCollectionNotFound = "COLLECTION_NOT_FOUND"
)

// NewError creates a new RAGError.
func NewError(code, message string, cause error) *RAGError {
	return &RAGError{Code: code, Message: message, Cause: cause}
}

// ===========================================================================
// Simple in-memory embedding provider (for testing / local dev)
// ===========================================================================

// SimpleEmbeddingProvider is a simple embedding provider using a deterministic hash.
// This is NOT for production - it's a lightweight fallback for testing.
type SimpleEmbeddingProvider struct {
	dims int
}

// NewSimpleEmbeddingProvider creates a simple embedding provider.
func NewSimpleEmbeddingProvider(dims int) *SimpleEmbeddingProvider {
	if dims <= 0 {
		dims = 384
	}
	return &SimpleEmbeddingProvider{dims: dims}
}

// Embed generates simple deterministic embeddings.
func (s *SimpleEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		emb := make([]float32, s.dims)
		// Simple hash-based embedding for testing
		var hash uint32 = 0
		for _, c := range text {
			hash = hash*31 + uint32(c)
		}
		for j := range emb {
			val := float32(int(hash)%1000) / 1000.0
			emb[j] = val
			hash = hash*31 + uint32(j)
		}
		// Normalize
		var norm float32
		for _, v := range emb {
			norm += v * v
		}
		norm = float32(sqrt(float64(norm)))
		if norm > 0 {
			for j := range emb {
				emb[j] /= norm
			}
		}
		result[i] = emb
	}
	return result, nil
}

// Dimensions returns the embedding dimension.
func (s *SimpleEmbeddingProvider) Dimensions() int {
	return s.dims
}

func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// ===========================================================================
// UsageRecord for RAG operations
// ===========================================================================

// RAGUsageRecord tracks RAG retrieval metrics.
type RAGUsageRecord struct {
	Query        string        `json:"query"`
	Collection   string        `json:"collection"`
	ResultsCount int           `json:"results_count"`
	LatencyMs    int64         `json:"latency_ms"`
	Timestamp    time.Time     `json:"timestamp"`
}
