# 灵枢 (LingShu) — AI-Native SRE Agent

> **Talk to your cluster. Let the agent do the work.**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org/)
[![CI](https://github.com/MENGLAI37/lingshu/actions/workflows/ci.yaml/badge.svg)](https://github.com/MENGLAI37/lingshu/actions/workflows/ci.yaml)
[![Coverage](https://img.shields.io/badge/coverage-33%25-yellow)]()

LingShu is a terminal-native, AI-powered operations agent for SRE teams. It connects to your Kubernetes cluster, understands natural language intent, and autonomously diagnoses and remediates issues — with a five-level safety gateway that ensures nothing dangerous happens without your approval.

---

## Why LingShu?

| Problem | LingShu's Answer |
|---------|-----------------|
| Switching between 5+ tools to diagnose one incident | One terminal, one natural language query |
| 3am on-call mistakes on production | Five-level risk gating (L0–L4) with environment-aware scoring |
| No audit trail for compliance | Immutable evidence chain with hash-linked audit records |
| LLM hallucinates kubectl commands | Agent loop enforces tool-use; never outputs raw commands |
| GitOps controllers revert manual fixes | Built-in ArgoCD/Flux detection with conflict warnings |
| Agent runs away with write operations | Circuit breaker + dead-loop detection + per-session L2 caps |

---

## Quick Start

### Prerequisites

- **Go 1.22+**
- **kubectl** with a valid kubeconfig (or in-cluster service account)
- **LLM API key** — OpenAI, DeepSeek, Claude, or local Ollama

### Install

```bash
git clone https://github.com/MENGLAI37/lingshu.git
cd lingshu
make build
```

### Run

```bash
# Interactive TUI mode
./bin/lingshu

# Headless mode — ask a question, get an answer
./bin/lingshu --no-tui "Why is nginx crashing in production?"

# CI/CD pipeline mode — auto-confirm L0–L2, machine-readable output
./bin/lingshu --no-tui --yes --pipe "Scale payment-api to 5 replicas"

# Dry-run — full reasoning chain, zero side effects
./bin/lingshu --no-tui --dry-run "Restart all failing pods"

# Autonomous ops demo
./bin/lingshu --auto-demo
```

### Configure LLM

```bash
export OPENAI_API_KEY="sk-..."     # OpenAI
export DEEPSEEK_API_KEY="sk-..."   # DeepSeek
export ANTHROPIC_API_KEY="sk-..."  # Claude
# Or use local Ollama — auto-detected at http://localhost:11434
```

---

## Features

### 🤖 Autonomous Agent Loop

The core reasoning engine follows a **Think → Act → Observe → Reflect** cycle:

1. **Think** — LLM analyzes the current state and decides which tools to call
2. **Act** — Execute K8s tools; security gateway evaluates every operation
3. **Observe** — Results are fed back into context for the next reasoning step
4. **Reflect** — Loop continues until resolution or timeout (5 min default)

Built-in safeguards:
- **Circuit breaker** — caps L2+ operations per session; auto-pauses on consecutive writes
- **Dead-loop detection** — identifies repeating tool patterns and error cycles
- **Global timeout** — context deadline prevents runaway execution
- **Panic recovery** — returns partial results instead of crashing

### 🛡️ Five-Level Security Gateway (L0–L4)

| Level | Color | Description | Behavior |
|-------|-------|-------------|----------|
| **L0** | Green | Read-only queries (`get`, `describe`, `logs`, `events`) | Auto-execute |
| **L1** | Blue | Safe writes (`top`, `status`) | Auto-execute |
| **L2** | Yellow | Moderate risk (`scale`, `restart`, `rollout`, `patch`) | Confirm |
| **L3** | Red | High risk (destructive operations) | Double-confirm |
| **L4** | Purple | Critical risk (cluster-level RBAC, namespace deletion) | Blocked |

The gateway combines **three risk evaluators** (tool type, environment, resource sensitivity) and **three blocking rules** (production safety, namespace protection, cluster-level RBAC):

- `production` environment adds +20 risk points; `kube-system` adds +30
- ClusterRole/RoleBinding modifications are always blocked
- On-call + change-window checks for L3+ operations
- **GitOps conflict detection** — warns before modifying ArgoCD/Flux-managed resources

### 📋 K8s Tool Arsenal

| Tool | Level | What it does |
|------|-------|-------------|
| `k8s_get` | L0 | Get pods, deployments, services, events, ingresses, configmaps |
| `k8s_describe` | L0 | Detailed resource description with status analysis |
| `k8s_logs` | L0 | Pod log retrieval with follow/stream support |
| `k8s_events` | L0 | Namespace events with warning classification |
| `k8s_top` | L1 | Pod and node resource usage (CPU/memory) |
| `k8s_status` | L1 | Multi-dimensional cluster health check (nodes, pods, deployments) |
| `k8s_scale` | L2 | Scale deployments, statefulsets, replica sets |
| `k8s_restart` | L2 | Rolling restart via annotation trigger |
| `k8s_rollout` | L2 | Rollout status, history, undo, pause, resume |
| `k8s_patch` | L2 | Strategic merge, JSON patch, apply patches |

All tools are registered in the agent's tool registry. The LLM sees their schemas and decides which to call — it **never outputs raw kubectl commands**.

### 🔍 Autonomous Operations Engine

The `--auto-demo` flag demonstrates a complete autonomous remediation pipeline:

```
Alert fires → Engine receives webhook → Auto-diagnoses with K8s tools
→ LLM analyzes root cause → Risk evaluation → User confirmation (L2+)
→ Execute fix → Audit trail recorded
```

The `alertd` binary runs as a standalone webhook server accepting alerts from Prometheus AlertManager, PagerDuty, and generic sources.

### 📊 Audit & Compliance

Every L1+ operation is recorded with:
- **Evidence chain** — hash-linked records (`prev_hash → content_hash`) for tamper-proof audit trails
- **Batch write** — async with configurable batch size, non-blocking for agent performance
- **File fallback** — automatically writes to JSONL when database is unavailable
- **Rich metadata** — pre-check results, impact analysis, rollback info, approval records

### 🖥️ Terminal UI (Bubble Tea)

- **Multi-line input** with history navigation (↑/↓)
- **Streaming output** — LLM responses render in real-time
- **Command preview** — risk-level highlighted before execution
- **Status bar** — cluster, namespace, token usage, cost
- **Syntax highlighting** — YAML, JSON, tables
- **Theme switching** — dark, light, high-contrast
- **Configuration panel** — change LLM provider/model on the fly
- **Confirmation modal** — clear risk assessment before L2+ execution

### 🔗 GitOps Conflict Detection

Before any L2+ operation, LingShu checks whether the target resource is managed by ArgoCD or Flux (via annotations and labels). If detected, the confirmation prompt includes a **conflict warning** explaining that the change will be reverted and suggesting the correct GitOps workflow.

### 🧩 Additional Capabilities

- **Multi-provider LLM router** — OpenAI, Claude, Ollama with automatic failover
- **RBAC self-check** — startup verification of service account permissions for each tool
- **Session management** — full CRUD with parent-child chains, TTL, token budgets
- **Workflow engine** — DAG-based workflows with conditions, variables, and tool actions
- **Scheduler** — cron/interval/once jobs for periodic health checks
- **Pre-mutation snapshots** — save resource YAML before L2+ changes for rollback
- **Configuration hot-reload** — Viper + fsnotify for zero-downtime config changes

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     lingshu (TUI/CLI)                    │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐             │
│  │ Bubble  │  │  Agent   │  │  Security  │             │
│  │ Tea TUI │  │  Loop    │  │  Gateway   │             │
│  └─────────┘  └────┬─────┘  └─────┬──────┘             │
│                    │              │                      │
│         ┌──────────┼──────────────┼──────────┐          │
│         │          │              │          │          │
│    ┌────▼──┐ ┌────▼──┐ ┌───────▼──┐ ┌─────▼───┐       │
│    │  LLM  │ │  K8s  │ │ Circuit  │ │  Audit  │       │
│    │ Router│ │ Tools │ │ Breaker  │ │ Manager │       │
│    └───────┘ └───────┘ └──────────┘ └─────────┘       │
└─────────────────────────────────────────────────────────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
     ┌────▼──┐    ┌─────▼─────┐   ┌────▼────┐
     │  LLM  │    │Kubernetes │   │  Infra  │
     │(Cloud │    │  Cluster  │   │PostgreSQL│
     │ Local)│    │           │   │ Redis   │
     └───────┘    └───────────┘   │ MinIO   │
                                  │ChromaDB │
                                  └─────────┘
```

---

## Project Structure

```
lingshu/
├── cmd/
│   ├── lingshu/          # Main CLI/TUI entry point
│   └── alertd/           # Alert webhook server
├── pkg/
│   ├── agent/            # Core agent loop, parser, timeout, circuit breaker
│   ├── alertd/           # Alert server (AlertManager/PagerDuty webhooks)
│   ├── audit/            # Audit logging with evidence chain
│   ├── cache/            # Redis cache wrapper
│   ├── config/           # Viper-based configuration
│   ├── db/               # Database abstraction (PostgreSQL / SQLite)
│   ├── gitops/           # ArgoCD/Flux ownership detection
│   ├── k8s/              # Kubernetes client manager, RBAC checks
│   ├── llm/              # LLM router (OpenAI, Claude, Ollama)
│   ├── logger/           # Structured logging (slog)
│   ├── rag/              # ChromaDB vector store for Runbook RAG
│   ├── scheduler/        # Cron job scheduler
│   ├── security/         # Five-level risk gateway
│   ├── session/          # Session persistence and lifecycle
│   ├── snapshot/         # Pre-mutation resource snapshots
│   ├── testutil/         # Shared test helpers and mocks
│   ├── tools/            # Tool interface + formatter
│   │   ├── l0/           # Read-only tools
│   │   ├── l1/           # Safe write tools
│   │   └── l2/           # Moderate risk tools
│   ├── tui/              # Terminal UI
│   │   ├── components/   # Chat, input, status bar, command preview
│   │   ├── models/       # TUI state model
│   │   ├── styles/       # Lipgloss style definitions
│   │   └── theme/        # Dark/light/high-contrast themes
│   └── workflow/         # DAG workflow engine
├── migrations/            # PostgreSQL migration scripts
├── charts/                # Helm chart for Kubernetes deployment
├── configs/               # Example configuration files
├── deployments/           # Kubernetes deployment manifests
├── tests/                 # Integration tests
├── docs/                  # PRDs, system design, task breakdown
├── Makefile               # Build, test, deploy targets
├── Dockerfile             # Multi-stage distroless build
└── docker-compose.yaml    # Dev environment (PG, Redis, MinIO, ChromaDB)
```

---

## Development

### Dev Environment

```bash
# Start dependencies (PostgreSQL, Redis, MinIO, ChromaDB)
make dev-up

# Initialize database schema
make migrate-up

# Build all binaries
make build

# Run all tests
make test

# Run tests with coverage
make test-coverage

# Lint
make lint
```

### Common Make Targets

| Target | Description |
|--------|------------|
| `make build` | Build all binaries |
| `make build-all` | Cross-compile (linux/darwin/windows × amd64/arm64) |
| `make test` | Run all unit tests with race detection |
| `make test-short` | Skip integration tests |
| `make test-integration` | Run integration tests (requires Kind cluster) |
| `make test-coverage` | Generate coverage report |
| `make lint` | Run golangci-lint |
| `make lint-fix` | Auto-fix lint issues |
| `make dev-up` | Start Docker Compose dev environment |
| `make dev-down` | Stop Docker Compose dev environment |
| `make migrate-up` | Run database migrations |
| `make migrate-down` | Rollback database migrations |
| `make kind-create` | Create local Kind cluster for testing |
| `make docker-build` | Build Docker image |
| `make helm-install` | Deploy to Kubernetes via Helm |

---

## Deployment

### Docker

```bash
docker build -t lingshu:latest .
docker run -e OPENAI_API_KEY=$OPENAI_API_KEY \
           -v ~/.kube:/home/nonroot/.kube \
           lingshu:latest --no-tui "Cluster health check"
```

### Kubernetes (Helm)

```bash
helm install lingshu ./charts/ops-ai \
  --namespace lingshu \
  --create-namespace \
  --set llm.apiKey=$OPENAI_API_KEY
```

### Docker Compose (Dev)

```bash
docker-compose up -d
```

---

## Test Coverage

| Package | Coverage | Notes |
|---------|----------|-------|
| `security` | 86.9% | Risk evaluators, rules, gateway |
| `rag` | 85.1% | ChromaDB vector store |
| `tools` | 75.2% | Tool interface, formatter |
| `llm` | 72.5% | OpenAI, Claude, Ollama providers |
| `alertd` | 68.4% | Alert webhook server |
| `gitops` | 64.6% | ArgoCD/Flux detection |
| `workflow` | 61.2% | Workflow engine |
| `scheduler` | 56.7% | Cron scheduler |
| `logger` | 55.0% | Structured logging |
| `k8s` | 41.2% | Client manager, RBAC |
| `tui/styles` | 100% | Style definitions |
| `tui/theme` | 100% | Theme system |

**Overall: 32.9%** (25 test suites, all passing)

---

## Configuration

Configuration is loaded from `~/.lingshu/config.yaml` or environment variables. Key sections:

```yaml
# LLM provider selection
llm:
  provider: openai       # openai | claude | ollama
  model: gpt-4o
  api_key: ${OPENAI_API_KEY}

# Agent loop behavior
agent:
  max_iterations: 10
  global_timeout: 5m
  dry_run: false

# Security gateway
security:
  strict_mode: true
  allow_l2_in_production: true
  allow_l3_in_production: false

# GitOps detection
gitops:
  detect_argocd: true
  detect_flux: true
```

---

## Roadmap

| Version | Theme | Status |
|---------|-------|--------|
| **v1.8** | MVP — Terminal interaction + basic diagnosis | ✅ Current |
| v1.9 | Enterprise-ready — Multi-tenancy, RBAC, secrets | 📋 Planned |
| v2.0 | Production scale — HA, performance optimization | 📋 Planned |
| v2.1 | Full-stack depth — GitOps, multi-cluster | 📋 Planned |
| v2.2 | Security & DR — Idempotency, audit chain | 📋 Planned |

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).

---

## Links

- **中文 README**: [README_zh.md](README_zh.md)
- **Design Docs**: [docs/](docs/)
- **API Spec**: [openapi.yaml](openapi.yaml)
