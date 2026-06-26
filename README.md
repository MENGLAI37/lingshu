# 灵枢 (LingShu)

> 面向 SRE 的 AI 原生智能运维代理

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org/)
[![Build Status](https://github.com/lingshu/lingshu/workflows/CI/badge.svg)](https://github.com/lingshu/lingshu/actions)

## 项目概述

灵枢（LingShu）是一款面向 Site Reliability Engineering (SRE) 领域的 AI 原生智能运维代理，旨在实现**自主故障响应**和**基础设施编排**。

### 核心价值

- 🤖 **AI 驱动**：基于 LLM 的自然语言交互，理解运维意图
- 🛡️ **安全第一**：五级风险控制 + 完整审计证据链
- ⚡ **即时响应**：秒级故障诊断，自动触发修复流程
- 🔄 **自主执行**：从诊断到修复的闭环自动化

## 核心功能

### 1.1 TUI 终端界面

基于 Bubble Tea 框架构建的交互式终端界面：

- 📝 **多行输入**：支持粘贴 YAML/JSON，↑/↓ 浏览历史
- 🌊 **流式输出**：LLM 响应实时渲染，无卡顿
- ⚠️ **命令预览**：执行前显示命令，风险等级高亮
- 📊 **状态栏**：显示集群、命名空间、Token 用量、成本
- 🎨 **语法高亮**：YAML/JSON/Table 结构化输出
- 🌓 **主题切换**：暗色/亮色/高对比度模式

### 1.2 命令执行与安全控制

| 风险等级 | 标识颜色 | 说明 | 操作 |
|---------|---------|------|------|
| L0 | 绿色 | 只读查询 | 自动执行 |
| L1 | 蓝色 | 低风险操作 | 确认后执行 |
| L2 | 黄色 | 中等风险 | 明确确认 |
| L3 | 红色 | 高风险操作 | 双重确认 |
| L4 | 紫色 | 极高风险 | 拒绝执行 |

### 1.3 智能诊断

支持多维度故障诊断：

- 🔍 Pod 重启原因分析
- 📈 资源瓶颈识别
- 🌐 网络连通性检查
- 💾 存储卷问题定位
- ⚙️ 配置错误检测

## 技术栈

| 层级 | 技术选型 |
|------|---------|
| **核心语言** | Go 1.22+ |
| **TUI 框架** | Bubble Tea / Bubbles / Lip Gloss |
| **Kubernetes** | client-go |
| **数据库** | PostgreSQL 15 / SQLite (fallback) |
| **缓存** | Redis 7.x (Sentinel) |
| **向量库** | ChromaDB |
| **对象存储** | MinIO (S3-compatible) |
| **日志** | slog (JSON 格式) |
| **配置** | Viper |
| **容器** | Docker / Kubernetes / Helm |

## 快速开始

### 前置要求

- Go 1.22+
- Docker & Docker Compose
- Kind (用于本地 K8s 测试)
- kubectl

### 1. 克隆代码

```bash
git clone https://github.com/lingshu/lingshu.git
cd lingshu
```

### 2. 启动开发环境

```bash
# 启动 PostgreSQL, Redis, MinIO, ChromaDB
make dev-up

# 初始化数据库
make migrate-up
```

### 3. 构建二进制

```bash
# 构建主程序
make build

# 或交叉编译
make build-all
```

### 4. 运行

```bash
# TUI 模式（推荐）
./bin/lingshu

# 无 TUI 模式
./bin/lingshu --no-tui

# 查看版本
./bin/lingshu --version
```

### 5. 运行测试

```bash
# 单元测试
make test

# 带覆盖率
make test-coverage

# 集成测试（需要 Kind 集群）
make kind-create
make test-integration
```

## 目录结构

```
lingshu/
├── cmd/                      # 可执行程序入口
│   ├── lingshu/             # 主程序 (TUI)
│   └── alertd/              # 告警 Webhook 服务
├── pkg/                      # 核心业务包
│   ├── cache/               # Redis 缓存封装
│   ├── config/               # 配置管理 (Viper)
│   ├── db/                   # 数据库封装 (sqlx)
│   ├── logger/               # 日志框架 (slog)
│   ├── tui/                  # TUI 组件
│   │   ├── components/       # UI 组件
│   │   │   ├── chat_view.go      # 聊天视图
│   │   │   ├── multiline_input.go # 多行输入
│   │   │   ├── stream_renderer.go # 流式渲染
│   │   │   ├── command_preview.go # 命令预览
│   │   │   ├── status_bar.go     # 状态栏
│   │   │   └── highlighted_renderer.go # 高亮渲染
│   │   ├── models/           # TUI 模型
│   │   ├── styles/           # 样式定义
│   │   └── theme/            # 主题系统
│   └── testutil/             # 测试工具
├── migrations/               # 数据库迁移脚本
├── charts/                  # Helm Chart
├── configs/                  # 配置文件示例
├── deployments/              # K8s 部署清单
├── tests/                    # 集成测试
└── docs/                     # 文档
```

## 开发指南

### Makefile 命令

```bash
# 构建
make build              # 构建所有二进制
make build-all          # 交叉编译 (linux/darwin/windows, amd64/arm64)

# 测试
make test               # 运行所有测试
make test-short         # 短测试（跳过集成测试）
make test-integration   # 集成测试
make test-coverage      # 生成覆盖率报告

# 代码质量
make lint               # 代码检查
make lint-fix           # 自动修复

# 开发环境
make dev-up             # 启动 Docker Compose
make dev-down           # 停止 Docker Compose
make dev-logs           # 查看容器日志

# 数据库
make migrate-up         # 执行迁移
make migrate-down       # 回滚迁移
make migrate-create     # 创建新迁移

# Kubernetes
make kind-create        # 创建 Kind 集群
make kind-delete        # 删除 Kind 集群
make helm-install       # 安装 Helm Chart
make helm-upgrade       # 升级 Helm Chart

# Docker
make docker-build       # 构建 Docker 镜像
make docker-push        # 推送镜像到仓库
```

### TUI 快捷键

| 按键 | 功能 |
|------|------|
| `Enter` | 发送消息 |
| `Shift+Enter` | 换行 |
| `↑/↓` | 浏览历史记录 |
| `?` | 显示/隐藏帮助 |
| `Esc` | 关闭弹窗/取消操作 |
| `Ctrl+C` / `q` | 退出程序 |
| `Y` | 确认执行命令 |
| `N` | 取消执行命令 |
| `PgUp/PgDn` | 翻页滚动 |
| `Home/End` | 跳转到开头/结尾 |

## 部署

### Docker Compose (开发环境)

```bash
docker-compose up -d
```

### Kubernetes (生产环境)

```bash
# 添加 Helm 仓库
helm repo add lingshu https://charts.lingshu.example.com
helm repo update

# 安装
helm install lingshu lingshu/lingshu \
  --namespace lingshu \
  --create-namespace \
  --set image.repository=ghcr.io/lingshu/lingshu \
  --set image.tag=v0.1.0
```

## API 文档

完整 REST API 规范见 [openapi.yaml](openapi.yaml)：

- 会话管理 (`/api/v2/sessions`)
- 对话与推理 (`/api/v2/sessions/{id}/chat`)
- 告警处理 (`/api/v2/alerts`)
- ChangeSet 操作 (`/api/v2/changesets`)
- 全局暂停 (`/api/v2/pauses`)
- 审计事件 (`/api/v2/audit/events`)
- 事件管理 (`/api/v2/incidents`)
- 健康检查 (`/api/v2/health`)

## 里程碑

| 版本 | 目标 | 状态 |
|------|------|------|
| v1.8 | MVP - 终端交互 + 基础诊断 | 🔄 开发中 |
| v1.9 | 企业就绪 - 多租户/RBAC/密钥 | 📋 规划中 |
| v2.0 | 规模化生产 - HA/性能优化 | 📋 规划中 |
| v2.1 | 全栈深度 - GitOps/多集群 | 📋 规划中 |
| v2.2 | 安全灾备 - 幂等性/审计链 | 📋 规划中 |

## 文档

- [研发任务拆解](docs/lingshu-task-breakdown-final.md)
- [PRD v2.3](docs/ops-ai-agent-prd-v2.3.md)
- [系统设计文档](docs/ops-ai-agent-system-design.md)

## 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 许可证

本项目采用 Apache License 2.0 - 详见 [LICENSE](LICENSE) 文件

## 联系方式

- 项目主页: https://github.com/lingshu/lingshu
- 文档: https://docs.lingshu.example.com
- 邮箱: team@lingshu.example.com
