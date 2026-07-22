package rag

import (
	"context"

	"github.com/lingshu/lingshu/pkg/agent"
)

// ===========================================================================
// RAG Agent Adapter — bridges the RAG package to the Agent Loop
// ===========================================================================

// AgentRetrieverAdapter adapts RunbookRAG to the agent.RAGRetriever interface.
type AgentRetrieverAdapter struct {
	runbookRAG *RunbookRAG
}

// NewAgentRetrieverAdapter creates a new adapter for the agent loop.
func NewAgentRetrieverAdapter(runbook *RunbookRAG) *AgentRetrieverAdapter {
	return &AgentRetrieverAdapter{runbookRAG: runbook}
}

// Search implements agent.RAGRetriever by delegating to the runbook RAG.
func (a *AgentRetrieverAdapter) Search(ctx context.Context, query string, collection string, topK int) ([]agent.RAGDocument, error) {
	results, err := a.runbookRAG.SearchRunbooks(ctx, query, topK)
	if err != nil {
		return nil, err
	}

	docs := make([]agent.RAGDocument, 0, len(results))
	for _, r := range results {
		docs = append(docs, agent.RAGDocument{
			Content:  r.Document.Content,
			Score:    r.Score,
			Metadata: r.Document.Metadata,
		})
	}
	return docs, nil
}

// ChromaDBRetrieverAdapter adapts the low-level Retriever to agent.RAGRetriever.
type ChromaDBRetrieverAdapter struct {
	retriever *Retriever
}

// NewChromaDBRetrieverAdapter creates a new adapter from a Retriever.
func NewChromaDBRetrieverAdapter(ret *Retriever) *ChromaDBRetrieverAdapter {
	return &ChromaDBRetrieverAdapter{retriever: ret}
}

// Search implements agent.RAGRetriever.
func (a *ChromaDBRetrieverAdapter) Search(ctx context.Context, query string, collection string, topK int) ([]agent.RAGDocument, error) {
	results, err := a.retriever.Retrieve(ctx, query, collection)
	if err != nil {
		return nil, err
	}

	docs := make([]agent.RAGDocument, 0, len(results))
	for _, r := range results {
		if len(docs) >= topK {
			break
		}
		docs = append(docs, agent.RAGDocument{
			Content:  r.Document.Content,
			Score:    r.Score,
			Metadata: r.Document.Metadata,
		})
	}
	return docs, nil
}
