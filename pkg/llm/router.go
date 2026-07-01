package llm

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ===========================================================================
// Router - LLM 路由与故障切换
// ===========================================================================

// Router manages multiple LLM providers with failover logic.
type Router struct {
	providers       []Provider
	configs         []ProviderConfig
	metrics         MetricsCollector
	fallbackTimeout time.Duration
	mu              sync.RWMutex
}

// RouterOption configures the router.
type RouterOption func(*Router)

// WithMetricsCollector sets the metrics collector.
func WithMetricsCollector(m MetricsCollector) RouterOption {
	return func(r *Router) {
		r.metrics = m
	}
}

// WithFallbackTimeout sets the timeout for failover.
func WithFallbackTimeout(d time.Duration) RouterOption {
	return func(r *Router) {
		r.fallbackTimeout = d
	}
}

// NewRouter creates a new LLM router.
func NewRouter(configs []ProviderConfig, opts ...RouterOption) *Router {
	r := &Router{
		configs:         configs,
		providers:       make([]Provider, 0, len(configs)),
		fallbackTimeout: 3 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}

	// Initialize providers
	for _, cfg := range configs {
		var provider Provider
		switch cfg.Name {
		case "openai":
			provider = NewOpenAIProvider(cfg)
		case "claude":
			provider = NewClaudeProvider(cfg)
		case "ollama":
			provider = NewOllamaProvider(cfg)
		default:
			// Try to infer from base URL or other hints
			if cfg.IsLocal {
				provider = NewOllamaProvider(cfg)
			} else {
				provider = NewOpenAIProvider(cfg)
			}
		}
		r.providers = append(r.providers, provider)
	}

	// Sort by priority (lower number = higher priority)
	sortProviders(r.providers, configs)

	return r
}

// Complete routes a completion request to the best available provider.
func (r *Router) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	providers := r.getProviders()
	if len(providers) == 0 {
		return nil, NewError(ErrCodeNoProviderAvailable, "no LLM providers configured", nil)
	}

	var lastErr error
	for _, provider := range providers {
		resp, err := r.tryComplete(ctx, provider, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Check context cancellation
		if ctx.Err() != nil {
			return nil, NewError(ErrCodeCancelled, "request cancelled", ctx.Err())
		}
	}

	return nil, NewError(ErrCodeNoProviderAvailable, "all providers failed", lastErr)
}

// Stream routes a streaming request to the best available provider.
func (r *Router) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	providers := r.getProviders()
	if len(providers) == 0 {
		return nil, NewError(ErrCodeNoProviderAvailable, "no LLM providers configured", nil)
	}

	for _, provider := range providers {
		ch, err := r.tryStream(ctx, provider, req)
		if err == nil {
			return ch, nil
		}

		if ctx.Err() != nil {
			return nil, NewError(ErrCodeCancelled, "request cancelled", ctx.Err())
		}
	}

	return nil, NewError(ErrCodeNoProviderAvailable, "all providers failed for streaming", nil)
}

// HealthCheck checks all providers and returns their status.
func (r *Router) HealthCheck(ctx context.Context) map[string]error {
	providers := r.getProviders()
	results := make(map[string]error, len(providers))

	for _, p := range providers {
		results[p.Name()] = p.HealthCheck(ctx)
	}

	return results
}

// GetProviders returns the list of configured providers.
func (r *Router) GetProviders() []Provider {
	return r.getProviders()
}

// GetProviderNames returns the names of all configured providers.
func (r *Router) GetProviderNames() []string {
	providers := r.getProviders()
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Name())
	}
	return names
}

// GetProviderConfig returns the config for a specific provider.
func (r *Router) GetProviderConfig(name string) *ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cfg := range r.configs {
		if cfg.Name == name {
			return &cfg
		}
	}
	return nil
}

// UpdateProviders updates the provider list dynamically.
func (r *Router) UpdateProviders(configs []ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configs = configs
	r.providers = make([]Provider, 0, len(configs))
	for _, cfg := range configs {
		var provider Provider
		switch cfg.Name {
		case "openai":
			provider = NewOpenAIProvider(cfg)
		case "claude":
			provider = NewClaudeProvider(cfg)
		case "ollama":
			provider = NewOllamaProvider(cfg)
		default:
			if cfg.IsLocal {
				provider = NewOllamaProvider(cfg)
			} else {
				provider = NewOpenAIProvider(cfg)
			}
		}
		r.providers = append(r.providers, provider)
	}
	sortProviders(r.providers, configs)
}

// ===========================================================================
// Internal helpers
// ===========================================================================

func (r *Router) tryComplete(ctx context.Context, provider Provider, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	// Apply per-provider timeout if configured
	providerCtx := ctx
	for _, cfg := range r.configs {
		if cfg.Name == provider.Name() && cfg.Timeout > 0 {
			var cancel context.CancelFunc
			providerCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()
			break
		}
	}

	resp, err := provider.Complete(providerCtx, req)
	latency := time.Since(start)

	if r.metrics != nil {
		var usage TokenUsage
		if resp != nil {
			usage = resp.Usage
		}
		r.metrics.RecordUsage(ctx, provider.Type(), provider.Name(), usage, latency, err)
	}

	return resp, err
}

func (r *Router) tryStream(ctx context.Context, provider Provider, req *CompletionRequest) (<-chan StreamChunk, error) {
	providerCtx := ctx
	for _, cfg := range r.configs {
		if cfg.Name == provider.Name() && cfg.Timeout > 0 {
			var cancel context.CancelFunc
			providerCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()
			break
		}
	}

	return provider.Stream(providerCtx, req)
}

func (r *Router) getProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, len(r.providers))
	copy(out, r.providers)
	return out
}

func sortProviders(providers []Provider, configs []ProviderConfig) {
	// Create a map of provider name to priority
	priorityMap := make(map[string]int)
	for _, cfg := range configs {
		priorityMap[cfg.Name] = cfg.Priority
	}

	sort.SliceStable(providers, func(i, j int) bool {
		pi := priorityMap[providers[i].Name()]
		pj := priorityMap[providers[j].Name()]
		return pi < pj
	})
}

// ===========================================================================
// Factory
// ===========================================================================

// CreateRouterFromConfig creates a router from a list of provider configs.
func CreateRouterFromConfig(configs []ProviderConfig, opts ...RouterOption) (*Router, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("no provider configs provided")
	}
	return NewRouter(configs, opts...), nil
}
