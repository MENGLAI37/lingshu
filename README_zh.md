# 灵枢 (LingShu) — AI 原生智能运维代理

> **用自然语言对话集群，让 AI 替你完成运维操作。**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org/)
[![CI](https://github.com/MENGLAI37/lingshu/actions/workflows/ci.yaml/badge.svg)](https://github.com/MENGLAI37/lingshu/actions/workflows/ci.yaml)
[![Coverage](https://img.shields.io/badge/coverage-33%25-yellow)]()

灵枢是一款面向 SRE（站点可靠性工程）团队的终端原生 AI 运维代理。它连接你的 Kubernetes 集群，理解自然语言运维意图，自主完成故障诊断与修复——全程受五级安全网关保护，未经你确认的高危操作绝不执行。

---

## 为什么选择灵枢？

| 痛点 | 灵枢的方案 |
|------|-----------|
| 一次排障要切换 5+ 个工具，耗时 15-25 分钟 | 一个终端，一句自然语言，30 秒出根因假设 |
| 凌晨 on-call，半睡半醒误操作生产环境 | 五级风险控制 (L0–L4)，环境感知动态评分 |
| 审计合规要准备 2-3 天 | 不可篡改的证据哈希链，一键生成审计报告 |
| LLM 幻觉输出 kubectl 命令，用户手动复制执行 | Agent Loop 强制工具调用，绝不输出原始命令 |
| GitOps Controller 回滚手动修复，产生对抗循环 | 内置 ArgoCD/Flux 检测，变更前冲突告警 |
| Agent 连续写操作失控 | 会话级熔断器 + 死循环检测 + L2 操作上限 |

---

## 快速开始

### 前置要求

- **Go 1.22+**
- **kubectl** 及有效 kubeconfig（或集群内 ServiceAccount）
- **LLM API Key** — OpenAI / DeepSeek / Claude / 本地 Ollama 任选其一

### 安装

```bash
git clone https://github.com/MENGLAI37/lingshu.git
cd lingshu
make build
```

### 运行

```bash
# 交互式 TUI 模式（推荐）
./bin/lingshu

# 无 TUI 模式 — 提问即答
./bin/lingshu --no-tui "生产环境 nginx 为什么一直重启？"

# CI/CD 管道模式 — 自动确认 L0-L2，机器可读输出
./bin/lingshu --no-tui --yes --pipe "把 payment-api 扩容到 5 个副本"

# 预览模式 — 完整推理链，零副作用
./bin/lingshu --no-tui --dry-run "重启所有异常 Pod"

# 自主运维演示
./bin/lingshu --auto-demo
```

### 配置 LLM

```bash
export OPENAI_API_KEY="sk-..."      # OpenAI
export DEEPSEEK_API_KEY="sk-..."    # DeepSeek
export ANTHROPIC_API_KEY="sk-..."   # Claude
# 或使用本地 Ollama — 自动探测 http://localhost:11434
```

首次运行时会自动检测 API Key 和 kubeconfig，缺失时给出引导式配置提示。

---

## 核心功能

### 🤖 自主 Agent 推理循环

核心引擎遵循 **思考 → 执行 → 观察 → 反思** 的闭环：

1. **Think（思考）** — LLM 分析当前状态，决策调用哪些工具
2. **Act（执行）** — 调用 K8s 工具；安全网关评估每次操作
3. **Observe（观察）** — 工具结果反馈到上下文，供下一轮推理
4. **Reflect（反思）** — 循环直到问题解决或超时（默认 5 分钟）

内置安全防护：

| 机制 | 作用 |
|------|------|
| **熔断器 (Circuit Breaker)** | 会话内 L2+ 操作上限；连续写操作自动暂停 |
| **死循环检测 (Dead-Loop Detection)** | 识别重复工具调用模式和错误循环 |
| **全局超时 (Global Timeout)** | context deadline 防止无限执行 |
| **Panic 恢复 (Panic Recovery)** | 崩溃时返回部分结果而非直接退出 |

### 🛡️ 五级安全网关 (L0–L4)

| 等级 | 颜色标识 | 说明 | 行为 |
|------|---------|------|------|
| **L0** | 绿色 | 只读查询 (`get`、`describe`、`logs`、`events`) | 自动执行 |
| **L1** | 蓝色 | 安全写入 (`top`、`status`) | 自动执行 |
| **L2** | 黄色 | 中等风险 (`scale`、`restart`、`rollout`、`patch`) | 确认后执行 |
| **L3** | 红色 | 高风险（破坏性操作） | 双重确认 |
| **L4** | 紫色 | 极高风险（集群级 RBAC、命名空间删除） | 拒绝执行 |

安全网关融合了 **三层风险评估器**（工具类型 + 环境上下文 + 资源敏感度）和 **三条阻断规则**（生产环境保护、命名空间隔离、集群级 RBAC 阻断）：

- `production` 环境 +20 风险分；`kube-system` 命名空间 +30 分
- ClusterRole / ClusterRoleBinding 修改一律阻断
- L3+ 操作需 on-call 在岗 + 变更窗口内
- **GitOps 冲突感知** — 修改 ArgoCD/Flux 管理资源前主动告警

### 📋 K8s 工具集

| 工具 | 等级 | 功能 |
|------|------|------|
| `k8s_get` | L0 | 查询 Pod、Deployment、Service、Event、Ingress、ConfigMap |
| `k8s_describe` | L0 | 资源详情及状态分析 |
| `k8s_logs` | L0 | Pod 日志拉取，支持 follow 流式输出 |
| `k8s_events` | L0 | 命名空间事件，Warning 分级 |
| `k8s_top` | L1 | Pod/Node 资源用量（CPU/内存） |
| `k8s_status` | L1 | 多维度集群健康检查（节点、Pod、部署状态） |
| `k8s_scale` | L2 | 扩缩容 Deployment、StatefulSet、ReplicaSet |
| `k8s_restart` | L2 | 通过 annotation 触发滚动重启 |
| `k8s_rollout` | L2 | 发布状态、历史、回滚、暂停、恢复 |
| `k8s_patch` | L2 | Strategic Merge / JSON Patch / Apply 补丁 |

所有工具在 Agent 工具注册中心统一管理。LLM 能看到完整的工具 Schema，自动决策调用哪个工具——**绝不输出原始 kubectl 命令让用户手动执行**。

### 🔍 自主运维引擎

`--auto-demo` 可演示完整的自主修复链路：

```
告警触发 → alertd 接收 webhook → 自主引擎启动诊断
→ 调用 K8s 工具收集信息 → LLM 分析根因 → 风险评估
→ L2+ 等待人工确认 → 执行修复 → 全程审计留痕
```

`alertd` 二进制可作为独立 Webhook 服务运行，接收 Prometheus AlertManager、PagerDuty 及通用告警源。

### 📊 审计与证据链

每条 L1+ 操作记录包含：
- **哈希证据链** — `prev_hash → content_hash` 链接，防篡改
- **批量异步写入** — 可配置批次大小，不阻塞 Agent 主循环
- **文件兜底 (File Fallback)** — 数据库不可用时自动写入 JSONL 文件
- **丰富元数据** — 前置检查结果、影响面分析、回滚信息、审批记录

支持按会话、用户、集群、命名空间、风险等级、时间范围筛选查询。

### 🖥️ 终端交互界面 (TUI)

基于 Bubble Tea 框架的现代化终端界面：

- **多行输入** — 支持粘贴 YAML/JSON，↑↓ 浏览历史
- **流式输出** — LLM 响应实时渲染，无卡顿
- **命令预览** — 执行前高亮展示命令及风险等级
- **状态栏** — 实时显示集群、命名空间、Token 用量、成本
- **语法高亮** — YAML / JSON / Table 结构化输出
- **主题切换** — 暗色 / 亮色 / 高对比度
- **配置面板** — 运行时切换 LLM Provider 和 Model
- **确认弹窗** — L2+ 操作前清晰的风险评估和影响面分析

### 🔗 GitOps 冲突检测

在执行 L2+ 操作前，灵枢会检查目标资源是否由 ArgoCD 或 Flux 管理（通过 annotation 和 label 检测）。若检测到 GitOps 管理，确认提示中会包含**冲突告警**，说明修改将被回滚，并给出正确的 GitOps 操作路径。

### 🧩 更多能力

- **多 LLM 路由** — 支持 OpenAI / Claude / Ollama，自动故障转移
- **RBAC 权限自检** — 启动时验证 ServiceAccount 对各工具的权限，缺失时明确提示
- **会话管理** — 完整 CRUD，支持父子会话链、TTL 过期、Token 预算
- **工作流引擎** — 基于 DAG 的工作流编排，支持条件分支和变量替换
- **定时调度器** — Cron / Interval / Once 三种模式，定时健康检查
- **变更前快照** — L2+ 操作前自动保存资源 YAML，支持回滚
- **配置热加载** — Viper + fsnotify，零停机配置变更

---

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                     lingshu (TUI/CLI)                    │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐             │
│  │ Bubble  │  │  Agent   │  │  安全网关  │             │
│  │ Tea TUI │  │  Loop    │  │  (L0-L4)   │             │
│  └─────────┘  └────┬─────┘  └─────┬──────┘             │
│                    │              │                      │
│         ┌──────────┼──────────────┼──────────┐          │
│         │          │              │          │          │
│    ┌────▼──┐ ┌────▼──┐ ┌───────▼──┐ ┌─────▼───┐       │
│    │  LLM  │ │  K8s  │ │  熔断器  │ │  审计   │       │
│    │ 路由器│ │ 工具集│ │          │ │  管理器 │       │
│    └───────┘ └───────┘ └──────────┘ └─────────┘       │
└─────────────────────────────────────────────────────────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
     ┌────▼──┐    ┌─────▼─────┐   ┌────▼────┐
     │  LLM  │    │Kubernetes │   │ 基础设施│
     │(云端  │    │  Cluster  │   │PostgreSQL│
     │ 本地) │    │           │   │ Redis   │
     └───────┘    └───────────┘   │ MinIO   │
                                  │ChromaDB │
                                  └─────────┘
```

---

## 项目结构

```
lingshu/
├── cmd/
│   ├── lingshu/          # 主程序入口 (TUI + CLI)
│   └── alertd/           # 告警 Webhook 服务
├── pkg/
│   ├── agent/            # 核心 Agent 循环、解析器、超时、熔断
│   ├── alertd/           # 告警接收 (AlertManager / PagerDuty webhook)
│   ├── audit/            # 审计日志 + 哈希证据链
│   ├── cache/            # Redis 缓存封装
│   ├── config/           # Viper 配置管理
│   ├── db/               # 数据库抽象层 (PostgreSQL / SQLite)
│   ├── gitops/           # ArgoCD / Flux 管理检测
│   ├── k8s/              # Kubernetes 客户端管理、RBAC 权限检查
│   ├── llm/              # LLM 路由 (OpenAI / Claude / Ollama)
│   ├── logger/           # 结构化日志 (slog)
│   ├── rag/              # ChromaDB 向量库 (Runbook RAG)
│   ├── scheduler/        # 定时任务调度器
│   ├── security/         # 五级风险安全网关
│   ├── session/          # 会话持久化及生命周期管理
│   ├── snapshot/         # 变更前资源快照
│   ├── testutil/         # 测试工具及 Mock
│   ├── tools/            # 工具接口 + 格式化器
│   │   ├── l0/           # 只读工具 (L0)
│   │   ├── l1/           # 安全写入 (L1)
│   │   └── l2/           # 中等风险 (L2)
│   ├── tui/              # 终端界面
│   │   ├── components/   # 聊天、输入、状态栏、命令预览等组件
│   │   ├── models/       # TUI 状态模型
│   │   ├── styles/       # Lipgloss 样式定义
│   │   └── theme/        # 暗色/亮色/高对比度主题
│   └── workflow/         # DAG 工作流引擎
├── migrations/            # PostgreSQL 迁移脚本
├── charts/                # Helm Chart (K8s 部署)
├── configs/               # 配置文件示例
├── deployments/           # K8s 部署清单
├── tests/                 # 集成测试
├── docs/                  # PRD、系统设计、任务拆解
├── Makefile               # 构建/测试/部署目标
├── Dockerfile             # 多阶段 Distroless 构建
└── docker-compose.yaml    # 开发环境 (PG + Redis + MinIO + ChromaDB)
```

---

## 开发指南

### 启动开发环境

```bash
# 启动依赖服务 (PostgreSQL, Redis, MinIO, ChromaDB)
make dev-up

# 初始化数据库
make migrate-up

# 构建所有二进制
make build

# 运行全部测试
make test

# 生成覆盖率报告
make test-coverage

# 代码检查
make lint
```

### Makefile 常用命令

| 命令 | 说明 |
|------|------|
| `make build` | 构建所有二进制 |
| `make build-all` | 交叉编译 (linux/darwin/windows × amd64/arm64) |
| `make test` | 运行单元测试（含 race 检测） |
| `make test-short` | 跳过集成测试 |
| `make test-integration` | 集成测试（需 Kind 集群） |
| `make test-coverage` | 生成覆盖率报告 |
| `make lint` | golangci-lint 代码检查 |
| `make lint-fix` | 自动修复 lint 问题 |
| `make dev-up` | 启动 Docker Compose 开发环境 |
| `make dev-down` | 停止 Docker Compose |
| `make migrate-up` | 执行数据库迁移 |
| `make migrate-down` | 回滚数据库迁移 |
| `make kind-create` | 创建本地 Kind 测试集群 |
| `make docker-build` | 构建 Docker 镜像 |
| `make helm-install` | Helm 部署到 Kubernetes |

---

## 部署

### Docker

```bash
docker build -t lingshu:latest .
docker run -e OPENAI_API_KEY=$OPENAI_API_KEY \
           -v ~/.kube:/home/nonroot/.kube \
           lingshu:latest --no-tui "集群健康检查"
```

### Kubernetes (Helm)

```bash
helm install lingshu ./charts/ops-ai \
  --namespace lingshu \
  --create-namespace \
  --set llm.apiKey=$OPENAI_API_KEY
```

### Docker Compose（开发环境）

```bash
docker-compose up -d
```

---

## 测试覆盖

| 包 | 覆盖率 | 说明 |
|-----|--------|------|
| `security` | 86.9% | 风险评估器、阻断规则、安全网关 |
| `rag` | 85.1% | ChromaDB 向量存储 |
| `tools` | 75.2% | 工具接口、格式化器 |
| `llm` | 72.5% | OpenAI、Claude、Ollama Provider |
| `alertd` | 68.4% | 告警 Webhook 服务 |
| `gitops` | 64.6% | ArgoCD/Flux 检测 |
| `workflow` | 61.2% | 工作流引擎 |
| `scheduler` | 56.7% | Cron 调度器 |
| `logger` | 55.0% | 结构化日志 |
| `k8s` | 41.2% | K8s 客户端、RBAC |
| `tui/styles` | 100% | 样式定义 |
| `tui/theme` | 100% | 主题系统 |

**总体覆盖率: 32.9%**（25 个测试套件，全部通过）

---

## 配置

配置文件默认路径 `~/.lingshu/config.yaml`，所有字段均可通过环境变量覆盖。

```yaml
# LLM 提供者选择
llm:
  provider: openai       # openai | claude | ollama
  model: gpt-4o
  api_key: ${OPENAI_API_KEY}

# Agent 循环行为
agent:
  max_iterations: 10
  global_timeout: 5m
  dry_run: false

# 安全网关
security:
  strict_mode: true
  allow_l2_in_production: true
  allow_l3_in_production: false

# GitOps 检测
gitops:
  detect_argocd: true
  detect_flux: true
```

---

## 路线图

| 版本 | 目标 | 状态 |
|------|------|------|
| **v1.8** | MVP — 终端交互 + 基础诊断 | ✅ 当前 |
| v1.9 | 企业就绪 — 多租户 / RBAC / 密钥管理 | 📋 规划中 |
| v2.0 | 生产规模化 — 高可用 / 性能优化 | 📋 规划中 |
| v2.1 | 全栈深度 — GitOps 深度运维 / 多集群 | 📋 规划中 |
| v2.2 | 安全灾备 — 幂等性 / 审计链完整性 | 📋 规划中 |

---

## 许可证

Apache License 2.0 — 详见 [LICENSE](LICENSE)。

---

## 相关链接

- **English README**: [README.md](README.md)
- **设计文档**: [docs/](docs/)
- **API 规范**: [openapi.yaml](openapi.yaml)
- **任务拆解**: [docs/lingshu-task-breakdown-final.md](docs/lingshu-task-breakdown-final.md)
- **系统设计**: [docs/ops-ai-agent-system-design.md](docs/ops-ai-agent-system-design.md)
