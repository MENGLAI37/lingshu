package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ===========================================================================
// ChromaDB Client
// ===========================================================================

const chromaDefaultBaseURL = "http://localhost:8000"

// ChromaDBClient implements VectorStore using ChromaDB HTTP API.
type ChromaDBClient struct {
	baseURL    string
	httpClient *http.Client
	tenant     string
	database   string
}

// ChromaDBOption configures the ChromaDB client.
type ChromaDBOption func(*ChromaDBClient)

// WithBaseURL sets the ChromaDB base URL.
func WithBaseURL(url string) ChromaDBOption {
	return func(c *ChromaDBClient) {
		c.baseURL = url
	}
}

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *http.Client) ChromaDBOption {
	return func(c *ChromaDBClient) {
		c.httpClient = client
	}
}

// NewChromaDBClient creates a new ChromaDB client.
func NewChromaDBClient(opts ...ChromaDBOption) *ChromaDBClient {
	c := &ChromaDBClient{
		baseURL:  chromaDefaultBaseURL,
		tenant:   "default_tenant",
		database: "default_database",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CreateCollection creates a new collection in ChromaDB.
func (c *ChromaDBClient) CreateCollection(ctx context.Context, collection string, dim int) error {
	reqBody := map[string]interface{}{
		"name": collection,
		"metadata": map[string]interface{}{
			"hnsw:space": "cosine",
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/collections?tenant=%s&database=%s", c.baseURL, c.tenant, c.database)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return NewError(ErrCodeStoreUnavailable, "chromaDB create collection failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return NewError(ErrCodeStoreUnavailable, fmt.Sprintf("chromaDB create collection returned %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	return nil
}

// DeleteCollection deletes a collection.
func (c *ChromaDBClient) DeleteCollection(ctx context.Context, collection string) error {
	url := fmt.Sprintf("%s/api/v1/collections/%s?tenant=%s&database=%s", c.baseURL, collection, c.tenant, c.database)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return NewError(ErrCodeStoreUnavailable, "chromaDB delete collection failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return NewError(ErrCodeStoreUnavailable, fmt.Sprintf("chromaDB delete collection returned %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	return nil
}

// ListCollections lists all collections.
func (c *ChromaDBClient) ListCollections(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/collections?tenant=%s&database=%s", c.baseURL, c.tenant, c.database)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewError(ErrCodeStoreUnavailable, "chromaDB list collections failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeStoreUnavailable, fmt.Sprintf("chromaDB list collections returned %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	var collections []chromaCollection
	if err := json.NewDecoder(resp.Body).Decode(&collections); err != nil {
		return nil, err
	}

	names := make([]string, len(collections))
	for i, c := range collections {
		names[i] = c.Name
	}
	return names, nil
}

// AddDocuments adds documents to a collection.
func (c *ChromaDBClient) AddDocuments(ctx context.Context, collection string, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	ids := make([]string, len(docs))
	contents := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))
	metadatas := make([]map[string]interface{}, len(docs))

	for i, d := range docs {
		ids[i] = d.ID
		contents[i] = d.Content
		embeddings[i] = d.Embedding
		metadatas[i] = map[string]interface{}{}
		for k, v := range d.Metadata {
			metadatas[i][k] = v
		}
	}

	reqBody := chromaAddRequest{
		IDs:        ids,
		Documents:  contents,
		Embeddings: embeddings,
		Metadatas:  metadatas,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/collections/%s/add?tenant=%s&database=%s", c.baseURL, collection, c.tenant, c.database)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return NewError(ErrCodeStoreUnavailable, "chromaDB add documents failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return NewError(ErrCodeStoreUnavailable, fmt.Sprintf("chromaDB add documents returned %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	return nil
}

// Search performs a similarity search.
func (c *ChromaDBClient) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// First, generate embedding if not provided
	// In production, this should be handled by the retriever with an embedding model.
	// Here we expect the query to be embedded externally or use ChromaDB's built-in embedding.

	reqBody := chromaQueryRequest{
		QueryTexts: []string{req.Query},
		NResults:   req.TopK,
		Where:      req.Filter,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/collections/%s/query?tenant=%s&database=%s", c.baseURL, req.Collection, c.tenant, c.database)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeSearchFailed, "chromaDB search failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeSearchFailed, fmt.Sprintf("chromaDB search returned %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	var queryResp chromaQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, err
	}

	return c.convertResults(queryResp, req.MinScore)
}

// ===========================================================================
// ChromaDB Types
// ===========================================================================

type chromaCollection struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

type chromaAddRequest struct {
	IDs        []string                 `json:"ids"`
	Documents  []string                 `json:"documents,omitempty"`
	Embeddings [][]float32              `json:"embeddings,omitempty"`
	Metadatas  []map[string]interface{} `json:"metadatas,omitempty"`
}

type chromaQueryRequest struct {
	QueryTexts   []string                 `json:"query_texts,omitempty"`
	QueryEmbeddings [][]float32           `json:"query_embeddings,omitempty"`
	NResults     int                      `json:"n_results"`
	Where        map[string]interface{}   `json:"where,omitempty"`
	WhereDocument map[string]interface{}  `json:"where_document,omitempty"`
}

type chromaQueryResponse struct {
	IDs       [][]string                 `json:"ids"`
	Documents [][]string                 `json:"documents"`
	Metadatas [][]map[string]interface{} `json:"metadatas"`
	Distances [][]float32                `json:"distances"`
}

// ===========================================================================
// Helpers
// ===========================================================================

func (c *ChromaDBClient) convertResults(resp chromaQueryResponse, minScore float32) ([]SearchResult, error) {
	if len(resp.IDs) == 0 || len(resp.IDs[0]) == 0 {
		return []SearchResult{}, nil
	}

	results := make([]SearchResult, 0, len(resp.IDs[0]))
	for i := 0; i < len(resp.IDs[0]); i++ {
		score := float32(1.0)
		if len(resp.Distances) > 0 && len(resp.Distances[0]) > i {
			// Convert distance to similarity score (cosine distance -> similarity)
			score = 1.0 - resp.Distances[0][i]
		}

		if score < minScore {
			continue
		}

		var content string
		if len(resp.Documents) > 0 && len(resp.Documents[0]) > i {
			content = resp.Documents[0][i]
		}

		var metadata map[string]string
		if len(resp.Metadatas) > 0 && len(resp.Metadatas[0]) > i {
			metadata = make(map[string]string)
			for k, v := range resp.Metadatas[0][i] {
				if s, ok := v.(string); ok {
					metadata[k] = s
				}
			}
		}

		results = append(results, SearchResult{
			Document: Document{
				ID:       resp.IDs[0][i],
				Content:  content,
				Metadata: metadata,
			},
			Score:    score,
			Distance: 1.0 - score,
		})
	}

	return results, nil
}
