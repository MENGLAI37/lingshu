# 运维 AI Agent 产品需求文档 (PRD) v1.9

> **文档用途**：面向产品设计师、架构师、开发团队的完整需求规格说明
> **版本**：v1.9 — 生产可靠版（补齐 Agent 自观测性、密钥/证书生命周期、灾难恢复、运行时护栏、多租户隔离、供应链安全、混沌工程、事件全生命周期、集群升级辅助、CSI 存储运维、容量规划、合规自动化、网络策略管理、Windows 支持、反馈闭环、插件机制、国际化、break-glass 紧急通道。从"生产安全"到"生产可靠 + 企业就绪"的跃迁。）
> **日期**：2026-06-25
> **变更**：v1.8 → v1.9 补齐了差距分析全部 18 项缺口（4 P0 + 8 P1 + 6 P2），详见末尾 Changelog

---

# 第一部分：给所有人的 Executive Summary

## 我们要做什么

v1.8 已经实现了"Agent 帮运维管集群"的生产级安全闭环。v1.9 的核心目标是补齐差距分析中的全部 18 项缺口，实现两条辅线的完整覆盖：

1. **"谁来管 Agent"** — Agent 自观测性、SLA/运行时护栏、灾难恢复
2. **"Agent 如何融入企业运维体系"** — 多租户隔离、供应链安全、合规自动化、事件全生命周期管理、容量规划
3. **"Agent 如何应对非理想环境"** — 证书/密钥轮换、集群升级辅助、CSI 深度运维、混沌工程、Windows 节点支持

## 一句话定位

> **v1.8 让 Agent 安全地管集群；v1.9 让 Agent 可靠地融入企业。**

## v1.9 与 v1.8 的关系

v1.9 是 v1.8 的**增量扩展**，不是重写。所有 v1.8 的功能分级、安全网关、回滚策略、审计日志、Agent Loop 模型均保持不变。v1.9 新增第 40-57 部分，解决差距分析中的 18 项缺口。

---

# 第二部分：差距分析 → 解决方案映射

| 优先级 | 编号 | 缺口 | 核心风险 | 解决方案所在章节 |
|--------|------|------|----------|----------------|
| **P0** | 1 | Agent 自观测性 | Agent 崩溃无人知，审计丢失无告警 | §40 |
| **P0** | 2 | 密钥轮换与证书生命周期 | 证书过期导致生产事故，合规不通过 | §41 |
| **P0** | 3 | 灾难恢复与集群级备份 | etcd 损坏/控制平面故障时 Agent 完全瘫痪 | §42 |
| **P0** | 4 | Agent SLA 与运行时护栏 | LLM 费用失控、数据库膨胀、并发打爆 | §43 |
| **P1** | 5 | 多租户/多团队隔离 | 团队间数据泄露、权限越界 | §44 |
| **P1** | 6 | 供应链安全 | 镜像漏洞、未签名镜像、准入策略冲突 | §45 |
| **P1** | 7 | 混沌工程集成 | 缺少主动韧性验证 | §46 |
| **P1** | 8 | 事件全生命周期管理 | 事故响应效率低，复盘质量差 | §47 |
| **P1** | 9 | K8s 集群升级辅助 | 升级风险高，缺乏预检和编排 | §48 |
| **P1** | 10 | CSI/存储运维深度 | 存储问题诊断能力弱，数据恢复无方案 | §49 |
| **P1** | 11 | 容量规划与资源推荐 | 资源浪费严重，容量预测缺失 | §50 |
| **P1** | 12 | 合规自动化 | 合规审计人力成本高，无法自动化 | §51 |
| **P2** | 13 | 网络策略主动管理 | 策略冲突难发现，无可视化 | §52 |
| **P2** | 14 | Windows 节点支持 | .NET 工作负载场景缺失 | §53 |
| **P2** | 15 | 反馈闭环与持续学习 | Agent 不会从经验中改进 | §54 |
| **P2** | 16 | 插件/扩展机制 | 无法注入自定义工具 | §55 |
| **P2** | 17 | 国际化/本地化 | 全球开源社区适配 | §56 |
| **P2** | 18 | 破窗效应防护 | P0 事故时审批人不可达无法操作 | §57 |

---

# 第三部分：P0 — 阻断级（不解决无法进入生产）

---

## 40. Agent 自观测性（v1.9 新增）

### 40.1 问题场景

v1.8 设计了完善的 K8s 可观测性诊断能力（Prometheus/Loki 查询、日志流式分析、健康概览），但**没有任何关于 Agent 自身可观测性的设计**：

- Agent 凌晨 3 点自动触发告警修复（§26），但 Agent 自身崩溃了谁通知运维？
- Agent 的 LLM 调用延迟突增（API provider 降级），运维如何感知？
- Agent 执行了一个 L3 操作后 hang 住（全局超时 §29 触发），但超时机制的可靠性谁来监控？
- 审计日志（§17）是异步写入的，如果磁盘满了导致审计丢失，如何告警？

### 40.2 设计目标

- Agent 必须能监控自己，崩溃时能自恢复并告警
- LLM API 延迟、token 消耗、工具成功率必须可观测
- 审计写入失败必须有降级策略和紧急告警
- 所有自观测指标以 Prometheus 格式暴露，可被现有监控体系消费

### 40.3 Go 接口定义

```go
// SelfObservabilityManager Agent 自观测管理器
type SelfObservabilityManager struct {
    metrics       *SelfMetrics
    healthChecker *HealthChecker
    alertPusher   *AlertPusher          // 向 alertd 反向推送自身异常
    config        SelfObservabilityConfig
}

type SelfMetrics struct {
    // LLM 指标
    LLMCallLatency    *prometheus.HistogramVec  // 按 provider/model 分桶
    LLMTokenConsumed  *prometheus.CounterVec    // input + output token
    LLMCallErrors     *prometheus.CounterVec    // 按错误类型分类
    
    // 工具执行指标
    ToolExecSuccess   *prometheus.CounterVec    // 按 tool_name 分类
    ToolExecFailure   *prometheus.CounterVec    // 按 tool_name + error_type 分类
    ToolExecDuration  *prometheus.HistogramVec  // 按 tool_name 分桶
    
    // 会话指标
    ActiveSessions    prometheus.Gauge
    SessionCrashes    prometheus.Counter
    SessionRecoveries prometheus.Counter
    
    // 审计指标
    AuditWriteSuccess prometheus.Counter
    AuditWriteFailure *prometheus.CounterVec    // 按 failure_reason 分类
    AuditQueueDepth   prometheus.Gauge          // 异步写入队列深度
    
    // 资源指标
    SQLiteDBSize      prometheus.Gauge          // 审计数据库大小（MB）
    SnapshotStoreSize prometheus.Gauge          // 快照存储大小（MB）
    MemoryUsage       prometheus.Gauge          // RSS（MB）
}

type HealthChecker struct {
    checks []HealthCheck
}

type HealthCheck struct {
    Name      string
    Interval  time.Duration
    CheckFunc func() HealthResult
    Severity  HealthSeverity // warning | critical
}

type HealthResult struct {
    Healthy bool
    Message string
    Detail  map[string]interface{}
}

type HealthSeverity string

const (
    HealthSeverityWarning  HealthSeverity = "warning"
    HealthSeverityCritical HealthSeverity = "critical"
)

// AuditWriteFailover 审计写入失败降级器
type AuditWriteFailover struct {
    diskCheckInterval time.Duration
    memoryBuffer      *ring.Ring        // 内存 ring buffer，磁盘满时临时存储
    alertFired        bool              // 避免重复告警
}
```

### 40.4 自观测指标端点

Agent 暴露 `/metrics`（Prometheus 格式），可被现有 Prometheus 抓取：

```
# 启动参数
ops-ai --metrics-bind :9090

# 指标示例
ops_ai_llm_call_latency_bucket{provider="openai",model="gpt-4",le="1.0"} 42
ops_ai_llm_call_latency_bucket{provider="openai",model="gpt-4",le="5.0"} 98
ops_ai_llm_token_consumed_total{provider="openai",model="gpt-4",type="input"} 152034
ops_ai_tool_exec_success_total{tool="kubectl_get"} 342
ops_ai_tool_exec_failure_total{tool="kubectl_apply",error="timeout"} 5
ops_ai_active_sessions 3
ops_ai_audit_write_failure_total{reason="disk_full"} 1
ops_ai_sqlite_db_size_mb 128.5
ops_ai_snapshot_store_size_mb 512.0
ops_ai_memory_usage_mb 64.2
```

### 40.5 健康检查项

| 检查项 | 频率 | 严重度 | 失败时的行为 |
|--------|------|--------|-------------|
| LLM API 连通性 | 30s | critical | 标记离线模式，触发本地模型 fallback |
| SQLite 审计写入 | 60s | critical | 切换到内存 ring buffer + 紧急告警 |
| 快照存储磁盘空间 | 60s | warning | TUI 警告；< 500MB 时停止自动快照 |
| Agent 内存占用 | 60s | warning | > 80% limit 时触发 GC + 告警 |
| kubeconfig 证书有效期 | 1h | warning | < 7 天时告警（§41 详细设计） |

### 40.6 审计写入失败降级策略

```go
func (a *AuditLogger) Write(entry AuditEntry) error {
    // 1. 尝试写入 SQLite
    err := a.sqliteSink.Write(entry)
    if err == nil {
        return nil
    }
    
    // 2. 判断是否为磁盘满
    if isDiskFull(err) {
        // 3. 写入内存 ring buffer（保留最近 1000 条）
        a.failover.memoryBuffer.Write(entry)
        
        // 4. 触发紧急告警（仅一次）
        if !a.failover.alertFired {
            a.alertPusher.Fire(Alert{
                Severity: "critical",
                Summary:  "Agent 审计日志磁盘已满，已切换到内存缓冲区",
                Runbook:  "检查 Agent 宿主机磁盘空间，清理旧快照或归档审计日志",
            })
            a.failover.alertFired = true
        }
        return nil // 不中断主流程
    }
    
    return err
}
```

### 40.7 TUI 自观测状态面板

```
  ═══════════════════════════════════════════════════════
  🤖  Agent 自观测状态
  ═══════════════════════════════════════════════════════

  健康检查
  ─────────────────────────────────────────────────────
  LLM API (OpenAI)      ✅ 正常          延迟: 1.2s
  SQLite 审计写入        ✅ 正常          队列: 0
  磁盘空间 (/data)       ⚠️  警告          剩余: 1.2GB
  内存占用               ✅ 正常          64MB / 256MB

  本会话指标
  ─────────────────────────────────────────────────────
  LLM 调用次数           12
  Token 消耗             8,432 input / 3,210 output
  工具执行成功率         98.5% (342/347)
  预计成本               $0.42

  [H] 隐藏面板  [R] 刷新  [D] 诊断详情
```

### 40.8 Agent 崩溃告警

利用 v1.6 已有的 alertd（§24）反向推送 Agent 自身异常：

```go
// AgentCrashReporter 崩溃报告器
type AgentCrashReporter struct {
    alertdEndpoint string
    sessionID      string
}

func (r *AgentCrashReporter) ReportCrash(recovery interface{}, stack []byte) {
    // 通过 alertd 的 webhook 接口推送
    payload := CrashReport{
        SessionID:     r.sessionID,
        Recovery:      fmt.Sprintf("%v", recovery),
        StackTrace:    string(stack),
        Timestamp:     time.Now(),
        LastOperation: getLastOperation(), // 从崩溃恢复 §29 获取
    }
    
    // alertd 收到后路由到运维通知渠道
    r.pushToAlertd(payload)
}
```

### 40.9 配置项

```yaml
# ~/.ops-ai/config.yaml
self_observability:
  enabled: true
  metrics_bind: ":9090"              # Prometheus metrics 端点
  health_check_interval: 30s
  
  # 审计写入失败降级
  audit_failover:
    memory_buffer_size: 1000          # 内存 ring buffer 条数
    disk_warning_threshold_gb: 1.0    # 磁盘剩余 < 1GB 时 warning
    disk_critical_threshold_gb: 0.5   # 磁盘剩余 < 500MB 时 critical
    
  # 告警推送（复用 alertd 配置）
  alert_push:
    alertd_url: "http://localhost:8080/webhook/agent-self"
    severity_filter: ["warning", "critical"]
```

### 40.10 System Prompt 补充

```
## Agent 自观测知识

当运维询问 Agent 自身状态时，你可以查询以下信息：
- `/self-status` — 查看 Agent 健康检查状态
- `/self-metrics` — 查看本会话的 LLM 调用、token 消耗、工具成功率
- `/self-audit` — 查看审计日志写入状态（包括 failover 模式）

如果 Agent 检测到自身异常（如 LLM API 延迟突增、磁盘空间不足），
应在回复开头主动告知运维：
"⚠️ Agent 自观测检测到 [异常描述]，可能影响排查效率。建议：[行动建议]"
```

---

## 41. 密钥轮换与证书生命周期管理（v1.9 新增）

### 41.1 问题场景

v1.8 在 Secret 脱敏（§5.6）、Secret 快照加密（AES-256-GCM）方面做得很好，也提到了 cert-manager 证书过期检查（§4 场景表），但**完全没有密钥轮换和证书续期的自动化设计**：

- 生产环境 TLS 证书过期是 P0 事故的常见原因（Let's Encrypt 90 天、企业 CA 1 年）
- 数据库密码、API Token 需要定期轮换（合规要求 90 天/180 天）
- kubeconfig 证书过期导致整个 Agent 无法连接集群
- Secret 轮换后关联的 Pod 需要重启，但 Agent 不知道哪些 Pod 依赖这个 Secret

### 41.2 设计目标

- 主动巡检所有证书过期时间，按风险等级排序预警
- 辅助 Secret 轮换：识别引用链，提示需要重启的 Pod
- 集成 External Secrets Operator，避免直接修改被 ESO 管控的 Secret
- kubeconfig 证书过期预警

### 41.3 Go 接口定义

```go
// CertificateLifecycleManager 证书生命周期管理器
type CertificateLifecycleManager struct {
    k8sClient     kubernetes.Interface
    cmClient      cmversioned.Interface  // cert-manager client
    scanner       *CertificateScanner
    secretAnalyzer *SecretDependencyAnalyzer
    config        CertificateConfig
}

// CertificateScanner 证书扫描器
type CertificateScanner struct {
    warningThreshold  time.Duration  // 默认: 30 天
    criticalThreshold time.Duration  // 默认: 7 天
}

type CertificateScanResult struct {
    Namespace     string
    Name          string
    Kind          string  // Certificate / Ingress / Secret / kubeconfig
    Type          string  // TLS / CA / kubeconfig / opaque-token
    ExpiresAt     time.Time
    DaysRemaining int
    Severity      string  // info | warning | critical | expired
    AutoRenewable bool    // 是否由 cert-manager 管理
    Action        string  // 建议的行动
}

// SecretDependencyAnalyzer Secret 依赖分析器
type SecretDependencyAnalyzer struct{}

type SecretDependencyChain struct {
    Secret    corev1.Secret
    UsedBy    []PodReference       // 直接挂载该 Secret 的 Pod
    Indirect  []IndirectReference  // 通过 EnvFrom/Projection 引用的 Pod
    CanRotate bool                 // 是否允许直接修改（非 ESO 管理）
    ESOInfo   *ESOManagedInfo      // 如果被 ESO 管理
}

type PodReference struct {
    Name      string
    Namespace string
    VolumeName string
    Key        string
}

type ESOManagedInfo struct {
    ExternalSecretName string
    StoreRef           string
    RefreshInterval    string
}

// SecretRotationPlanner Secret 轮换计划器
type SecretRotationPlanner struct{}

type RotationPlan struct {
    Secret        corev1.Secret
    DependentPods []PodReference
    Steps         []RotationStep
    RollbackPlan  []RollbackStep
}

type RotationStep struct {
    Order       int
    Action      string  // "update_secret" | "rolling_restart" | "verify"
    Target      string
    Verification string // 验证方式
}
```

### 41.4 证书过期巡检命令

```
ops-ai cert scan                          # 扫描所有证书
ops-ai cert scan --namespace payment      # 仅扫描 payment namespace
ops-ai cert scan --critical-only          # 仅显示 critical / expired
```

### 41.5 TUI 证书扫描结果展示

```
  ═══════════════════════════════════════════════════════
  📜  证书生命周期扫描结果
  ═══════════════════════════════════════════════════════

  🔴 Critical (≤ 7 天)
  ─────────────────────────────────────────────────────
  payment/tls-api-cert        TLS   2 天后过期   cert-manager ✅ 自动续期
  default/ingress-tls         TLS   已过期 3 天  手动管理 ❌ 需立即处理

  🟡 Warning (≤ 30 天)
  ─────────────────────────────────────────────────────
  kube-system/kubeconfig      kubeconfig   15 天后过期  需轮换
  monitoring/grafana-tls      TLS          22 天后过期  cert-manager ✅

  🟢 Healthy (> 30 天)
  ─────────────────────────────────────────────────────
  default/ca-bundle           CA    298 天后过期  无需行动

  [D] 查看详情  [R] 轮换 Secret  [E] 导出报告  [Q] 返回
```

### 41.6 Secret 轮换辅助流程

```
运维: "轮换 payment-api 的数据库密码 Secret"

Agent:
  1. 扫描 Secret "payment-api-db" 的依赖链
     ├─ 直接挂载: pod/payment-api-7d8f9, pod/payment-api-7d8fa
     ├─ 环境变量引用: pod/payment-worker-3a2b1
     └─ ESO 管理: ❌ 非 ESO 管理，可直接修改

  2. 生成轮换计划（TUI 展示）:
     Step 1: 更新 Secret "payment-api-db"（L2，需确认）
     Step 2: Rolling restart deployment/payment-api
     Step 3: Rolling restart deployment/payment-worker
     Step 4: 验证新 Pod 启动成功

  3. 运维确认后执行
```

### 41.7 kubeconfig 证书过期预警

```go
func (m *CertificateLifecycleManager) CheckKubeconfigCert(kubeconfigPath string) (*CertificateScanResult, error) {
    config, err := clientcmd.LoadFromFile(kubeconfigPath)
    if err != nil {
        return nil, err
    }
    
    for _, authInfo := range config.AuthInfos {
        if authInfo.ClientCertificateData != nil {
            cert, err := x509.ParseCertificate(authInfo.ClientCertificateData)
            if err != nil {
                continue
            }
            
            daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)
            severity := "info"
            if daysRemaining <= 7 {
                severity = "critical"
            } else if daysRemaining <= 30 {
                severity = "warning"
            }
            
            return &CertificateScanResult{
                Kind:          "kubeconfig",
                Name:          "current-kubeconfig",
                Type:          "kubeconfig",
                ExpiresAt:     cert.NotAfter,
                DaysRemaining: daysRemaining,
                Severity:      severity,
                AutoRenewable: false,
                Action:        "联系集群管理员轮换 kubeconfig 证书",
            }, nil
        }
    }
    return nil, nil
}
```

### 41.8 External Secrets Operator 集成

```go
// 检测 Secret 是否被 ESO 管理
func (a *SecretDependencyAnalyzer) DetectESOManagement(secret corev1.Secret) *ESOManagedInfo {
    // 检查 ownerReferences 是否包含 ExternalSecret
    for _, owner := range secret.OwnerReferences {
        if owner.Kind == "ExternalSecret" && owner.APIVersion == "external-secrets.io/v1beta1" {
            return &ESOManagedInfo{
                ExternalSecretName: owner.Name,
            }
        }
    }
    
    // 检查 annotations
    if esName, ok := secret.Annotations["externalsecrets.kubernetes.io/controller-owner-ref"]; ok {
        return &ESOManagedInfo{
            ExternalSecretName: esName,
        }
    }
    
    return nil
}
```

ESO 管理的 Secret 行为变更：
- **L0 诊断**：正常展示 Secret metadata，但 values 标记为 `[ESO-managed]`
- **L1-L3 修改**：直接拒绝，提示 "该 Secret 由 ExternalSecrets 管理，请修改 ExternalSecret 资源"
- **轮换建议**：提示运维修改对应的 ExternalSecret，触发 ESO 同步

### 41.9 配置项

```yaml
# ~/.ops-ai/config.yaml
certificate_lifecycle:
  enabled: true
  scan_interval: 24h                  # 自动巡检频率
  warning_threshold_days: 30
  critical_threshold_days: 7
  
  # 证书来源
  sources:
    cert_manager: true                # 扫描 Certificate 资源
    ingress_tls: true                 # 扫描 Ingress TLS
    secrets: true                     # 扫描类型为 tls 的 Secret
    kubeconfig: true                  # 启动时检查 kubeconfig
    
  # ESO 集成
  external_secrets:
    enabled: true
    api_version: "external-secrets.io/v1beta1"
```

### 41.10 L0 命令扩展

```
# 证书生命周期（新增 L0）
ops-ai cert scan [--namespace <ns>] [--critical-only]
ops-ai cert status <cert-name> [-n <namespace>]
ops-ai secret deps <secret-name> [-n <namespace>]    # 查看 Secret 依赖链
ops-ai secret rotate-plan <secret-name> [-n <namespace>]  # 生成轮换计划（不执行）
```

### 41.11 System Prompt 补充

```
## 证书与 Secret 生命周期知识

当运维询问证书或 Secret 相关问题时：

1. **证书过期检查**：主动使用 cert scan 工具获取证书列表，优先报告 critical/expired
2. **Secret 轮换**：
   - 首先检测 Secret 是否被 External Secrets Operator (ESO) 管理
   - 如果是 ESO 管理：提示修改 ExternalSecret 资源，不要直接修改 Secret
   - 如果不是 ESO 管理：生成轮换计划，包含依赖 Pod 的 rolling restart
3. **kubeconfig 证书**：Agent 启动时自动检查，< 7 天时在 TUI 状态栏显示 ⚠️
4. **cert-manager 管理的证书**：若 cert-manager 正常运行，通常无需人工干预；
   若 cert-manager 异常（如 Challenge 失败），则需要排查

常见证书类型：
- cert-manager Certificate → 自动续期（依赖 cert-manager 健康）
- Ingress TLS Secret → 可能由 cert-manager 或手动管理
- kubeconfig client certificate → 需集群管理员轮换
- 自签名 CA → 长期有效，但需跟踪过期时间
```

---

## 42. 灾难恢复（DR）与集群级备份策略（v1.9 新增）

### 42.1 问题场景

v1.8 的回滚策略（§7）设计得非常精细——按资源类型差异化回滚，但这只是**资源级回滚**，不是**灾难恢复**：

- etcd 损坏 = 整个集群失联，Agent 的 kubectl 全部失效，Agent 能做什么？
- PVC 快照文档明确说"数据不会自动恢复"（§7 表格），但生产环境 PV 数据丢失是常态需求
- 跨集群 DR：主集群挂了，Agent 如何帮助切换到备用集群？
- 控制平面故障：API Server 不可达时，Agent 的所有工具链都依赖 API Server，如何降级？

### 42.2 设计目标

- API Server 不可达时，Agent 能诊断 etcd 状态（通过节点 SSH 或云厂商 API）
- 集成 Velero 进行备份/恢复操作（L3）
- 控制平面降级模式：API Server 不可达时的紧急诊断能力
- PVC 数据快照：L3 操作前对关键 PVC 创建 CSI VolumeSnapshot
- 跨集群故障转移 runbook

### 42.3 Go 接口定义

```go
// DisasterRecoveryManager 灾难恢复管理器
type DisasterRecoveryManager struct {
    k8sClient      kubernetes.Interface
    veleroClient   veleroversioned.Interface
    cloudProvider  CloudProviderDR      // AWS/Azure/GCP 控制平面 API
    sshClient      *NodeSSHClient       // 节点 SSH（如配置允许）
    config         DRConfig
}

// EtcdDiagnoser etcd 诊断器
type EtcdDiagnoser struct {
    sshClient *NodeSSHClient
}

type EtcdHealthResult struct {
    Endpoint        string
    Healthy         bool
    MemberList      []EtcdMember
    Leader          string
    DbSize          int64
    DbSizeInUse     int64
    AlarmList       []string  // NOSPACE 等
    Suggestions     []string
}

type EtcdMember struct {
    ID         uint64
    Name       string
    PeerURLs   []string
    ClientURLs []string
    Healthy    bool
}

// VeleroIntegration Velero 集成
type VeleroIntegration struct {
    veleroClient veleroversioned.Interface
}

type BackupJob struct {
    Name            string
    Namespace       string           // velero namespace
    IncludedNS      []string         // 备份的 namespace
    IncludedResources []string
    StorageLocation string
    Phase           string           // New | InProgress | Completed | Failed
    StartTime       time.Time
    CompletionTime  *time.Time
    Errors          int
    Warnings        int
}

// CSISnapshotManager CSI 快照管理器
type CSISnapshotManager struct {
    snapshotClient snapshotversioned.Interface
}

type VolumeSnapshotResult struct {
    Name          string
    Namespace     string
    PVCName       string
    SnapshotClass string
    ReadyToUse    bool
    RestoreSize   int64
    CreatedAt     time.Time
}

// ClusterFailoverRunbook 集群故障转移 Runbook
type ClusterFailoverRunbook struct {
    PrimaryCluster   string
    StandbyCluster   string
    FailoverSteps    []FailoverStep
    Verification     []VerificationStep
}

type FailoverStep struct {
    Order       int
    Description string
    Command     string  // 可执行的命令模板
    Manual      bool    // 是否需要人工执行
}
```

### 42.4 etcd 健康检查（API Server 不可达时）

```
ops-ai dr etcd-check

# 当 API Server 不可达时自动触发
```

```
  ═══════════════════════════════════════════════════════
  🔥  控制平面降级模式 — etcd 诊断
  ═══════════════════════════════════════════════════════

  API Server 状态:  ❌ 不可达（连续 3 次超时）
  降级诊断路径:     通过控制平面节点 SSH / 云厂商 API

  etcd 集群状态
  ─────────────────────────────────────────────────────
  节点        状态      角色        DB 大小    告警
  ─────────────────────────────────────────────────────
  cp-01       ✅ 健康    Leader      512MB      无
  cp-02       ✅ 健康    Follower    512MB      无
  cp-03       ❌ 离线    Follower    --         --

  诊断结论
  ─────────────────────────────────────────────────────
  etcd 多数派存活（2/3），集群仍可服务。
  API Server 不可达可能是网络分区或 API Server Pod 崩溃。

  建议行动
  ─────────────────────────────────────────────────────
  1. 检查 cp-03 节点状态（通过云厂商控制台或节点 SSH）
  2. 检查 kube-system namespace 的 API Server Pod 状态
  3. 如果 API Server Pod 崩溃，尝试通过节点 SSH 查看日志

  [S] SSH 到 cp-03  [C] 查看云厂商事件  [Q] 返回
```

### 42.5 Velero 备份/恢复（L3 工具）

```
ops-ai velero backup create --name daily-backup --include-namespaces payment,order
ops-ai velero backup get                          # 查看备份列表
ops-ai velero restore create --from-backup daily-backup --include-namespaces payment
ops-ai velero schedule get                        # 查看定时备份策略
```

操作分级：
- **L0**：`velero backup get`, `velero restore get`（只读）
- **L3**：`velero backup create`, `velero restore create`（创建/删除备份）
  - 强制执行：影响面分析 + 自动快照 + 集群名确认 + 可选双人审批

### 42.6 CSI VolumeSnapshot 操作（L3）

```go
func (m *CSISnapshotManager) CreateSnapshot(ctx context.Context, pvcName, namespace string) (*VolumeSnapshotResult, error) {
    // 1. 检查 PVC 是否存在
    pvc, err := m.k8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }
    
    // 2. 查找合适的 VolumeSnapshotClass
    snapshotClass, err := m.findSnapshotClass(ctx, pvc.Spec.StorageClassName)
    if err != nil {
        return nil, fmt.Errorf("未找到适用于 StorageClass %s 的 VolumeSnapshotClass: %w", *pvc.Spec.StorageClassName, err)
    }
    
    // 3. 创建 VolumeSnapshot
    snapshot := &snapshotv1.VolumeSnapshot{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: fmt.Sprintf("%s-snap-", pvcName),
            Namespace:    namespace,
            Labels: map[string]string{
                "ops-ai/created-by": "agent",
                "ops-ai/pvc-name":   pvcName,
            },
        },
        Spec: snapshotv1.VolumeSnapshotSpec{
            Source: snapshotv1.VolumeSnapshotSource{
                PersistentVolumeClaimName: &pvcName,
            },
            VolumeSnapshotClassName: &snapshotClass,
        },
    }
    
    created, err := m.snapshotClient.SnapshotV1().VolumeSnapshots(namespace).Create(ctx, snapshot, metav1.CreateOptions{})
    if err != nil {
        return nil, err
    }
    
    return &VolumeSnapshotResult{
        Name:          created.Name,
        Namespace:     namespace,
        PVCName:       pvcName,
        SnapshotClass: snapshotClass,
        CreatedAt:     time.Now(),
    }, nil
}
```

### 42.7 控制平面降级模式

```go
// ControlPlaneDegradationMode 控制平面降级模式
type ControlPlaneDegradationMode struct {
    enabled       bool
    sshEnabled    bool
    cloudEnabled  bool
}

func (a *Agent) enterDegradationMode(reason string) {
    a.degradationMode = &ControlPlaneDegradationMode{
        enabled: true,
    }
    
    // TUI 展示降级状态
    a.tui.ShowDegradationBanner(reason)
    
    // 可用的降级工具
    a.availableTools = []Tool{
        // SSH 工具（如配置允许）
        &SSHNodeTool{enabled: a.config.DR.SSH.Enabled},
        // 云厂商 API 工具
        &CloudProviderTool{provider: a.config.CloudProvider},
        // etcd 诊断工具
        &EtcdDiagnoseTool{},
    }
}
```

降级模式 TUI：

```
  ═══════════════════════════════════════════════════════
  ⚠️  控制平面降级模式 — API Server 不可达
  ═══════════════════════════════════════════════════════

  原因: 连续 3 次 API Server 调用超时（最后成功: 14:32:15）

  可用工具（降级模式）
  ─────────────────────────────────────────────────────
  [1] etcd 诊断        — 通过节点 SSH 检查 etcd 健康
  [2] 节点 SSH         — 登录控制平面节点查看日志
  [3] 云厂商诊断       — 查询云厂商控制平面事件
  [4] 跨集群切换       — 切换到备用集群上下文

  ⚠️  降级模式下仅支持只读诊断，禁止修改操作

  [Q] 退出降级模式（重试 API Server 连接）
```

### 42.8 跨集群故障转移 Runbook

```go
func (m *DisasterRecoveryManager) GenerateFailoverRunbook(primary, standby string) (*ClusterFailoverRunbook, error) {
    return &ClusterFailoverRunbook{
        PrimaryCluster: primary,
        StandbyCluster: standby,
        FailoverSteps: []FailoverStep{
            {Order: 1, Description: "确认主集群完全不可恢复", Manual: true},
            {Order: 2, Description: "在备用集群提升主应用副本", Command: "kubectl scale deploy/app --replicas=3 --cluster %s", Manual: false},
            {Order: 3, Description: "切换 DNS/LoadBalancer 到备用集群", Manual: true},
            {Order: 4, Description: "验证备用集群服务健康", Command: "kubectl get pods --cluster %s", Manual: false},
            {Order: 5, Description: "通知相关团队", Manual: true},
        },
    }, nil
}
```

### 42.9 配置项

```yaml
# ~/.ops-ai/config.yaml
disaster_recovery:
  enabled: true
  
  # 控制平面降级模式
  degradation_mode:
    api_timeout_threshold: 3            # 连续超时次数触发降级
    auto_enter: true                    # 自动进入降级模式
    
  # 节点 SSH（可选，用于 etcd 诊断）
  ssh:
    enabled: false                      # 默认关闭，需显式开启
    private_key_path: "~/.ssh/ops-ai"
    user: "ubuntu"
    control_plane_nodes: []             # 控制平面节点 IP/主机名
    
  # Velero 集成
  velero:
    enabled: true
    namespace: "velero"
    default_backup_ttl: "720h"          # 30 天
    
  # CSI 快照
  csi_snapshot:
    enabled: true
    default_class: ""                   # 空则自动检测
    
  # 跨集群故障转移
  failover:
    primary_cluster: "prod"
    standby_cluster: "prod-dr"
    runbook_path: "~/.ops-ai/failover-runbook.yaml"
```

### 42.10 L0 命令扩展

```
# 灾难恢复（新增）
ops-ai dr etcd-check                    # etcd 健康诊断（降级模式自动触发）
ops-ai dr status                        # 查看当前 DR 状态
ops-ai velero backup get                # 查看 Velero 备份列表（L0）
ops-ai velero restore get               # 查看恢复任务列表（L0）
ops-ai snapshot get                     # 查看 VolumeSnapshot 列表（L0）
ops-ai failover runbook                 # 查看故障转移 Runbook（L0）
```

### 42.11 System Prompt 补充

```
## 灾难恢复知识

当集群出现严重故障时（API Server 不可达、etcd 损坏等）：

1. **自动降级模式**：如果 API Server 连续超时，Agent 会自动进入降级模式，
   此时可用工具受限，仅能进行诊断。

2. **etcd 诊断**：通过节点 SSH 或云厂商 API 检查 etcd 健康。
   - 多数派存活（>50%）→ etcd 集群仍可服务，问题可能在 API Server
   - 少数派存活 → 需要紧急恢复 etcd（联系集群管理员）

3. **Velero 恢复**：
   - 备份恢复是 L3 操作，需要完整确认流程
   - 恢复前必须确认备份时间点，避免数据丢失
   - 可以按 namespace 选择性恢复

4. **CSI 快照**：
   - L3 操作前可以自动对关键 PVC 创建快照
   - 快照不等于备份，存储层故障时快照也可能丢失
   - 关键数据应同时使用 Velero 备份

5. **跨集群切换**：
   - 主集群不可恢复时，参考 failover runbook 执行切换
   - 切换步骤中标注 [MANUAL] 的需要人工执行
```

---

## 43. Agent SLA 定义与运行时护栏（v1.9 新增）

### 43.1 问题场景

v1.8 定义了 Agent Loop 全局超时（§29）和会话级爆炸半径控制（§27），但**没有定义 Agent 自身的 SLA 和运行时资源护栏**：

- 单次排障会话消耗 $5+ 的 LLM 费用（12 轮对话 x GPT-4），谁来设上限？
- Agent 的 SQLite 数据库无限增长（审计日志+会话历史+快照索引），多久清理一次？
- Agent 在 CI/CD 模式下（§22 `--pipe`）被并发调用 100 次，LLM API rate limit 打爆怎么办？
- Agent 内存泄漏（Go 进程），长时间运行后 OOM，运维如何知道？

### 43.2 设计目标

- 单会话 token 预算硬上限：超过阈值时拒绝继续调用 LLM
- 审计日志/会话历史保留策略：自动归档和清理
- CI/CD 并发控制：信号量限流 + LLM API rate limit 感知退避
- Agent 进程资源基线和告警

### 43.3 Go 接口定义

```go
// SLAManager SLA 管理器
type SLAManager struct {
    tokenBudget   *TokenBudget
    retentionMgr  *RetentionManager
    concurrency   *ConcurrencyLimiter
    resourceGuard *ResourceGuard
}

// TokenBudget 单会话 token 预算
type TokenBudget struct {
    maxTokensPerSession   int   // 默认: 100,000
    maxCostPerSessionUSD  float64 // 默认: $5.00
    currentSessionTokens  int
    currentSessionCost    float64
    exhausted             bool
}

func (b *TokenBudget) CheckBudget(tokens int, cost float64) error {
    if b.exhausted {
        return fmt.Errorf("token 预算已耗尽（已使用 %d tokens / $%.2f）。请开始新会话继续排查。", 
            b.currentSessionTokens, b.currentSessionCost)
    }
    
    if b.currentSessionTokens+tokens > b.maxTokensPerSession {
        b.exhausted = true
        return fmt.Errorf("token 预算即将耗尽（已使用 %d / %d tokens）。建议开始新会话。",
            b.currentSessionTokens, b.maxTokensPerSession)
    }
    
    if b.currentSessionCost+cost > b.maxCostPerSessionUSD {
        b.exhausted = true
        return fmt.Errorf("成本预算即将耗尽（已使用 $%.2f / $%.2f）。建议开始新会话。",
            b.currentSessionCost, b.maxCostPerSessionUSD)
    }
    
    b.currentSessionTokens += tokens
    b.currentSessionCost += cost
    return nil
}

// RetentionManager 保留策略管理器
type RetentionManager struct {
    auditRetention    time.Duration  // 默认: 90 天
    sessionRetention  time.Duration  // 默认: 30 天
    snapshotRetention time.Duration  // 默认: 7 天
    cleanupInterval   time.Duration  // 默认: 24 小时
}

func (r *RetentionManager) RunCleanup(ctx context.Context) {
    ticker := time.NewTicker(r.cleanupInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.cleanupAuditLogs()
            r.cleanupSessionHistory()
            r.cleanupSnapshots()
        }
    }
}

// ConcurrencyLimiter CI/CD 并发限流器
type ConcurrencyLimiter struct {
    semaphore       chan struct{}    // 信号量
    maxConcurrent   int              // 默认: 10
    llmRateLimiter  *LLMRateLimiter  // LLM API rate limit 感知
}

type LLMRateLimiter struct {
    requestsPerMinute int
    tokensPerMinute   int
    backoffDuration   time.Duration
}

func (l *ConcurrencyLimiter) Acquire(ctx context.Context) error {
    select {
    case l.semaphore <- struct{}{}:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("获取并发信号量超时，当前并发数已达到上限 %d", l.maxConcurrent)
    }
}

func (l *ConcurrencyLimiter) Release() {
    select {
    case <-l.semaphore:
    default:
    }
}

// ResourceGuard 资源护栏
type ResourceGuard struct {
    maxMemoryMB     int64   // 默认: 512MB
    maxCPUPercent   float64 // 默认: 200% (2 cores)
    memoryThreshold float64 // 内存告警阈值: 80%
}

func (g *ResourceGuard) CheckMemory() (*ResourceAlert, error) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    usedMB := int64(m.Sys) / 1024 / 1024
    if usedMB > int64(float64(g.maxMemoryMB)*g.memoryThreshold) {
        return &ResourceAlert{
            Resource: "memory",
            Severity: "warning",
            Message:  fmt.Sprintf("Agent 内存使用率达到 %.0f%% (%dMB / %dMB)", 
                float64(usedMB)/float64(g.maxMemoryMB)*100, usedMB, g.maxMemoryMB),
            Suggestion: "考虑重启 Agent 或增加内存限制",
        }, nil
    }
    return nil, nil
}
```

### 43.4 TUI 预算与资源状态展示

```
  ═══════════════════════════════════════════════════════
  💰  本会话 SLA / 预算状态
  ═══════════════════════════════════════════════════════

  Token 预算
  ─────────────────────────────────────────────────────
  已使用:     34,521 / 100,000 tokens (34.5%)
  预估成本:   $1.42 / $5.00 (28.4%)
  状态:       ✅ 正常

  资源使用
  ─────────────────────────────────────────────────────
  内存:       128MB / 512MB (25.0%)  ✅
  CPU:        45% / 200%             ✅
  运行时间:   12m 34s

  ⚠️  当 token 使用超过 80% 时将发出警告，超过 100% 时强制终止会话。

  [Q] 关闭面板
```

预算耗尽时的 TUI：

```
  ═══════════════════════════════════════════════════════
  🛑  会话预算已耗尽
  ═══════════════════════════════════════════════════════

  本会话已消耗:
  - Tokens:   102,341 / 100,000
  - 成本:     $5.23 / $5.00

  为了保护成本，Agent 已停止调用 LLM。

  你的选项:
  ─────────────────────────────────────────────────────
  [N] 开始新会话（重置预算，保留当前上下文摘要）
  [R] 查看当前排查摘要（无需 LLM）
  [C] 取消

  提示: 使用 `/cost` 命令随时查看成本状态。
```

### 43.5 CI/CD 并发控制

```go
func (a *Agent) ExecuteCICD(ctx context.Context, input string, mode string) error {
    // 1. 获取并发信号量
    if err := a.slaManager.concurrency.Acquire(ctx); err != nil {
        return fmt.Errorf("并发限流: %w", err)
    }
    defer a.slaManager.concurrency.Release()
    
    // 2. 检查 LLM API rate limit
    if err := a.slaManager.concurrency.llmRateLimiter.Wait(ctx); err != nil {
        return fmt.Errorf("LLM API rate limit: %w", err)
    }
    
    // 3. 设置 CI/CD 模式的更严格预算
    a.slaManager.tokenBudget.maxTokensPerSession = 50000  // CI/CD 模式更严格
    a.slaManager.tokenBudget.maxCostPerSessionUSD = 2.00
    
    // 4. 执行
    return a.Run(ctx, input)
}
```

CI/CD 并发超限时的输出（`--pipe` 模式）：

```
{"status": "error", "error": "并发限流：当前并发数已达到上限 10，请稍后重试", "retry_after": "30s", "code": 429}
```

### 43.6 自动清理策略

| 数据类型 | 保留期 | 清理动作 | 归档位置 |
|----------|--------|----------|----------|
| 审计日志 | 90 天 | 自动删除 | 可配置 S3/GCS |
| 会话历史 | 30 天 | 自动删除 | 本地压缩归档 |
| 快照索引 | 7 天 | 自动删除 | 无 |
| 自动快照文件 | 与快照索引同步 | 自动删除 | 无 |
| 崩溃恢复记录 | 30 天 | 自动删除 | 无 |

```go
func (r *RetentionManager) cleanupAuditLogs() {
    cutoff := time.Now().Add(-r.auditRetention)
    
    // 先归档（如配置了归档目标）
    if r.archiveTarget != nil {
        r.archiveAuditLogsBefore(cutoff)
    }
    
    // 再删除
    r.db.Exec("DELETE FROM audit_logs WHERE timestamp < ?", cutoff)
}
```

### 43.7 Agent 进程资源限制建议

```yaml
# Agent 容器化部署时的资源限制建议（补充 §31）
# ops-ai-deployment.yaml
resources:
  limits:
    memory: "512Mi"
    cpu: "1000m"
  requests:
    memory: "128Mi"
    cpu: "100m"
```

### 43.8 配置项

```yaml
# ~/.ops-ai/config.yaml
sla:
  enabled: true
  
  # Token 预算
  token_budget:
    max_tokens_per_session: 100000      # 单会话 token 上限
    max_cost_usd_per_session: 5.00      # 单会话成本上限
    warning_threshold_percent: 80       # 80% 时发出警告
    
  # 保留策略
  retention:
    audit_logs: "2160h"                 # 90 天
    session_history: "720h"             # 30 天
    snapshot_index: "168h"              # 7 天
    cleanup_interval: "24h"
    archive_target: ""                  # 归档目标（如 s3://bucket/ops-ai-audit/）
    
  # CI/CD 并发控制
  concurrency:
    max_concurrent_sessions: 10         # 最大并发会话数
    llm_requests_per_minute: 60         # LLM API RPM 限制
    llm_tokens_per_minute: 100000       # LLM API TPM 限制
    backoff_base: "1s"
    backoff_max: "30s"
    
  # 资源护栏
  resource_guard:
    max_memory_mb: 512
    memory_warning_threshold: 0.8       # 80%
    max_cpu_percent: 200.0
```

### 43.9 System Prompt 补充

```
## Agent SLA 与资源管理知识

当运维询问成本或资源相关问题时：

1. **成本查询**：使用 `/cost` 命令查看当前会话的 token 消耗和预估成本
2. **预算耗尽**：如果会话预算已耗尽，你可以：
   - 提供已排查的摘要（不调用 LLM）
   - 建议开始新会话（重置预算）
   - 提示运维使用本地模型（如果配置）降低成本
3. **历史查询**：超过保留期的会话历史已自动清理，无法查询
4. **并发限制**：CI/CD 模式下有并发上限，超限时会返回 429 错误
```

---

# 第四部分：P1 — 重要级（规模化前应解决）

---

## 44. 多租户/多团队隔离（v1.9 新增）

### 44.1 问题场景

v1.8 的 RBAC 清单（§16）定义了 Agent 需要的 K8s 权限，但这是**Agent 视角的权限**，不是**多团队使用时的隔离设计**。全文仅在审计日志中出现一次 `tenant_id: "ops-ai-audit"`，没有任何多租户架构设计：

- 企业里 3 个 SRE 团队共享一个 Agent 部署，A 团队的 SRE 不应该看到 B 团队的审计日志
- 不同团队有不同集群权限：支付团队只能操作 payment namespace，平台团队可以全集群
- Agent 的配置文件（LLM API Key、成本预算）需要按团队隔离
- 会话历史包含敏感信息（Secret 脱敏后的上下文仍有泄露风险），跨团队可见性需要控制

### 44.2 设计目标

- 团队/用户身份模型：所有操作按团队归属
- 审计日志按团队隔离查询
- 配置继承：全局 → 团队 → 用户
- 会话可见性三级控制：私有 / 团队内共享 / 全局共享
- Namespace 权限映射：按团队绑定可操作的白名单

### 44.3 Go 接口定义

```go
// TenantManager 多租户管理器
type TenantManager struct {
    identityProvider IdentityProvider  // OIDC / SAML / 本地
    teams            map[string]*Team
    users            map[string]*User
    config           TenantConfig
}

type IdentityProvider interface {
    Authenticate(ctx context.Context, token string) (*User, error)
    GetTeamMembership(userID string) ([]string, error)
}

type Team struct {
    ID              string
    Name            string
    AllowedNamespaces []string         // namespace 白名单
    LLMBudget       *LLMBudget         // 团队级 LLM 预算
    DefaultConfig   map[string]interface{}
}

type User struct {
    ID       string
    Name     string
    Email    string
    Teams    []string
    Role     UserRole  // admin | operator | viewer
}

type UserRole string

const (
    UserRoleAdmin    UserRole = "admin"
    UserRoleOperator UserRole = "operator"
    UserRoleViewer   UserRole = "viewer"
)

// SessionVisibility 会话可见性
type SessionVisibility string

const (
    SessionVisibilityPrivate SessionVisibility = "private"   // 仅自己可见
    SessionVisibilityTeam    SessionVisibility = "team"      // 团队内可见
    SessionVisibilityGlobal  SessionVisibility = "global"    // 全局可见（仅 admin）
)

type TeamSessionStore struct {
    teamID      string
    sessions    map[string]*TeamSession
}

type TeamSession struct {
    SessionID   string
    OwnerID     string
    TeamID      string
    Visibility  SessionVisibility
    Summary     string  // 脱敏后的会话摘要
    CreatedAt   time.Time
}

// NamespaceACL namespace 访问控制
type NamespaceACL struct {
    teamID      string
    allowedNS   []string
    deniedNS    []string  // 优先级高于 allowed
}

func (a *NamespaceACL) CanAccess(namespace string) bool {
    for _, d := range a.deniedNS {
        if d == namespace {
            return false
        }
    }
    for _, allowed := range a.allowedNS {
        if allowed == namespace || allowed == "*" {
            return true
        }
    }
    return false
}
```

### 44.4 团队身份与启动流程

```bash
# 本地模式：通过 flag 指定团队
ops-ai --team payment --user alice

# 企业模式：通过 OIDC token
ops-ai --oidc-token $ID_TOKEN

# CI/CD 模式：通过 ServiceAccount 映射
ops-ai --pipe --team ci-cd-team
```

启动时的团队验证：

```
  ═══════════════════════════════════════════════════════
  👥  团队身份验证
  ═══════════════════════════════════════════════════════

  用户:       alice@company.com
  团队:       payment-team
  角色:       operator
  可用命名空间: payment, payment-staging
  LLM 预算:   $100.00 / 月（本月已用 $23.45）

  [C] 继续  [S] 切换团队  [Q] 退出
```

### 44.5 审计日志按团队隔离

```go
func (a *AuditLogger) Query(ctx context.Context, req AuditQueryRequest) ([]AuditEntry, error) {
    // 1. 验证用户身份
    user := a.tenantManager.GetCurrentUser()
    
    // 2. 构建查询条件
    query := a.db.Where("timestamp >= ? AND timestamp <= ?", req.Start, req.End)
    
    // 3. 团队隔离：非 admin 只能查自己团队的数据
    if user.Role != UserRoleAdmin {
        query = query.Where("team_id = ?", user.Teams[0])
    }
    
    // 4. 用户级过滤（可选）
    if req.FilterUserID != "" && user.Role != UserRoleAdmin {
        // 只能查自己或团队成员
        if req.FilterUserID != user.ID && !a.isSameTeam(user.ID, req.FilterUserID) {
            return nil, fmt.Errorf("无权查询该用户的审计日志")
        }
    }
    
    // 5. 执行查询
    var entries []AuditEntry
    err := query.Find(&entries).Error
    return entries, err
}
```

审计查询命令：

```
ops-ai audit query --since 24h                    # 查本团队 24h 内的审计
ops-ai audit query --since 24h --team payment     # admin 可指定团队
ops-ai audit query --user alice --since 7d        # 查指定用户的审计
```

### 44.6 配置继承机制

```yaml
# /etc/ops-ai/global-config.yaml   # 全局配置（管理员维护）
llm:
  default_provider: "openai"
  fallback_provider: "local"
  
safety:
  blast_radius:
    enabled: true
    max_l2_per_namespace: 3

# /etc/ops-ai/teams/payment-team.yaml  # 团队级配置
team_id: "payment-team"
allowed_namespaces: ["payment", "payment-staging"]
llm_budget_monthly: 100.00

llm:
  provider: "openai"                    # 可覆盖全局
  model: "gpt-4"                        # 团队指定模型

# ~/.ops-ai/config.yaml  # 用户级配置（优先级最高）
user:
  default_team: "payment-team"
  
llm:
  model: "gpt-4-turbo"                  # 用户偏好模型
```

配置合并逻辑：

```go
func (c *Config) Merge() *MergedConfig {
    result := c.Global.DeepCopy()
    
    // 团队配置覆盖全局
    if c.Team != nil {
        result.MergeTeam(c.Team)
    }
    
    // 用户配置覆盖团队
    if c.User != nil {
        result.MergeUser(c.User)
    }
    
    return result
}
```

### 44.7 会话可见性控制

```
运维: "/session share team"

Agent:
  将会话可见性设置为 "team"。
  团队内其他成员可以通过 `/session list --team` 查看会话摘要。
  
  ⚠️  注意：共享会话时， Secret 值和敏感操作参数仍会被脱敏。

运维: "/session list --team"

Agent:
  payment-team 的共享会话:
  ─────────────────────────────────────────────────────
  ID          创建者    摘要                    时间
  ─────────────────────────────────────────────────────
  sess-abc    alice     payment-api OOM 排查      2h ago
  sess-def    bob       ingress 证书过期处理      5h ago
```

### 44.8 配置项

```yaml
# ~/.ops-ai/config.yaml
tenant:
  enabled: true
  
  # 身份提供商
  identity_provider:
    type: "oidc"                        # oidc | saml | local
    oidc:
      issuer_url: "https://auth.company.com"
      client_id: "ops-ai-agent"
      # 本地缓存 token
      token_cache_path: "~/.ops-ai/.oidc-token"
      
  # 当前用户/团队
  current_team: "payment-team"
  current_user: "alice@company.com"
  
  # 会话可见性默认值
  default_session_visibility: "private"  # private | team | global
  
  # Namespace ACL
  namespace_acl:
    mode: "whitelist"                   # whitelist | blacklist
    default_allow: false                # 默认拒绝
```

### 44.9 System Prompt 补充

```
## 多租户与团队隔离知识

当前会话的团队上下文：
- 团队: {current_team}
- 可用命名空间: {allowed_namespaces}
- 角色: {user_role}

约束：
1. 所有操作必须在允许的命名空间内执行
2. 审计日志会自动归属到当前团队
3. 会话默认私有，可通过 `/session share team` 共享给团队
4. 非 admin 用户不能查询其他团队的审计日志

当运维请求越权操作时：
"⚠️ 当前团队 '{team}' 没有权限操作 namespace '{namespace}'。
允许的命名空间: {allowed_namespaces}。
请联系平台团队申请权限。"
```

---

## 45. 供应链安全（v1.9 新增）

### 45.1 问题场景

v1.8 没有提及任何供应链安全相关内容——SBOM 生成、镜像签名验证、漏洞扫描、准入控制策略：

- Agent 自己以 distroless 镜像部署（§31），但镜像没有签名验证流程
- Agent 诊断 Pod 时发现了镜像漏洞，但没有集成 Trivy/Grype 做自动扫描
- Agent 执行 `kubectl apply` 时，如果集群有 OPA Gatekeeper/Kyverno 准入策略，Agent 被 deny 后只是报错（§5.7 提到了 admission webhook 感知），但没有主动合规预检
- Agent 推荐用户部署某个镜像，但没验证该镜像是否被签名/是否有已知漏洞

### 45.2 设计目标

- 诊断 Pod 时自动检查镜像 CVE
- Agent 自身镜像 Cosign 签名 + 部署时验证指南
- apply 操作前检查 OPA Gatekeeper/Kyverno 策略，预测是否会被拒绝
- Agent 镜像构建时生成 SBOM

### 45.3 Go 接口定义

```go
// SupplyChainSecurityManager 供应链安全管理器
type SupplyChainSecurityManager struct {
    scanner       ImageScanner           // Trivy/Grype 集成
    verifier      ImageVerifier          // Cosign/Sigstore 验证
    policyChecker AdmissionPolicyChecker // 准入策略预检
    sbomGenerator SBOMGenerator
}

// ImageScanner 镜像漏洞扫描器
type ImageScanner interface {
    Scan(ctx context.Context, imageRef string) (*ScanResult, error)
}

type TrivyScanner struct {
    serverURL string  // Trivy Server 地址
}

type ScanResult struct {
    ImageRef        string
    ScanTime        time.Time
    Vulnerabilities []Vulnerability
    Summary         VulnSummary
}

type Vulnerability struct {
    VulnID       string   // CVE-2024-XXXX
    Severity     string   // CRITICAL | HIGH | MEDIUM | LOW
    PackageName  string
    InstalledVersion string
    FixedVersion string
    Description  string
}

type VulnSummary struct {
    Critical int
    High     int
    Medium   int
    Low      int
}

// ImageVerifier 镜像签名验证器
type ImageVerifier interface {
    Verify(ctx context.Context, imageRef string) (*VerifyResult, error)
}

type CosignVerifier struct {
    keyRef     string  // 公钥或 KMS 引用
    rekorURL   string  // Rekor 透明日志地址
}

type VerifyResult struct {
    ImageRef    string
    Verified    bool
    SignatureCount int
    SignerIdentities []string
    TransparencyLog  bool  // 是否上传到 Rekor
}

// AdmissionPolicyChecker 准入策略预检器
type AdmissionPolicyChecker struct {
    k8sClient kubernetes.Interface
}

type PolicyCheckResult struct {
    Allowed    bool
    DeniedBy   string  // 哪个策略拒绝
    Reason     string
    Violations []PolicyViolation
    Suggestions []string
}

type PolicyViolation struct {
    Policy   string  // kyverno:require-labels / gatekeeper:required-annotations
    Rule     string
    Message  string
}
```

### 45.4 镜像漏洞扫描集成

诊断 Pod 时自动触发镜像扫描：

```go
func (a *Agent) diagnosePod(ctx context.Context, pod corev1.Pod) (*DiagnosisResult, error) {
    result := &DiagnosisResult{}
    
    // 1. 常规诊断
    // ...
    
    // 2. 镜像漏洞扫描（v1.9 新增）
    for _, container := range pod.Spec.Containers {
        scanResult, err := a.supplyChainSecurity.scanner.Scan(ctx, container.Image)
        if err != nil {
            result.Warnings = append(result.Warnings, 
                fmt.Sprintf("镜像 %s 漏洞扫描失败: %v", container.Image, err))
            continue
        }
        
        if scanResult.Summary.Critical > 0 || scanResult.Summary.High > 0 {
            result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
                Type:     "image-vuln",
                Severity: "high",
                Message:  fmt.Sprintf("镜像 %s 发现 %d Critical + %d High 漏洞", 
                    container.Image, scanResult.Summary.Critical, scanResult.Summary.High),
                Detail: scanResult,
            })
        }
    }
    
    return result, nil
}
```

TUI 展示漏洞扫描结果：

```
  ═══════════════════════════════════════════════════════
  🔒  镜像安全扫描结果
  ═══════════════════════════════════════════════════════

  Pod: payment-api-7d8f9
  容器: payment-api
  镜像: registry.company.com/payment-api:v2.3.1

  漏洞摘要
  ─────────────────────────────────────────────────────
  Critical:  1  🔴
  High:      3  🟠
  Medium:    12 🟡
  Low:       45 🟢

  关键漏洞 (Critical)
  ─────────────────────────────────────────────────────
  CVE-2024-21626  glibc 2.35   CRITICAL  容器逃逸漏洞
                  已修复版本: 2.35-1ubuntu2.1

  建议行动
  ─────────────────────────────────────────────────────
  1. 升级镜像到 registry.company.com/payment-api:v2.3.2
  2. 紧急程度: 建议本周内修复（Critical 容器逃逸）

  [D] 查看全部漏洞  [I] 查看修复指南  [Q] 返回
```

### 45.5 准入策略预检

apply 操作前自动检查：

```go
func (p *AdmissionPolicyChecker) PreCheck(ctx context.Context, obj unstructured.Unstructured, namespace string) (*PolicyCheckResult, error) {
    result := &PolicyCheckResult{Allowed: true}
    
    // 1. 检查 Kyverno 策略
    kyvernoPolicies, err := p.listKyvernoPolicies(ctx)
    if err == nil {
        for _, policy := range kyvernoPolicies {
            if violation := p.checkKyvernoPolicy(policy, obj); violation != nil {
                result.Allowed = false
                result.Violations = append(result.Violations, *violation)
            }
        }
    }
    
    // 2. 检查 Gatekeeper 约束
    constraints, err := p.listGatekeeperConstraints(ctx)
    if err == nil {
        for _, constraint := range constraints {
            if violation := p.checkGatekeeperConstraint(constraint, obj); violation != nil {
                result.Allowed = false
                result.Violations = append(result.Violations, *violation)
            }
        }
    }
    
    // 3. 生成建议
    if !result.Allowed {
        result.Suggestions = p.generateFixSuggestions(result.Violations)
    }
    
    return result, nil
}
```

准入策略预检 TUI：

```
  ═══════════════════════════════════════════════════════
  🛡️  准入策略预检结果
  ═══════════════════════════════════════════════════════

  操作: apply deployment/payment-api

  检查结果: ❌ 将被拒绝

  违反策略 (2)
  ─────────────────────────────────────────────────────
  1. kyverno:require-labels
     规则: check-team-label
     消息: Deployment 必须包含标签 "team"
     
  2. gatekeeper:required-annotations
     消息: 缺少注解 "cost-center"

  修复建议
  ─────────────────────────────────────────────────────
  添加以下内容到 Deployment:
    metadata:
      labels:
        team: "payment"
      annotations:
        cost-center: "cc-12345"

  [A] 自动修复并重新预检  [C] 取消操作  [F] 强制应用（将被拒绝，仅查看错误）
```

### 45.6 Agent 镜像签名验证

Agent 自身镜像构建时签名：

```dockerfile
# Dockerfile 补充（§31 扩展）
# Stage 3: Sign (CI 中执行)
FROM gcr.io/projectsigstore/cosign:v2.2.0 as signer
COPY --from=builder /app/ops-ai /ops-ai
RUN cosign sign --key env://COSIGN_PRIVATE_KEY /ops-ai
```

部署时验证：

```bash
# 验证 Agent 镜像签名
cosign verify --key cosign.pub registry.company.com/ops-ai:v1.9.0

# 启动时 Agent 自检签名（可选）
ops-ai --verify-image-signature
```

### 45.7 SBOM 生成

```go
// 构建时生成 SBOM
type SBOMGenerator struct{}

func (g *SBOMGenerator) Generate(imageRef string) (*SBOM, error) {
    // 使用 syft 或 trivy 生成 SBOM
    // 输出格式: SPDX-JSON 或 CycloneDX-JSON
}
```

### 45.8 L0 命令扩展

```
# 供应链安全（新增 L0）
ops-ai scan image <image-ref>              # 扫描镜像漏洞
ops-ai verify image <image-ref>            # 验证镜像签名
ops-ai policy check <file.yaml>            # 准入策略预检
ops-ai sbom get <image-ref>                # 查看镜像 SBOM
ops-ai policy list                         # 列出集群准入策略
```

### 45.9 配置项

```yaml
# ~/.ops-ai/config.yaml
supply_chain_security:
  enabled: true
  
  # 镜像扫描
  image_scanner:
    type: "trivy"                       # trivy | grype
    trivy_server_url: "http://trivy-server:8080"
    auto_scan_on_diagnose: true         # 诊断 Pod 时自动扫描
    severity_filter: ["CRITICAL", "HIGH"]  # 仅报告 Critical + High
    
  # 镜像签名验证
  image_verification:
    enabled: true
    type: "cosign"
    key_ref: "k8s://ops-ai/cosign-pub"  # 公钥位置
    rekor_url: "https://rekor.sigstore.dev"
    require_signature: false            # 是否强制要求签名（渐进式推进）
    
  # 准入策略预检
  admission_pre_check:
    enabled: true
    check_kyverno: true
    check_gatekeeper: true
    auto_fix_suggestions: true          # 自动生成修复建议
    
  # SBOM
  sbom:
    enabled: true
    format: "spdx-json"                 # spdx-json | cyclonedx-json
```

### 45.10 System Prompt 补充

```
## 供应链安全知识

当运维询问镜像安全或准入策略相关问题时：

1. **镜像漏洞扫描**：
   - 诊断 Pod 时自动扫描镜像 CVE
   - 仅报告 Critical + High 级别漏洞（可配置）
   - 提供修复版本建议

2. **镜像签名验证**：
   - 推荐运维优先使用已签名镜像
   - 未签名镜像在 TUI 中显示 ⚠️ 警告

3. **准入策略预检**：
   - apply 操作前自动检查 Kyverno/Gatekeeper 策略
   - 如果被拒绝，提供具体修复建议
   - 可以自动修复常见标签/注解缺失

4. **SBOM**：
   - Agent 自身镜像构建时生成 SBOM
   - 可通过 `ops-ai sbom get` 查询
```

---

## 46. 混沌工程与故障注入集成（v1.9 新增）

### 46.1 问题场景

v1.8 设计了大量故障诊断能力（节点诊断 §9.4、网络诊断 §38、依赖诊断 §37），但**完全没有混沌工程和故障注入**：

- 运维不只是"故障发生了去修"，还需要"主动验证系统韧性"
- ChaosMesh/Litmus 是 K8s 生态主流混沌工具，Agent 应该能辅助设计和执行混沌实验
- 故障演练是 SRE 的日常工作，Agent 可以帮助生成混沌实验 YAML + 分析实验结果
- Agent 的告警自动修复闭环（§26）需要通过混沌实验来验证有效性

### 46.2 设计目标

- ChaosMesh/Litmus 集成：作为 L3 工具，支持创建/删除混沌实验
- 故障演练 Runbook：根据服务拓扑自动生成混沌实验建议
- 混沌实验结果分析：实验结束后自动收集指标变化
- 与告警自动修复联动：混沌实验触发告警 → Agent 自动修复 → 验证修复有效性

### 46.3 Go 接口定义

```go
// ChaosEngineeringManager 混沌工程管理器
type ChaosEngineeringManager struct {
    k8sClient      kubernetes.Interface
    chaosClient    chaosmesh.Interface    // ChaosMesh 客户端
    litmusClient   litmus.Interface       // Litmus 客户端
    config         ChaosConfig
}

// ChaosExperiment 混沌实验
type ChaosExperiment struct {
    Name        string
    Namespace   string
    Type        ChaosType
    Target      ChaosTarget
    Duration    time.Duration
    Spec        map[string]interface{}
    SafetyGuards []SafetyGuard
}

type ChaosType string

const (
    ChaosTypePodKill       ChaosType = "pod-kill"
    ChaosTypeNetworkDelay  ChaosType = "network-delay"
    ChaosTypeNetworkLoss   ChaosType = "network-loss"
    ChaosTypeCPUStress     ChaosType = "cpu-stress"
    ChaosTypeMemoryStress  ChaosType = "memory-stress"
    ChaosTypeDNSFault      ChaosType = "dns-fault"
    ChaosTypeIOStress      ChaosType = "io-stress"
)

type ChaosTarget struct {
    Kind      string  // Deployment / StatefulSet / Pod
    Name      string
    Namespace string
    Selector  map[string]string
    Percent   int     // 影响百分比（默认 50%）
}

type SafetyGuard struct {
    Type      string  // pdb-check / hpa-check / metric-threshold
    Condition string
    Action    string  // abort / pause / continue
}

// ChaosRunbookGenerator 混沌实验 Runbook 生成器
type ChaosRunbookGenerator struct {
    k8sClient kubernetes.Interface
}

type ChaosRunbook struct {
    ServiceName    string
    Topology       ServiceTopology
    Experiments    []ChaosExperiment
    MetricsBaseline map[string]float64
    SuccessCriteria []SuccessCriterion
}

type ServiceTopology struct {
    Deployments   []string
    Dependencies  []string  // 依赖服务
    IngressPaths  []string
    Databases     []string
}

type SuccessCriterion struct {
    Metric    string  // error-rate / latency-p99 / availability
    Threshold float64
    Duration  time.Duration
}

// ChaosResultAnalyzer 混沌实验结果分析器
type ChaosResultAnalyzer struct {
    promClient promv1.API
}

type ChaosAnalysisResult struct {
    ExperimentName string
    Baseline       MetricSnapshot
    During         MetricSnapshot
    Recovery       MetricSnapshot
    Passed         bool
    Issues         []string
    Recommendations []string
}
```

### 46.4 混沌实验创建（L3 工具）

```
ops-ai chaos create --type pod-kill --target deployment/payment-api --duration 5m
ops-ai chaos create --type network-delay --target deployment/payment-api --latency 100ms --duration 10m
ops-ai chaos list                           # 查看运行中的实验
ops-ai chaos delete <experiment-name>       # 终止实验
ops-ai chaos runbook generate --service payment-api   # 生成故障演练 Runbook
```

操作分级：
- **L0**：`chaos list`, `chaos runbook generate`（只读）
- **L3**：`chaos create`, `chaos delete`（故障注入）
  - 强制执行：影响面分析 + 自动快照 + 集群名确认 + **PDB 强制检查** + 可选双人审批

### 46.5 故障演练 Runbook 自动生成

```
运维: "为 payment-api 生成混沌实验方案"

Agent:
  1. 分析 payment-api 的拓扑
     ├─ Deployment: payment-api (replicas: 3)
     ├─ Dependencies: payment-db, redis-cache, order-service
     ├─ Ingress: /api/v1/payment
     └─ PDB: payment-api-pdb (minAvailable: 2)

  2. 生成混沌实验方案:

  ═══════════════════════════════════════════════════════
  🧪  payment-api 混沌实验方案
  ═══════════════════════════════════════════════════════

  实验 1: Pod Kill（验证自愈能力）
  ─────────────────────────────────────────────────────
  类型:      pod-kill
  目标:      deployment/payment-api
  影响:      随机杀死 1 个 Pod（33%，符合 PDB minAvailable: 2）
  持续时间:   5 分钟
  成功标准:   错误率 < 1%, P99 延迟 < 500ms

  实验 2: 网络延迟（验证超时处理）
  ─────────────────────────────────────────────────────
  类型:      network-delay
  目标:      deployment/payment-api → payment-db
  延迟:      200ms
  持续时间:   10 分钟
  成功标准:   数据库连接池无耗尽，降级策略生效

  实验 3: DNS 故障（验证缓存机制）
  ─────────────────────────────────────────────────────
  类型:      dns-fault
  目标:      payment namespace
  故障:      随机返回 NXDOMAIN
  持续时间:   5 分钟
  成功标准:   服务发现降级生效，本地 DNS 缓存正常工作

  [E] 执行全部实验  [S] 选择执行  [Q] 返回
```

### 46.6 混沌实验结果分析

```go
func (a *ChaosResultAnalyzer) Analyze(ctx context.Context, experiment ChaosExperiment) (*ChaosAnalysisResult, error) {
    result := &ChaosAnalysisResult{
        ExperimentName: experiment.Name,
    }
    
    // 1. 获取基线指标（实验前 5 分钟）
    result.Baseline = a.queryMetrics(ctx, experiment.Target, time.Now().Add(-10*time.Minute), time.Now().Add(-5*time.Minute))
    
    // 2. 获取实验期间指标
    result.During = a.queryMetrics(ctx, experiment.Target, experiment.StartTime, experiment.EndTime)
    
    // 3. 获取恢复后指标（实验后 5 分钟）
    result.Recovery = a.queryMetrics(ctx, experiment.Target, experiment.EndTime, experiment.EndTime.Add(5*time.Minute))
    
    // 4. 评估成功标准
    result.Passed = true
    for _, criterion := range experiment.SuccessCriteria {
        actual := result.Recovery.Get(criterion.Metric)
        if actual > criterion.Threshold {
            result.Passed = false
            result.Issues = append(result.Issues, 
                fmt.Sprintf("%s: %.2f > %.2f (阈值)", criterion.Metric, actual, criterion.Threshold))
        }
    }
    
    return result, nil
}
```

### 46.7 与告警自动修复联动

```
闭环验证流程:
1. Chaos 实验触发告警（如 pod-kill 导致服务降级）
2. alertd 接收到告警，创建自动修复会话
3. Agent 执行诊断 + 修复（如 scale up replicas）
4. 修复完成后，ChaosResultAnalyzer 验证：
   - 指标是否恢复到基线？
   - 修复措施是否有效？
5. 生成验证报告，更新 Runbook RAG（§54 反馈闭环）
```

### 46.8 配置项

```yaml
# ~/.ops-ai/config.yaml
chaos_engineering:
  enabled: true
  
  # 混沌工具选择
  provider: "chaos-mesh"                # chaos-mesh | litmus
  chaos_mesh:
    namespace: "chaos-testing"
    
  # 安全护栏（默认开启）
  safety_guards:
    pdb_check: true                     # 实验前检查 PDB
    max_duration: "30m"                 # 单次实验最大时长
    max_affected_percent: 50            # 最多影响 50% Pod
    forbidden_namespaces: ["kube-system", "monitoring", "velero"]
    
  # 结果分析
  result_analysis:
    prometheus_url: "http://prometheus:9090"
    metrics:
      - "error_rate"
      - "latency_p99"
      - "availability"
    baseline_duration: "5m"
    recovery_duration: "5m"
```

### 46.9 System Prompt 补充

```
## 混沌工程知识

当运维询问混沌实验或韧性验证相关问题时：

1. **实验安全**：
   - 所有混沌实验是 L3 操作，需要完整确认流程
   - 实验前强制检查 PDB，确保不会影响可用性
   - 禁止在 kube-system、monitoring 等核心 namespace 执行

2. **自动生成方案**：
   - 根据服务拓扑自动生成 3-5 个混沌实验
   - 每个实验包含明确的成功标准

3. **结果分析**：
   - 实验结束后自动对比基线/实验期间/恢复后指标
   - 如果未通过，分析根因并提供改进建议

4. **与自动修复联动**：
   - 混沌实验可以验证自动修复的有效性
   - 实验结果会反馈到 Runbook RAG 中
```

---

## 47. 事件全生命周期管理（v1.9 新增）

### 47.1 问题场景

v1.8 设计了告警触发（§26）和 on-call 接力（§23），但**缺少完整的事件生命周期管理**——严重度分级、war room 协作、postmortem 自动化：

- P0 事故发生时，需要快速创建 incident channel（Slack/飞书）、通知 stakeholders、建立 timeline
- 事故复盘需要完整的 timeline（谁在什么时间做了什么操作），Agent 有审计日志但没自动生成 timeline 的能力
- Postmortem 文档需要 root cause + action items + lessons learned，Agent 有 root cause 分析能力但没有结构化输出
- 严重度分级影响操作权限：P0 事故时可能需要临时提升权限（break-glass），但文档没有设计

### 47.2 设计目标

- `ops-ai incident create --sev P0`：自动创建事件上下文
- 从审计日志自动提取关键事件时间线
- War room 集成：自动在 Slack/飞书创建频道
- Postmortem 自动起草
- Break-glass 程序（详细见 §57）

### 47.3 Go 接口定义

```go
// IncidentManager 事件管理器
type IncidentManager struct {
    store         IncidentStore
    notifier      IncidentNotifier       // Slack/飞书通知
    timelineGen   *TimelineGenerator
    postmortemGen *PostmortemGenerator
}

type Incident struct {
    ID            string
    Severity      IncidentSeverity       // P0 | P1 | P2 | P3 | P4
    Title         string
    Status        IncidentStatus         // open | acknowledged | resolved | closed
    StartedAt     time.Time
    AcknowledgedAt *time.Time
    ResolvedAt    *time.Time
    Commander     string                 # 事件指挥官
    Responders    []string
    Timeline      []TimelineEvent
    AffectedServices []string
    RootCause     string
    ActionItems   []ActionItem
    SlackChannel  string
    SessionID     string                 # 关联的 Agent 会话
}

type IncidentSeverity string

const (
    IncidentSeverityP0 IncidentSeverity = "P0"
    IncidentSeverityP1 IncidentSeverity = "P1"
    IncidentSeverityP2 IncidentSeverity = "P2"
    IncidentSeverityP3 IncidentSeverity = "P3"
    IncidentSeverityP4 IncidentSeverity = "P4"
)

type IncidentStatus string

const (
    IncidentStatusOpen          IncidentStatus = "open"
    IncidentStatusAcknowledged  IncidentStatus = "acknowledged"
    IncidentStatusResolved      IncidentStatus = "resolved"
    IncidentStatusClosed        IncidentStatus = "closed"
)

type TimelineEvent struct {
    Timestamp time.Time
    Actor     string
    Action    string
    Detail    string
    Source    string  // audit-log | alert | user | agent
}

// TimelineGenerator 时间线生成器
type TimelineGenerator struct {
    auditStore AuditStore
}

// PostmortemGenerator 复盘文档生成器
type PostmortemGenerator struct {
    llmClient LLMClient
}

type Postmortem struct {
    IncidentID    string
    Summary       string
    Timeline      []TimelineEvent
    RootCause     string
    Impact        ImpactAnalysis
    ActionItems   []ActionItem
    LessonsLearned []string
}

type ImpactAnalysis struct {
    Duration      time.Duration
    AffectedUsers int
    Services      []string
    DataLoss      bool
}

type ActionItem struct {
    Description string
    Owner       string
    DueDate     time.Time
    Priority    string
    Status      string
}
```

### 47.4 事件创建与管理命令

```
ops-ai incident create --sev P0 --title "payment-api 服务不可用"
ops-ai incident list                        # 查看事件列表
ops-ai incident status <incident-id>        # 查看事件详情
ops-ai incident resolve <incident-id> --root-cause "数据库连接池耗尽"
ops-ai incident postmortem <incident-id>    # 生成复盘文档
ops-ai incident timeline <incident-id>      # 查看时间线
```

### 47.5 事件创建流程

```
运维: "ops-ai incident create --sev P0 --title 'payment-api 服务不可用'"

Agent:
  1. 创建事件记录
  2. 关联当前会话（sessionID）
  3. 自动提取当前上下文：
     - 正在排查的 Pod: payment-api-xxx
     - 相关告警: HighErrorRate-payment-api
     - on-call 人员: alice, bob
  4. 创建 War Room（如配置了 Slack/飞书）
  5. 通知 stakeholders

  ═══════════════════════════════════════════════════════
  🚨  事件已创建
  ═══════════════════════════════════════════════════════

  事件 ID:    INC-20240625-001
  严重度:      P0 🔴
  标题:       payment-api 服务不可用
  状态:       Open
  指挥官:     alice（当前用户）

  War Room
  ─────────────────────────────────────────────────────
  Slack: #incident-20240625-001
  已邀请: @alice, @bob, @payment-team

  自动关联
  ─────────────────────────────────────────────────────
  会话:       sess-abc123
  告警:       HighErrorRate-payment-api
  服务:       payment-api, payment-db

  [A] 确认事件  [U] 更新状态  [R] 请求增援  [Q] 继续排查
```

### 47.6 时间线自动生成

```go
func (g *TimelineGenerator) Generate(ctx context.Context, sessionID string) ([]TimelineEvent, error) {
    // 1. 从审计日志提取操作事件
    auditEntries, err := g.auditStore.Query(ctx, AuditQueryRequest{
        SessionID: sessionID,
    })
    if err != nil {
        return nil, err
    }
    
    var timeline []TimelineEvent
    
    // 2. 转换审计日志为时间线事件
    for _, entry := range auditEntries {
        timeline = append(timeline, TimelineEvent{
            Timestamp: entry.Timestamp,
            Actor:     entry.UserID,
            Action:    fmt.Sprintf("%s %s/%s", entry.Action, entry.ResourceKind, entry.ResourceName),
            Detail:    entry.Detail,
            Source:    "audit-log",
        })
    }
    
    // 3. 合并告警事件（从 alertd）
    alerts := g.alertStore.GetBySession(sessionID)
    for _, alert := range alerts {
        timeline = append(timeline, TimelineEvent{
            Timestamp: alert.FiredAt,
            Actor:     "alertd",
            Action:    fmt.Sprintf("告警触发: %s", alert.Name),
            Detail:    alert.Description,
            Source:    "alert",
        })
    }
    
    // 4. 按时间排序
    sort.Slice(timeline, func(i, j int) bool {
        return timeline[i].Timestamp.Before(timeline[j].Timestamp)
    })
    
    return timeline, nil
}
```

### 47.7 Postmortem 自动起草

```go
func (g *PostmortemGenerator) Generate(ctx context.Context, incident *Incident) (*Postmortem, error) {
    // 1. 获取时间线
    timeline, err := g.timelineGen.Generate(ctx, incident.SessionID)
    if err != nil {
        return nil, err
    }
    
    // 2. 获取会话历史（root cause 分析）
    sessionHistory := g.sessionStore.Get(incident.SessionID)
    
    // 3. 使用 LLM 生成结构化复盘文档
    prompt := fmt.Sprintf(`
基于以下事件信息，生成一份 postmortem 文档：

事件: %s
严重度: %s
持续时间: %s
影响服务: %v

时间线:
%s

排查过程:
%s

请生成包含以下部分的 Markdown 文档：
1. 摘要
2. 时间线
3. 影响分析
4. 根因
5. 行动项（带负责人和截止日期）
6. 经验教训
`, incident.Title, incident.Severity, incident.Duration(), incident.AffectedServices,
       formatTimeline(timeline), sessionHistory.Summary)
    
    postmortemMD, err := g.llmClient.Complete(ctx, prompt)
    if err != nil {
        return nil, err
    }
    
    // 4. 解析结构化数据
    return g.parsePostmortem(postmortemMD), nil
}
```

### 47.8 War Room 集成

```yaml
# 配置示例
incident_management:
  war_room:
    type: "slack"                       # slack | feishu
    slack:
      webhook_url: "https://hooks.slack.com/services/..."
      channel_prefix: "incident-"
      auto_invite:
        - "@on-call"
        - "@{team}-team"
```

### 47.9 配置项

```yaml
# ~/.ops-ai/config.yaml
incident_management:
  enabled: true
  
  # War Room 集成
  war_room:
    type: "slack"
    slack:
      webhook_url: ""
      bot_token: ""
      channel_prefix: "incident-"
      auto_invite_oncall: true
      
  # 严重度定义（影响操作权限）
  severity:
    P0:
      color: "#FF0000"
      auto_break_glass: true            # P0 自动启用 break-glass
      notification_channels: ["pagerduty", "slack"]
    P1:
      color: "#FF6600"
      auto_break_glass: false
      notification_channels: ["slack"]
    P2:
      color: "#FFCC00"
    P3:
      color: "#0066FF"
    P4:
      color: "#999999"
      
  # Postmortem
  postmortem:
    auto_generate: true                 # 事件关闭后自动生成
    template: "default"
    required_approvers: 1               # 复盘文档需要 1 人审批
    due_days_after_close: 7             # 事件关闭后 7 天内完成复盘
```

### 47.10 System Prompt 补充

```
## 事件全生命周期管理知识

当运维处理生产事故时：

1. **事件创建**：
   - P0/P1 事故应立即创建事件: `ops-ai incident create --sev P0`
   - 事件会自动关联当前会话和告警
   - P0 事件自动启用 break-glass 模式（§57）

2. **时间线**：
   - 事件时间线自动从审计日志和告警生成
   - 可以在排查过程中随时查看: `ops-ai incident timeline`

3. **War Room**：
   - P0/P1 事件自动创建 Slack/飞书频道
   - Agent 的操作会实时推送到 War Room

4. **Postmortem**：
   - 事件关闭后自动生成复盘文档草稿
   - 包含完整时间线、根因分析、行动项
   - 需要在 7 天内完成并审批

5. **严重度定义**：
   - P0: 服务完全不可用，影响所有用户，需要立即响应
   - P1: 核心功能受损，影响大量用户，需要 30 分钟内响应
   - P2: 部分功能受损，有 workaround，需要 2 小时内响应
   - P3: 轻微问题，不影响核心功能
   - P4: 观察项，无需立即行动
```

---

## 48. K8s 集群升级辅助（v1.9 新增）

### 48.1 问题场景

v1.8 有 API 版本废弃检测（§11.1 规则 4，known deprecated mappings），但**没有集群升级的完整辅助流程**：

- K8s 每年 3 个小版本，升级是高危操作：API 废弃 → 工作负载 break → 控制平面组件版本不兼容
- 升级前需要检查：所有 API 资源版本、已废弃 API 使用情况、addon 兼容性、节点 OS 兼容性
- 升级中需要：逐节点 drain+升级+验证，PDB 保护，滚动控制
- 升级后需要：API 可用性验证、关键工作负载健康检查、回滚预案

### 48.2 设计目标

- `ops-ai upgrade preflight --from 1.28 --to 1.30`：全面升级前检查
- 节点滚动升级编排
- 升级后验证

### 48.3 Go 接口定义

```go
// ClusterUpgradeManager 集群升级管理器
type ClusterUpgradeManager struct {
    k8sClient     kubernetes.Interface
    checker       *UpgradePreflightChecker
    orchestrator  *NodeUpgradeOrchestrator
    validator     *PostUpgradeValidator
}

// UpgradePreflightChecker 升级前检查器
type UpgradePreflightChecker struct{}

type PreflightResult struct {
    FromVersion       string
    ToVersion         string
    DeprecatedAPIs    []DeprecatedAPIUsage
    AddonCompat       []AddonCompatibility
    NodeOSCompat      []NodeOSCompatibility
    ResourceHealth    []ResourceHealthCheck
    RiskLevel         string  // low | medium | high | critical
    Blockers          []string
    Warnings          []string
}

type DeprecatedAPIUsage struct {
    APIVersion     string
    Kind           string
    Namespace      string
    Name           string
    Replacement    string
    Risk           string
}

type AddonCompatibility struct {
    Addon       string    // Calico / Cilium / Flannel / Ingress-NGINX
    CurrentVersion string
    RequiredVersion string
    Compatible  bool
    Notes       string
}

type NodeOSCompatibility struct {
    NodeName    string
    OSImage     string
    KernelVersion string
    Compatible  bool
    Notes       string
}

// NodeUpgradeOrchestrator 节点升级编排器
type NodeUpgradeOrchestrator struct {
    k8sClient kubernetes.Interface
}

type NodeUpgradePlan struct {
    Nodes           []NodeUpgradeStep
    TotalDuration   time.Duration
    PDBProtected    bool
    BatchSize       int
}

type NodeUpgradeStep struct {
    NodeName      string
    CurrentVersion string
    TargetVersion  string
    Steps         []UpgradeStep
    EstimatedDuration time.Duration
}

type UpgradeStep struct {
    Order       int
    Action      string  // cordon | drain | upgrade | uncordon | verify
    Command     string
    Verification string
}
```

### 48.4 升级前检查命令

```
ops-ai upgrade preflight --from 1.28 --to 1.30
```

TUI 展示：

```
  ═══════════════════════════════════════════════════════
  ⬆️   集群升级预检报告: 1.28 → 1.30
  ═══════════════════════════════════════════════════════

  总体风险: 🟠 中等风险（2 blockers, 5 warnings）

  🔴 Blockers（必须解决）
  ─────────────────────────────────────────────────────
  1. 发现 3 个已废弃 API 仍在使用:
     - autoscaling/v2beta2 HorizontalPodAutoscaler (payment/hpa-api)
     - policy/v1beta1 PodDisruptionBudget (default/pdb-app)
     - flowcontrol.apiserver.k8s.io/v1beta1 FlowSchema
     
  2. addon 版本不兼容:
     - Calico v3.25.0 → 需要 ≥ v3.26.0（1.30 兼容）

  🟡 Warnings（建议解决）
  ─────────────────────────────────────────────────────
  1. 2 个节点内核版本 < 5.4（建议升级节点 OS）
  2. 未配置 PodDisruptionBudget 的 Deployment: cache-service
  3. etcd 备份未验证（上次备份: 3 天前）

  兼容性矩阵
  ─────────────────────────────────────────────────────
  Addon              当前版本    要求版本     状态
  ─────────────────────────────────────────────────────
  Calico             v3.25.0     ≥ v3.26.0   ❌
  Ingress-NGINX      v1.8.0      ≥ v1.8.0    ✅
  metrics-server     v0.6.4      ≥ v0.6.0    ✅
  CSI driver         v1.9.0      ≥ v1.9.0    ✅

  [F] 修复 Blockers  [C] 继续升级（不推荐）  [Q] 返回
```

### 48.5 废弃 API 扫描

```go
func (c *UpgradePreflightChecker) ScanDeprecatedAPIs(ctx context.Context, targetVersion string) ([]DeprecatedAPIUsage, error) {
    // 基于 kubent/kube-no-trouble 逻辑
    deprecatedMappings := map[string]string{
        "autoscaling/v2beta2": "autoscaling/v2",
        "autoscaling/v2beta1": "autoscaling/v2",
        "policy/v1beta1":      "policy/v1",
        "flowcontrol.apiserver.k8s.io/v1beta1": "flowcontrol.apiserver.k8s.io/v1",
        // ... 更多映射
    }
    
    var results []DeprecatedAPIUsage
    
    // 扫描所有 namespace 的资源
    for oldAPI, newAPI := range deprecatedMappings {
        // 使用 discovery API 获取所有使用该 API 版本的资源
        usages := c.findAPIUsages(ctx, oldAPI)
        for _, usage := range usages {
            results = append(results, DeprecatedAPIUsage{
                APIVersion:  oldAPI,
                Kind:        usage.Kind,
                Namespace:   usage.Namespace,
                Name:        usage.Name,
                Replacement: newAPI,
                Risk:        "high",
            })
        }
    }
    
    return results, nil
}
```

### 48.6 节点滚动升级编排

```
运维: "开始节点滚动升级"

Agent:
  生成节点升级计划:

  ═══════════════════════════════════════════════════════
  ⬆️   节点滚动升级计划
  ═══════════════════════════════════════════════════════

  批次: 3 个节点，每批 1 个节点
  预估总时长: 45 分钟
  PDB 保护: ✅

  节点: worker-01
  ─────────────────────────────────────────────────────
  1. kubectl cordon worker-01
  2. kubectl drain worker-01 --ignore-daemonsets --delete-emptydir-data
     等待: Pod 迁移完成（最多 5 分钟）
  3. [MANUAL] 升级节点 OS/K8s 版本
  4. kubectl uncordon worker-01
  5. 验证: 节点 Ready, Pod 正常运行

  [S] 开始第一批  [P] 查看全部计划  [Q] 返回
```

### 48.7 升级后验证

```go
func (v *PostUpgradeValidator) Validate(ctx context.Context) (*ValidationResult, error) {
    checks := []ValidationCheck{
        {Name: "API Server 可用性", Check: v.checkAPIServer},
        {Name: "核心 CRD 健康", Check: v.checkCRDs},
        {Name: "核心工作负载", Check: v.checkCoreWorkloads},
        {Name: "节点状态", Check: v.checkNodes},
        {Name: "网络连通性", Check: v.checkNetwork},
    }
    
    result := &ValidationResult{}
    for _, check := range checks {
        ok, detail := check.Check(ctx)
        result.Checks = append(result.Checks, CheckResult{
            Name:   check.Name,
            Passed: ok,
            Detail: detail,
        })
        if !ok {
            result.Passed = false
        }
    }
    
    return result, nil
}
```

### 48.8 L0 命令扩展

```
# 集群升级（新增）
ops-ai upgrade preflight --from <ver> --to <ver>
ops-ai upgrade deprecated-apis              # 扫描已废弃 API
ops-ai upgrade plan                         # 生成升级计划
ops-ai upgrade validate                     # 升级后验证
ops-ai addon compat --version <k8s-ver>     # 检查 addon 兼容性
```

### 48.9 配置项

```yaml
# ~/.ops-ai/config.yaml
cluster_upgrade:
  enabled: true
  
  # 版本兼容性矩阵
  addon_compatibility:
    calico:
      "1.30": ">= v3.26.0"
      "1.29": ">= v3.25.0"
    cilium:
      "1.30": ">= v1.14.0"
    ingress_nginx:
      "1.30": ">= v1.8.0"
      
  # 升级策略
  upgrade_strategy:
    batch_size: 1                       # 每批升级节点数
    drain_timeout: "5m"
    pod_eviction_timeout: "2m"
    verify_interval: "30s"
    verify_count: 10                    # 验证 10 次
    
  # 回滚预案
  rollback:
    auto_snapshot_before_upgrade: true
    etcd_backup_required: true
```

### 48.10 System Prompt 补充

```
## 集群升级知识

当运维询问集群升级相关问题时：

1. **升级前**：
   - 必须先执行 preflight 检查
   - 解决所有 blockers 后才能继续
   - 确认 addon 兼容性矩阵
   - 确保 etcd 和 Velero 备份已就绪

2. **废弃 API**：
   - 扫描所有 namespace 中使用的废弃 API
   - 提供迁移到新 API 版本的建议
   - 可以在升级前自动 apply 修复

3. **节点升级**：
   - 逐节点 drain → 升级 → uncordon
   - 每批升级后验证 Pod 健康
   - PDB 保护：如果 drain 被 PDB 阻塞，提示运维处理

4. **升级后验证**：
   - 检查 API Server、CRD、核心工作负载、节点、网络
   - 任何检查失败都建议回滚
```

---

## 49. CSI/存储运维深度（v1.9 新增）

### 49.1 问题场景

v1.8 在 PVC 快照方面明确承认"数据不会自动恢复"（§7），但**没有进一步设计 CSI 集成来弥补这个缺口**：

- 存储问题是 K8s 运维的高频痛点：PVC Pending、Volume 挂载失败、I/O 饱和、快照恢复
- CSI 驱动问题：driver not found、snapshot controller 未部署、StorageClass 参数错误
- 有状态服务运维：StatefulSet + PVC 的组合操作（扩容、迁移、快照恢复）极其复杂
- 存储性能诊断：IOPS/吞吐瓶颈、延迟突增，需要 CSI metrics 或节点级诊断

### 49.2 设计目标

- CSI 驱动健康检查
- VolumeSnapshot 操作支持（L3）
- PVC Pending 原因分析
- 存储性能诊断
- 有状态服务运维 Runbook

### 49.3 Go 接口定义

```go
// StorageOpsManager 存储运维管理器
type StorageOpsManager struct {
    k8sClient      kubernetes.Interface
    snapshotClient snapshotversioned.Interface
    csiClient      csiv1.Interface
    config         StorageConfig
}

// CSIDriverChecker CSI 驱动检查器
type CSIDriverChecker struct {
    k8sClient kubernetes.Interface
}

type CSIDriverStatus struct {
    Name           string
    Version        string
    NodePluginReady bool
    ControllerReady bool
    SnapshotReady   bool  // snapshot controller 是否部署
    Issues         []string
}

// PVCDiagnoser PVC 诊断器
type PVCDiagnoser struct {
    k8sClient kubernetes.Interface
}

type PVCDiagnosisResult struct {
    PVCName       string
    Namespace     string
    Phase         string
    Reason        string  // Pending 原因
    Details       map[string]string
    Suggestions   []string
}

// StoragePerformanceChecker 存储性能检查器
type StoragePerformanceChecker struct {
    promClient promv1.API
}

type StoragePerformanceResult struct {
    PVCName     string
    ReadIOPS    float64
    WriteIOPS   float64
    ReadThroughput float64  // MB/s
    WriteThroughput float64 // MB/s
    LatencyP50  float64    // ms
    LatencyP99  float64    // ms
    Issues      []string
}

// StatefulSetOpsHelper 有状态服务运维辅助
type StatefulSetOpsHelper struct {
    k8sClient      kubernetes.Interface
    snapshotClient snapshotversioned.Interface
}
```

### 49.4 CSI 驱动健康检查

```
ops-ai storage csi-status
```

```
  ═══════════════════════════════════════════════════════
  💾  CSI 驱动状态
  ═══════════════════════════════════════════════════════

  驱动: ebs.csi.aws.com
  版本: v1.24.0
  
  组件状态
  ─────────────────────────────────────────────────────
  Controller Plugin:   ✅ Running (2/2 replicas)
  Node Plugin:         ✅ Running (5/5 nodes)
  Snapshot Controller: ⚠️  未部署（无法创建 VolumeSnapshot）
  
  能力
  ─────────────────────────────────────────────────────
  创建/删除卷:         ✅
  卷扩容:              ✅
  快照:                ❌（缺少 snapshot controller）
  克隆:                ✅

  建议
  ─────────────────────────────────────────────────────
  部署 snapshot controller 以启用卷快照功能：
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/...
```

### 49.5 PVC Pending 原因分析

```go
func (d *PVCDiagnoser) DiagnosePendingPVC(ctx context.Context, pvcName, namespace string) (*PVCDiagnosisResult, error) {
    pvc, err := d.k8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }
    
    result := &PVCDiagnosisResult{
        PVCName:   pvcName,
        Namespace: namespace,
        Phase:     string(pvc.Status.Phase),
    }
    
    // 分析 Pending 原因
    if pvc.Status.Phase != corev1.ClaimPending {
        result.Reason = "PVC 已绑定"
        return result, nil
    }
    
    // 1. StorageClass 不存在
    if pvc.Spec.StorageClassName != nil {
        _, err := d.k8sClient.StorageV1().StorageClasses().Get(ctx, *pvc.Spec.StorageClassName, metav1.GetOptions{})
        if err != nil {
            result.Reason = "StorageClass 不存在"
            result.Details["storage_class"] = *pvc.Spec.StorageClassName
            result.Suggestions = append(result.Suggestions, 
                fmt.Sprintf("创建 StorageClass %s 或修改 PVC 使用存在的 StorageClass", *pvc.Spec.StorageClassName))
            return result, nil
        }
    }
    
    // 2. 容量不足
    // 检查 StorageClass 对应的 Provisioner 是否有足够存储
    
    // 3. 拓扑不匹配
    // 检查 volumeBindingMode: WaitForFirstConsumer 时 Pod 调度约束
    
    // 4. WFFC 驱动等待 Pod
    sc, _ := d.k8sClient.StorageV1().StorageClasses().Get(ctx, *pvc.Spec.StorageClassName, metav1.GetOptions{})
    if sc != nil && sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
        result.Reason = "WaitForFirstConsumer: 等待 Pod 调度"
        result.Details["volume_binding_mode"] = "WaitForFirstConsumer"
        result.Suggestions = append(result.Suggestions, "确保有 Pending Pod 需要该 PVC")
        return result, nil
    }
    
    return result, nil
}
```

### 49.6 VolumeSnapshot 操作（L3）

已在 §42.6 中定义，此处补充 snapshot restore：

```go
func (m *StorageOpsManager) RestoreFromSnapshot(ctx context.Context, snapshotName, namespace, newPVCName string) (*corev1.PersistentVolumeClaim, error) {
    // 1. 获取 VolumeSnapshot
    snapshot, err := m.snapshotClient.SnapshotV1().VolumeSnapshots(namespace).Get(ctx, snapshotName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }
    
    // 2. 检查 snapshot ready
    if snapshot.Status == nil || snapshot.Status.ReadyToUse == nil || !*snapshot.Status.ReadyToUse {
        return nil, fmt.Errorf("snapshot %s 尚未就绪", snapshotName)
    }
    
    // 3. 从 snapshot 创建 PVC
    pvc := &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      newPVCName,
            Namespace: namespace,
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            StorageClassName: snapshot.Spec.VolumeSnapshotClassName,
            DataSource: &corev1.TypedLocalObjectReference{
                APIGroup: strPtr("snapshot.storage.k8s.io"),
                Kind:     "VolumeSnapshot",
                Name:     snapshotName,
            },
            AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
            Resources: corev1.VolumeResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: *snapshot.Status.RestoreSize,
                },
            },
        },
    }
    
    return m.k8sClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}
```

### 49.7 存储性能诊断

```
ops-ai storage perf --pvc payment-data --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  💾  存储性能诊断
  ═══════════════════════════════════════════════════════

  PVC: payment-data
  Pod: payment-api-7d8f9
  
  IOPS
  ─────────────────────────────────────────────────────
  读 IOPS:    1,234    🟢
  写 IOPS:    567      🟢
  
  吞吐
  ─────────────────────────────────────────────────────
  读吞吐:     45.2 MB/s  🟢
  写吞吐:     12.3 MB/s  🟢
  
  延迟
  ─────────────────────────────────────────────────────
  P50:        2.1 ms   🟢
  P99:        45.6 ms  🟡  偏高
  
  问题
  ─────────────────────────────────────────────────────
  P99 延迟 45.6ms 高于阈值 20ms。
  可能原因:
  1. 存储层 I/O 饱和（节点上其他 Pod 争用）
  2. 存储类型不适合当前 workload（考虑 gp3 → io2）
  
  [D] 查看详细指标  [Q] 返回
```

### 49.8 有状态服务运维 Runbook

```
运维: "扩容 StatefulSet payment-db"

Agent:
  1. 分析 StatefulSet + PVC 状态
  2. 生成扩容计划:

  ═══════════════════════════════════════════════════════
  📋  StatefulSet 扩容计划: payment-db
  ═══════════════════════════════════════════════════════

  当前状态: 3 replicas
  目标状态: 5 replicas

  StatefulSet 扩容特性
  ─────────────────────────────────────────────────────
  - 按序号顺序创建: payment-db-3, payment-db-4
  - 每个新 Pod 会自动创建 PVC: data-payment-db-3, data-payment-db-4
  - 新 Pod 必须 Running 后才创建下一个

  预估时间: 10 分钟

  [S] 开始扩容  [C] 取消
```

### 49.9 L0 命令扩展

```
# 存储运维（新增 L0）
ops-ai storage csi-status                  # CSI 驱动状态
ops-ai storage pvc diagnose <pvc> [-n <ns>] # PVC 诊断
ops-ai storage snapshot get                # 查看快照列表
ops-ai storage perf --pvc <pvc> [-n <ns>]  # 存储性能诊断
ops-ai storage sc list                     # StorageClass 列表
```

### 49.10 配置项

```yaml
# ~/.ops-ai/config.yaml
storage_ops:
  enabled: true
  
  csi:
    snapshot_controller_required: false   # 是否要求 snapshot controller
    
  performance:
    prometheus_metrics:
      - "kubelet_volume_stats_capacity_bytes"
      - "kubelet_volume_stats_used_bytes"
      - "storage_operation_duration_seconds"
    latency_threshold_p99: 20             # ms
    iops_threshold_warning: 1000
    
  statefulset:
    default_termination_grace_period: "60s"
    pvc_resize_timeout: "5m"
```

### 49.11 System Prompt 补充

```
## 存储运维知识

当运维询问存储相关问题时：

1. **PVC Pending**：
   - 常见原因：StorageClass 不存在、容量不足、拓扑不匹配、WFFC 等待 Pod
   - 使用 `ops-ai storage pvc diagnose` 自动分析

2. **VolumeSnapshot**：
   - 创建/恢复是 L3 操作，需要完整确认
   - 恢复时从 snapshot 创建新 PVC，不会覆盖原数据
   - 需要 CSI 驱动和 snapshot controller 支持

3. **有状态服务**：
   - StatefulSet 扩容是按序号的，比 Deployment 慢
   - 缩容是从高序号开始，数据不会自动删除
   - PVC 扩容需要存储类支持 expand 能力

4. **性能诊断**：
   - 通过 Prometheus 或节点指标获取 IOPS/吞吐/延迟
   - P99 延迟高通常意味着存储层饱和或存储类型不匹配
```

---

## 50. 容量规划与资源推荐引擎（v1.9 新增）

### 50.1 问题场景

v1.8 有 HPA 分析（§4 场景表）和资源使用趋势检查，但这是**反应式**的——"HPA 到了 max，帮你扩容"。缺少**主动式**的容量规划和资源推荐：

- 资源浪费：Pod request 过高导致集群资源利用率低（平均 30-50% 浪费是常态）
- 资源不足：Pod limit 过低导致 OOMKilled，request 过低导致调度到已经满载的节点
- 容量预测：基于历史趋势预测何时需要扩容节点池
- Right-sizing 建议：基于实际使用量推荐 request/limit 调整

### 50.2 设计目标

- Right-sizing 推荐引擎：基于 Prometheus 历史数据分析
- VPA 集成：识别已部署 VPA 的命名空间，推荐 VPA 配置
- 节点池容量预测：基于 Pod 调度趋势 + 节点资源水位
- 资源浪费检测：识别 request 远大于实际使用的 Pod

### 50.3 Go 接口定义

```go
// CapacityPlanningManager 容量规划管理器
type CapacityPlanningManager struct {
    promClient    promv1.API
    k8sClient     kubernetes.Interface
    recommender   *ResourceRecommender
    predictor     *CapacityPredictor
}

// ResourceRecommender 资源推荐引擎
type ResourceRecommender struct {
    promClient promv1.API
}

type ResourceRecommendation struct {
    Namespace     string
    Deployment    string
    Container     string
    CurrentRequest corev1.ResourceList
    CurrentLimit   corev1.ResourceList
    RecommendedRequest corev1.ResourceList
    RecommendedLimit   corev1.ResourceList
    Confidence    float64  // 0-1
    Reason        string
    Savings       *ResourceSavings
}

type ResourceSavings struct {
    CPUCores    float64
    MemoryGB    float64
    CostPerMonth float64  // 估算节省
}

// CapacityPredictor 容量预测器
type CapacityPredictor struct {
    promClient promv1.API
}

type CapacityPrediction struct {
    NodePool      string
    CurrentNodes  int
    CurrentUtilization float64
    PredictedNodes int      // 30 天后预测
    PredictedUtilization float64
    RiskDate      *time.Time  // 何时会资源耗尽
    Recommendations []string
}

// VPARecommender VPA 推荐器
type VPARecommender struct {
    k8sClient kubernetes.Interface
}

type VPARecommendation struct {
    Namespace   string
    Deployment  string
    HasVPA      bool
    VPAConfig   *vpa.VerticalPodAutoscaler
    RecommendedMode string  // Off | Initial | Recreate | Auto
}
```

### 50.4 Right-sizing 推荐

```
ops-ai capacity recommend --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  📊  资源优化推荐 — payment namespace
  ═══════════════════════════════════════════════════════

  基于最近 7 天 Prometheus 数据

  过度申请（可缩减）
  ─────────────────────────────────────────────────────
  Deployment        容器       CPU        内存       月节省
  ─────────────────────────────────────────────────────
  payment-api       api        500→200m   1→512Mi    $45
  payment-worker    worker     1000→400m  2→1Gi      $89
  
  申请不足（建议增加）
  ─────────────────────────────────────────────────────
  Deployment        容器       CPU        内存       风险
  ─────────────────────────────────────────────────────
  cache-service     redis      100→200m   256→512Mi  OOM 风险

  总计潜在节省: $134/月

  [A] 应用推荐（L2，需确认）  [E] 导出报告  [Q] 返回
```

### 50.5 容量预测

```
ops-ai capacity predict
```

```
  ═══════════════════════════════════════════════════════
  📈  节点池容量预测
  ═══════════════════════════════════════════════════════

  节点池: worker-nodes
  
  当前状态
  ─────────────────────────────────────────────────────
  节点数:       10
  CPU 利用率:    72%
  内存利用率:    68%
  
  30 天预测
  ─────────────────────────────────────────────────────
  节点数:       14（+4）
  CPU 利用率:    58%
  内存利用率:    55%
  
  风险日期
  ─────────────────────────────────────────────────────
  CPU 耗尽预测:   2024-07-15（20 天后）
  内存耗尽预测:   2024-07-20（25 天后）

  建议
  ─────────────────────────────────────────────────────
  1. 在 7 月 10 日前扩容节点池至 14 节点
  2. 或优化 payment-worker 资源申请（可释放 2 节点容量）

  [Q] 返回
```

### 50.6 VPA 集成

```go
func (r *VPARecommender) RecommendVPA(ctx context.Context, namespace, deployment string) (*VPARecommendation, error) {
    // 1. 检查是否已有 VPA
    vpaList, err := r.k8sClient.AutoscalingV1().VerticalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
    
    // 2. 分析 Deployment 资源使用波动
    // 3. 推荐 VPA 模式和配置
    return &VPARecommendation{
        Namespace:   namespace,
        Deployment:  deployment,
        HasVPA:      len(vpaList.Items) > 0,
        RecommendedMode: "Auto",  // 或 "Recreate" 根据场景
    }, nil
}
```

### 50.7 配置项

```yaml
# ~/.ops-ai/config.yaml
capacity_planning:
  enabled: true
  
  # 推荐引擎
  recommender:
    history_window: "7d"                # 分析历史数据窗口
    percentile: 95                      # 基于 P95 使用量推荐
    safety_margin: 1.2                  # 安全系数（推荐值 = P95 * 1.2）
    min_confidence: 0.7                 # 最小置信度
    
  # 预测
  predictor:
    forecast_window: "30d"              # 预测未来 30 天
    growth_model: "linear"              # linear | exponential
    
  # 成本估算
  cost:
    cpu_cost_per_core_month: 10.0       # 美元
    memory_cost_per_gb_month: 5.0       # 美元
    
  # 浪费检测
  waste_detection:
    cpu_threshold: 0.3                  # request 使用率 < 30% 视为浪费
    memory_threshold: 0.3
```

### 50.8 System Prompt 补充

```
## 容量规划知识

当运维询问资源优化或容量相关问题时：

1. **Right-sizing**：
   - 基于 Prometheus 历史数据（默认 7 天）
   - 使用 P95 使用量 * 安全系数作为推荐值
   - 只推荐 confident > 0.7 的结果

2. **容量预测**：
   - 基于 Pod 创建趋势和节点资源水位
   - 预测资源耗尽日期
   - 提供扩容和优化两种建议

3. **VPA**：
   - 推荐 mode: Auto（自动调整，需要 Pod 重启）
   - 对于不允许重启的服务，推荐 Off 或 Initial

4. **成本**：
   - 资源节省估算基于配置的成本模型
   - 实际成本因云厂商和购买方式而异
```

---

## 51. 合规自动化（v1.9 新增）

### 51.1 问题场景

v1.8 有审计日志（§17）和 RBAC 清单（§16），但**没有合规扫描和合规报告自动化**：

- PCI-DSS 要求：网络隔离、加密传输、访问审计、密钥轮换——Agent 有部分能力但没有合规映射
- SOC 2 要求：变更管理、访问控制、监控告警——需要定期生成合规报告
- 等保 2.0（中国市场）：安全审计、入侵防范、数据完整性——需要控制项检查
- CIS Benchmark：K8s 安全配置基线，Agent 有部分检查（PSA、NetworkPolicy）但没有系统化

### 51.2 设计目标

- CIS Benchmark 扫描
- 合规映射报告
- RBAC 审计
- Pod Security Standards 合规检查
- 定期合规报告

### 51.3 Go 接口定义

```go
// ComplianceManager 合规管理器
type ComplianceManager struct {
    k8sClient     kubernetes.Interface
    cisChecker    *CISBenchmarkChecker
    rbacAuditor   *RBACAuditor
    pssChecker    *PSSChecker
    reportGen     *ComplianceReportGenerator
}

// CISBenchmarkChecker CIS 基线检查器
type CISBenchmarkChecker struct {
    k8sClient kubernetes.Interface
}

type CISCheckResult struct {
    ID          string
    Description string
    Severity    string  // critical | high | medium | low
    Passed      bool
    Evidence    string
    Remediation string
}

// RBACAuditor RBAC 审计器
type RBACAuditor struct {
    k8sClient kubernetes.Interface
}

type RBACAuditResult struct {
    Issues []RBACIssue
}

type RBACIssue struct {
    Type        string  // wildcard-permission | cluster-admin-abuse | unused-binding
    Resource    string
    Subject     string
    Severity    string
    Description string
    Remediation string
}

// PSSChecker Pod Security Standards 检查器
type PSSChecker struct {
    k8sClient kubernetes.Interface
}

type PSSResult struct {
    Namespace   string
    Level       string  // privileged | baseline | restricted
    Violations  []PSSViolation
}

type PSSViolation struct {
    PodName     string
    Policy      string
    Violation   string
    Severity    string
}

// ComplianceReportGenerator 合规报告生成器
type ComplianceReportGenerator struct{}

type ComplianceReport struct {
    GeneratedAt    time.Time
    Framework      string  // CIS | PCI-DSS | SOC2 | 等保2.0
    OverallScore   float64 // 0-100
    PassedChecks   int
    FailedChecks   int
    Results        []FrameworkCheckResult
}

type FrameworkCheckResult struct {
    ControlID     string
    ControlName   string
    Status        string  // pass | fail | partial
    Evidence      string
    AgentCapability string  // full | partial | manual
}
```

### 51.4 CIS Benchmark 扫描

```
ops-ai compliance scan --framework cis
ops-ai compliance scan --framework pci-dss
ops-ai compliance scan --framework soc2
ops-ai compliance scan --framework 等保2.0
```

```
  ═══════════════════════════════════════════════════════
  📋  CIS Kubernetes Benchmark v1.8 扫描结果
  ═══════════════════════════════════════════════════════

  总体评分: 78/100
  通过: 42  失败: 12  部分: 6

  🔴 Critical (2)
  ─────────────────────────────────────────────────────
  5.1.5 确保 default service account 未自动挂载
        状态: ❌ 失败
        证据: 12 个 Pod 自动挂载 default SA token
        修复: kubectl patch pod/... -p '{"spec":...}'
        
  5.1.6 确保 ServiceAccount token 只读挂载
        状态: ❌ 失败
        证据: 3 个 Pod 以可写方式挂载 SA token

  🟠 High (4)
  ─────────────────────────────────────────────────────
  5.2.1 最小化 Secret 访问
        状态: ⚠️  部分
        ...

  [D] 查看详情  [A] 自动修复（仅部分可自动修复）  [E] 导出报告  [Q] 返回
```

### 51.5 RBAC 审计

```go
func (a *RBACAuditor) Audit(ctx context.Context) (*RBACAuditResult, error) {
    result := &RBACAuditResult{}
    
    // 1. 扫描 wildcard 权限
    roles, _ := a.k8sClient.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
    for _, role := range roles.Items {
        for _, rule := range role.Rules {
            for _, apiGroup := range rule.APIGroups {
                if apiGroup == "*" {
                    for _, resource := range rule.Resources {
                        if resource == "*" {
                            result.Issues = append(result.Issues, RBACIssue{
                                Type:        "wildcard-permission",
                                Resource:    role.Name,
                                Subject:     "ClusterRole",
                                Severity:    "high",
                                Description: fmt.Sprintf("ClusterRole %s 包含 * * 通配符权限", role.Name),
                                Remediation: "限制为特定资源类型和 API 组",
                            })
                        }
                    }
                }
            }
        }
    }
    
    // 2. 扫描 cluster-admin 滥用
    bindings, _ := a.k8sClient.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
    for _, binding := range bindings.Items {
        if binding.RoleRef.Name == "cluster-admin" {
            for _, subject := range binding.Subjects {
                result.Issues = append(result.Issues, RBACIssue{
                    Type:        "cluster-admin-abuse",
                    Resource:    binding.Name,
                    Subject:     fmt.Sprintf("%s/%s", subject.Kind, subject.Name),
                    Severity:    "high",
                    Description: fmt.Sprintf("%s 被绑定到 cluster-admin", subject.Name),
                    Remediation: "创建最小权限 RoleBinding",
                })
            }
        }
    }
    
    return result, nil
}
```

### 51.6 合规映射报告

将 Agent 的安全检查结果映射到合规框架：

```go
func (g *ComplianceReportGenerator) MapToFramework(ctx context.Context, framework string, checks []CheckResult) (*ComplianceReport, error) {
    mappings := map[string]map[string]string{
        "PCI-DSS": {
            "network-segmentation":   "Req 1.3",
            "encryption-transit":     "Req 4.1",
            "access-audit":           "Req 10.2",
            "key-rotation":           "Req 3.6",
        },
        "SOC2": {
            "change-management":      "CC8.1",
            "access-control":         "CC6.1",
            "monitoring":             "CC7.2",
        },
        "等保2.0": {
            "security-audit":         "8.1.4.3",
            "intrusion-prevention":   "8.1.4.8",
            "data-integrity":         "8.1.4.5",
        },
    }
    
    // 生成报告
    report := &ComplianceReport{
        Framework: framework,
        GeneratedAt: time.Now(),
    }
    
    for _, check := range checks {
        if mapping, ok := mappings[framework][check.Category]; ok {
            report.Results = append(report.Results, FrameworkCheckResult{
                ControlID:   mapping,
                ControlName: check.Name,
                Status:      check.Status,
                Evidence:    check.Evidence,
            })
        }
    }
    
    return report, nil
}
```

### 51.7 定期合规报告

```yaml
# ~/.ops-ai/config.yaml
compliance:
  enabled: true
  
  # 扫描配置
  scan:
    cis_version: "1.8"
    frameworks: ["CIS", "SOC2"]         # 启用的框架
    
  # 定期报告
  scheduled_reports:
    enabled: true
    cron: "0 9 * * 1"                   # 每周一上午 9 点
    recipients: ["security@company.com"]
    formats: ["markdown", "pdf"]
    
  # RBAC 审计
  rbac_audit:
    enabled: true
    check_wildcard: true
    check_cluster_admin: true
    check_unused_bindings: true
    
  # PSS 检查
  pss_check:
    enabled: true
    target_level: "restricted"          # 目标级别
    namespaces: ["*"]                   # 检查的 namespace
```

### 51.8 System Prompt 补充

```
## 合规自动化知识

当运维询问合规或安全基线相关问题时：

1. **CIS Benchmark**：
   - 覆盖 K8s 集群安全配置基线
   - 部分检查项可自动修复（如标签、注解缺失）
   - Critical/High 级别失败需要优先处理

2. **合规框架映射**：
   - CIS → PCI-DSS / SOC2 / 等保2.0
   - Agent 自动生成映射报告，标明哪些控制项已覆盖
   - 标注 "manual" 的项需要人工补充证据

3. **RBAC 审计**：
   - 扫描 wildcard 权限、cluster-admin 滥用、未使用绑定
   - 提供最小权限修复建议

4. **定期报告**：
   - 可配置每周/月自动生成合规报告
   - 支持 Markdown 和 PDF 格式
   - 自动推送到安全团队邮箱
```

---

# 第五部分：P2 — 增强级（锦上添花）

---

## 52. 网络策略主动管理与可视化（v1.9 新增）

### 52.1 问题场景

v1.8 有网络诊断（§38 四层诊断：DNS→Service→NetworkPolicy→Pod），但仅限于"诊断不通"。缺少网络策略的主动管理：生成策略建议、可视化策略拓扑、审计策略冲突。

### 52.2 设计目标

- 基于服务依赖自动生成 NetworkPolicy 建议
- 可视化网络策略拓扑图
- 检测策略冲突和冗余规则

### 52.3 Go 接口定义

```go
// NetworkPolicyManager 网络策略管理器
type NetworkPolicyManager struct {
    k8sClient     kubernetes.Interface
    recommender   *NetworkPolicyRecommender
    visualizer    *PolicyTopologyVisualizer
    conflictDetector *PolicyConflictDetector
}

// NetworkPolicyRecommender 策略推荐器
type NetworkPolicyRecommender struct {
    k8sClient kubernetes.Interface
}

type NetworkPolicyRecommendation struct {
    Namespace   string
    TargetPod   string
    IngressRules []IngressRuleRecommendation
    EgressRules  []EgressRuleRecommendation
    Reason      string
}

type IngressRuleRecommendation struct {
    FromNamespaces []string
    FromPods       []string
    Ports          []networkingv1.NetworkPolicyPort
}

// PolicyTopologyVisualizer 策略拓扑可视化
type PolicyTopologyVisualizer struct{}

type PolicyTopology struct {
    Nodes []PolicyNode
    Edges []PolicyEdge
}

type PolicyNode struct {
    Namespace string
    Pod       string
    Labels    map[string]string
    Isolated  bool
}

type PolicyEdge struct {
    From      string
    To        string
    Allowed   bool
    PolicyRef string
}
```

### 52.4 策略推荐

```
ops-ai netpol recommend --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  🌐  NetworkPolicy 推荐 — payment namespace
  ═══════════════════════════════════════════════════════

  当前状态: 未隔离（允许所有入站/出站流量）

  推荐策略（基于实际流量分析）
  ─────────────────────────────────────────────────────
  payment-api:
    入站: 允许来自 ingress-nginx (80, 443)
          允许来自 payment-worker (8080)
    出站: 允许到 payment-db (5432)
          允许到 redis-cache (6379)
          允许到 kube-dns (53)

  payment-worker:
    入站: 拒绝所有（无外部调用）
    出站: 允许到 payment-db (5432)
          允许到 kafka (9092)

  [A] 应用推荐策略（L2，需确认）  [E] 导出 YAML  [Q] 返回
```

### 52.5 策略冲突检测

```go
func (d *PolicyConflictDetector) DetectConflicts(ctx context.Context, namespace string) ([]PolicyConflict, error) {
    policies, _ := d.k8sClient.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
    
    var conflicts []PolicyConflict
    
    for i, p1 := range policies.Items {
        for j, p2 := range policies.Items {
            if i >= j {
                continue
            }
            // 检测选择器重叠但规则矛盾的情况
            if d.hasOverlappingSelector(&p1, &p2) && d.hasContradictoryRules(&p1, &p2) {
                conflicts = append(conflicts, PolicyConflict{
                    Policy1: p1.Name,
                    Policy2: p2.Name,
                    Type:    "contradictory-rules",
                    Detail:  "两个策略匹配相同 Pod 但规则矛盾",
                })
            }
        }
    }
    
    return conflicts, nil
}
```

### 52.6 L0 命令扩展

```
ops-ai netpol recommend [-n <ns>]           # 生成策略建议
ops-ai netpol visualize [-n <ns>]           # 可视化策略拓扑
ops-ai netpol conflicts [-n <ns>]           # 检测策略冲突
ops-ai netpol audit [-n <ns>]               # 审计网络隔离状态
```

### 52.7 配置项

```yaml
network_policy:
  enabled: true
  recommend_default_deny: false       # 是否推荐默认拒绝策略
  analysis_period: "24h"              # 流量分析周期
```

---

## 53. Windows 节点支持（v1.9 新增）

### 53.1 问题场景

多架构构建（§31）覆盖了 amd64/arm64，但 Windows 节点在 K8s 生态中仍有大量使用（特别是 .NET 工作负载）。Agent 的工具链（kubectl exec、日志收集）在 Windows 节点上有行为差异，需要适配。

### 53.2 设计目标

- Windows 节点诊断适配
- Windows 容器日志收集
- Windows 特定故障排查（如 HNS 网络问题）

### 53.3 Windows 适配点

| 功能 | Linux 节点 | Windows 节点 |
|------|-----------|-------------|
| kubectl exec | bash/sh | cmd / PowerShell |
| 日志路径 | /var/log | C:\var\log |
| 进程查看 | ps | Get-Process |
| 网络诊断 | iptables / conntrack | HNS / VFP |
| 文件系统 | ext4/xfs | NTFS |

### 53.4 Go 接口定义

```go
// WindowsNodeAdapter Windows 节点适配器
type WindowsNodeAdapter struct {
    k8sClient kubernetes.Interface
}

func (a *WindowsNodeAdapter) DetectWindowsNodes(ctx context.Context) ([]corev1.Node, error) {
    nodes, err := a.k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
        LabelSelector: "kubernetes.io/os=windows",
    })
    return nodes.Items, err
}

func (a *WindowsNodeAdapter) GetWindowsDiagnostics(ctx context.Context, nodeName string) (*WindowsDiagnostics, error) {
    // 通过 kubectl exec 在 Windows 节点上运行 PowerShell 命令
    // 或使用 HostProcess Pod 进行诊断
}

type WindowsDiagnostics struct {
    NodeName      string
    OSVersion     string
    HNSEndpoints  []HNSEndpoint
    VFPPolicies   []string
    ContainerLogs []ContainerLog
}
```

### 53.5 Windows 故障诊断

```
ops-ai node diagnose <windows-node>

Windows 节点特有检查:
- HNS (Host Network Service) 状态
- VFP (Virtual Filtering Platform) 策略
- Windows 防火墙规则
- ContainerD / Docker 服务状态
- .NET 运行时版本
```

### 53.6 配置项

```yaml
windows:
  enabled: true
  shell: "powershell"                 # powershell | cmd
  diagnostic_image: "mcr.microsoft.com/windows/nanoserver:1809"
```

---

## 54. 反馈闭环与持续学习（v1.9 新增）

### 54.1 问题场景

v1.8 没有设计"操作结果 → Agent 改进"的反馈闭环。Agent 执行了一个修复操作，成功了或失败了，这个经验如何反哺到 Runbook RAG（§39）或 System Prompt 规则中？

### 54.2 设计目标

- 操作结果自动记录（成功/失败/回滚）
- 成功案例自动 enrich Runbook RAG
- 失败案例自动更新 System Prompt 规则（避免重复犯错）
- 人工反馈收集（👍/👎）

### 54.3 Go 接口定义

```go
// FeedbackLoopManager 反馈闭环管理器
type FeedbackLoopManager struct {
    store         FeedbackStore
    runbookRAG    *RunbookRAG
    promptUpdater *SystemPromptUpdater
}

type OperationFeedback struct {
    SessionID     string
    Operation     string
    ToolCall      ToolCallPattern
    Result        string  // success | failed | rolled-back
    UserFeedback  *UserFeedback
    MetricsDelta  map[string]float64
    Learnings     []string
}

type UserFeedback struct {
    Rating        int     // 1-5
    Comment       string
    WouldRecommend bool
}

// SystemPromptUpdater System Prompt 更新器
type SystemPromptUpdater struct {
    ruleStore RuleStore
}

func (u *SystemPromptUpdater) AddRuleFromFailure(ctx context.Context, feedback OperationFeedback) error {
    // 从失败案例生成新规则
    rule := PromptRule{
        Condition: fmt.Sprintf("当 %s 失败且错误包含 '%s'", feedback.ToolCall.ToolName, feedback.ToolCall.Error),
        Action:    fmt.Sprintf("先检查 %s，再重试", feedback.Learnings[0]),
        Source:    "auto-generated-from-feedback",
        Confidence: 0.8,
    }
    
    return u.ruleStore.AddRule(rule)
}
```

### 54.4 反馈收集流程

```
运维执行修复操作后:

Agent:
  "修复操作已执行。请评价此次辅助是否有帮助："

  [1] ⭐ 非常有帮助
  [2] ⭐⭐ 有帮助
  [3] ⭐⭐⭐ 一般
  [4] ⭐⭐⭐⭐ 需要改进
  [5] ⭐⭐⭐⭐⭐ 完全没用 / 导致更严重问题

  你的反馈将帮助我们改进 Agent 的排查能力。
```

### 54.5 Runbook RAG 自动更新

```go
func (f *FeedbackLoopManager) UpdateRunbookRAG(ctx context.Context, feedback OperationFeedback) error {
    if feedback.Result != "success" {
        return nil // 只从成功案例学习
    }
    
    // 1. 提取成功案例的上下文
    caseStudy := RunbookCase{
        Scenario:    feedback.Operation,
        Symptoms:    feedback.ToolCall.Args,
        Solution:    feedback.Learnings,
        Source:      "agent-feedback",
        SuccessRate: 1.0,
    }
    
    // 2. 添加到 RAG 向量数据库
    return f.runbookRAG.IndexCase(caseStudy)
}
```

### 54.6 配置项

```yaml
feedback_loop:
  enabled: true
  
  # 自动学习
  auto_learn:
    from_success: true
    from_failure: true
    min_confidence: 0.7
    
  # 人工反馈
  user_feedback:
    prompt_after_session: true          # 会话结束后请求反馈
    min_session_length: 3               # 至少 3 轮对话才请求反馈
    
  # RAG 更新
  runbook_rag:
    auto_index: true
    review_required: false              # 是否需人工审核后入库
```

---

## 55. 插件/扩展机制（v1.9 新增）

### 55.1 问题场景

v1.8 定义了完善的工具接口（Tool interface，§14），但没有设计插件/扩展框架。运维团队如何注入自定义工具（如内部 CMDB 查询、自研运维平台 API）？第三方如何扩展 Agent 能力？

### 55.2 设计目标

- 插件注册和发现机制
- 自定义工具注入接口
- 插件隔离和安全沙箱

### 55.3 Go 接口定义

```go
// PluginManager 插件管理器
type PluginManager struct {
    registry  PluginRegistry
    loader    PluginLoader
    sandbox   PluginSandbox
}

// Plugin 插件接口
type Plugin interface {
    Name() string
    Version() string
    Init(config map[string]interface{}) error
    GetTools() []Tool
    GetCommands() []CLICommand
}

// PluginRegistry 插件注册表
type PluginRegistry struct {
    plugins map[string]Plugin
}

func (r *PluginRegistry) Register(p Plugin) error {
    // 验证插件签名（如果启用）
    // 检查插件权限
    r.plugins[p.Name()] = p
    return nil
}

// PluginLoader 插件加载器
type PluginLoader struct {
    pluginDir string
}

func (l *PluginLoader) LoadAll() ([]Plugin, error) {
    // 从 ~/.ops-ai/plugins/ 加载所有插件
    // 支持 Go plugin (.so) 或 gRPC 插件（外部进程）
}
```

### 55.4 插件目录结构

```
~/.ops-ai/plugins/
├── cmdb-query/                      # 内部 CMDB 查询插件
│   ├── plugin.yaml                  # 插件描述
│   └── cmdb-query.so                # 编译后的插件
├── custom-monitor/                  # 自定义监控插件
│   ├── plugin.yaml
│   └── custom-monitor
└── internal-api/                    # 内部运维平台 API
    ├── plugin.yaml
    └── internal-api
```

### 55.5 插件描述文件

```yaml
# plugin.yaml
name: "cmdb-query"
version: "1.0.0"
author: "ops-team"
description: "查询内部 CMDB 获取服务器信息"
permissions:
  - "network:outbound:cmdb.company.com"
  - "file:read:~/.ops-ai/cmdb-token"
tools:
  - name: "cmdb_lookup"
    description: "根据 IP 查询 CMDB 信息"
    args:
      - name: "ip"
        type: "string"
        required: true
```

### 55.6 自定义工具注入

```go
// 插件实现示例
type CMDBPlugin struct{}

func (p *CMDBPlugin) Name() string { return "cmdb-query" }
func (p *CMDBPlugin) Version() string { return "1.0.0" }

func (p *CMDBPlugin) Init(config map[string]interface{}) error {
    // 初始化 CMDB 客户端
    return nil
}

func (p *CMDBPlugin) GetTools() []Tool {
    return []Tool{
        &CMDBLookupTool{},
    }
}

type CMDBLookupTool struct{}

func (t *CMDBLookupTool) Name() string { return "cmdb_lookup" }
func (t *CMDBLookupTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    ip := args["ip"].(string)
    // 调用 CMDB API
    return queryCMDB(ip)
}
```

### 55.7 L0 命令扩展

```
ops-ai plugin list                      # 列出已加载插件
ops-ai plugin install <path>            # 安装插件
ops-ai plugin remove <name>             # 卸载插件
ops-ai plugin verify <name>             # 验证插件签名
```

### 55.8 配置项

```yaml
plugins:
  enabled: true
  plugin_dir: "~/.ops-ai/plugins"
  
  # 安全
  security:
    require_signature: false            # 是否要求插件签名
    allowed_permissions:                # 允许的权限列表
      - "network:outbound:*"
      - "file:read:~/.ops-ai/*"
    max_execution_time: "30s"           # 插件工具最大执行时间
    
  # 内置插件
  builtin_plugins:
    - "cmdb-query"
    - "custom-monitor"
```

---

## 56. 国际化/本地化（v1.9 新增）

### 56.1 问题场景

v1.8 文档全中文撰写，System Prompt 为英文。但 TUI 界面语言、错误消息语言、审计日志语言没有明确策略。面向全球开源社区，至少需要中英双语支持。

### 56.2 设计目标

- TUI 界面多语言支持
- 错误消息/审计日志多语言
- System Prompt 多语言模板
- 文档多语言

### 56.3 实现方案

```go
// I18nManager 国际化管理器
type I18nManager struct {
    locale    string
    messages  map[string]map[string]string
}

func (i *I18nManager) T(key string, args ...interface{}) string {
    msg := i.messages[i.locale][key]
    if msg == "" {
        msg = i.messages["en"][key] // fallback 到英文
    }
    return fmt.Sprintf(msg, args...)
}

// 使用示例
fmt.Println(i18n.T("prompt.confirm_deletion", resourceName))
// zh: "确认删除资源 %s？"
// en: "Confirm deletion of resource %s?"
```

### 56.4 语言文件

```yaml
# ~/.ops-ai/i18n/zh.yaml
prompt:
  confirm_deletion: "确认删除资源 %s？"
  operation_cancelled: "操作已取消"
  budget_exhausted: "会话预算已耗尽"
  
error:
  api_timeout: "API 调用超时"
  permission_denied: "权限不足"
  
# ~/.ops-ai/i18n/en.yaml
prompt:
  confirm_deletion: "Confirm deletion of resource %s?"
  operation_cancelled: "Operation cancelled"
  budget_exhausted: "Session budget exhausted"
  
error:
  api_timeout: "API call timed out"
  permission_denied: "Permission denied"
```

### 56.5 配置项

```yaml
i18n:
  locale: "zh"                        # zh | en | auto
  fallback: "en"
  # 自动检测优先级: flag > env > system locale
```

---

## 57. 破窗效应防护与紧急通道（Break-Glass）（v1.9 新增）

### 57.1 问题场景

v1.8 有 L4（最高风险操作）需要双人审批，但没有设计 break-glass 紧急程序——当审批人不可达时如何紧急操作？这在 P0 事故时是刚需。

### 57.2 设计目标

- P0 事故时允许临时提升操作权限
- 全程审计 + 事后强制复盘
- 自动通知 + 时间窗口限制
- 与事件管理（§47）联动

### 57.3 Go 接口定义

```go
// BreakGlassManager 紧急通道管理器
type BreakGlassManager struct {
    store      BreakGlassStore
    notifier   BreakGlassNotifier
    config     BreakGlassConfig
}

type BreakGlassRequest struct {
    ID            string
    Requester     string
    IncidentID    string       // 关联的事件
    Reason        string
    RequestedAt   time.Time
    ExpiresAt     time.Time
    ApprovedBy    []string     // 事后审批人（可以是自动化）
    Permissions   []string     // 临时获得的权限
    Status        string       // pending | active | expired | revoked
}

// BreakGlassApproval 紧急通道审批
type BreakGlassApproval struct {
    RequestID     string
    Approver      string
    ApprovedAt    time.Time
    Method        string  // emergency-contact | auto-escalation | post-incident
}
```

### 57.4 Break-Glass 流程

```
场景: P0 事故，审批人 offline，需要紧急执行 L4 操作

运维: "ops-ai break-glass activate --incident INC-001 --reason 'payment-db 主节点宕机，需要强制删除 StatefulSet 重新创建'"

Agent:
  1. 验证当前事件是 P0 且已确认
  2. 验证请求者身份和当前 on-call 状态
  3. 生成 break-glass token（15 分钟有效期）
  4. 自动通知:
     - Slack: #security-alerts
     - 邮件: security@company.com, manager@company.com
     - PagerDuty: 高优先级事件
  5. 激活 break-glass 模式

  ═══════════════════════════════════════════════════════
  🚨  BREAK-GLASS 模式已激活
  ═══════════════════════════════════════════════════════

  Token:      BG-20240625-001
  请求者:      alice
  关联事件:    INC-20240625-001
  原因:       payment-db 主节点宕机，需要强制删除 StatefulSet 重新创建
  有效期:     15 分钟（14:32 过期）

  临时权限
  ─────────────────────────────────────────────────────
  ✅ L4 操作豁免（单次会话）
  ✅ 绕过双人审批
  ⚠️  所有操作将被高亮审计

  已通知: security@, manager@, #security-alerts

  [C] 继续操作  [R] 撤销  [Q] 返回
```

### 57.5 Break-Glass 安全护栏

```go
func (m *BreakGlassManager) Activate(ctx context.Context, req BreakGlassRequest) (*BreakGlassRequest, error) {
    // 1. 验证事件严重度
    incident := m.incidentStore.Get(req.IncidentID)
    if incident == nil || incident.Severity != IncidentSeverityP0 {
        return nil, fmt.Errorf("break-glass 仅适用于 P0 事件")
    }
    
    // 2. 验证请求者权限
    user := m.tenantManager.GetCurrentUser()
    if user.Role != UserRoleAdmin && user.Role != UserRoleOperator {
        return nil, fmt.Errorf("权限不足")
    }
    
    // 3. 生成限时 token
    req.ID = generateBreakGlassToken()
    req.ExpiresAt = time.Now().Add(m.config.Duration)
    req.Status = "active"
    
    // 4. 通知
    m.notifier.Notify(req)
    
    // 5. 记录
    m.store.Create(req)
    
    // 6. 设置自动过期
    go m.autoExpire(req.ID, m.config.Duration)
    
    return &req, nil
}
```

### 57.6 事后审计与复盘

```go
func (m *BreakGlassManager) PostIncidentReview(ctx context.Context, tokenID string) error {
    req := m.store.Get(tokenID)
    if req == nil {
        return fmt.Errorf("token not found")
    }
    
    // 1. 生成 break-glass 使用报告
    report := BreakGlassReport{
        Request:       req,
        Operations:    m.auditStore.GetByBreakGlassToken(tokenID),
        Duration:      req.ExpiresAt.Sub(req.RequestedAt),
    }
    
    // 2. 强制创建事后复盘任务
    return m.incidentManager.CreatePostMortemTask(req.IncidentID, 
        fmt.Sprintf("Break-glass 使用审查: %s", tokenID))
}
```

### 57.7 配置项

```yaml
# ~/.ops-ai/config.yaml
break_glass:
  enabled: true
  
  # 激活条件
  activation:
    require_p0_incident: true           # 必须关联 P0 事件
    require_oncall: true                # 请求者必须在 on-call 列表
    max_duration: "15m"                 # 最大有效期
    
  # 通知
  notification:
    channels: ["slack", "email", "pagerduty"]
    slack_channel: "#security-alerts"
    email_recipients:
      - "security@company.com"
      - "sre-manager@company.com"
      
  # 事后审查
  post_incident:
    require_review: true                # 必须事后审查
    review_deadline: "24h"              # 24 小时内完成审查
    auto_escalate_if_no_review: true    # 未按时审查自动升级
```

### 57.8 System Prompt 补充

```
## Break-Glass 紧急通道知识

Break-glass 是在紧急情况下临时提升 Agent 操作权限的机制：

1. **适用场景**：
   - P0 生产事故
   - 常规审批流程无法满足时间要求
   - 例如：数据库主节点宕机，需要立即强制重建

2. **激活条件**：
   - 必须关联已确认的 P0 事件
   - 请求者必须在 on-call 列表中
   - 有效期最长 15 分钟

3. **安全护栏**：
   - 所有操作会被高亮标记在审计日志中
   - 自动通知安全团队和管理层
   - 事后必须完成审查

4. **禁止**：
   - 非 P0 事件使用 break-glass
   - 超出有效期继续操作
   - 未关联事件直接使用

5. **事后审查**：
   - 24 小时内必须完成 break-glass 使用审查
   - 审查内容包括：必要性、操作正确性、是否有替代方案
   - 未按时审查会自动升级到管理层
```

---

# 第六部分：开发路线图 v1.9

## Phase 1: P0 阻断级（4 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 1 | §40 Agent 自观测性 | /metrics 端点、健康检查、审计 failover |
| 1 | §43 Agent SLA 与运行时护栏 | Token 预算、保留策略、并发控制 |
| 2 | §41 密钥轮换与证书生命周期 | cert scan、Secret 依赖链、ESO 集成 |
| 2 | §42 灾难恢复与集群级备份 | etcd 诊断、Velero 集成、CSI 快照 |
| 3-4 | P0 集成测试 + 安全回归测试 | 测试报告、文档更新 |

## Phase 2: P1 重要级（6 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 5 | §44 多租户/多团队隔离 | 团队身份模型、审计隔离、配置继承 |
| 5 | §45 供应链安全 | 镜像扫描、准入预检、SBOM |
| 6 | §46 混沌工程集成 | ChaosMesh 集成、实验生成、结果分析 |
| 6 | §47 事件全生命周期管理 | Incident 管理、War Room、Postmortem |
| 7 | §48 集群升级辅助 | Preflight 检查、节点升级编排 |
| 7 | §49 CSI/存储运维深度 | CSI 诊断、PVC 分析、性能诊断 |
| 8 | §50 容量规划与资源推荐 | Right-sizing、容量预测、VPA 集成 |
| 8 | §51 合规自动化 | CIS 扫描、RBAC 审计、合规报告 |
| 9-10 | P1 集成测试 + 企业场景验证 | 测试报告、性能基准 |

## Phase 3: P2 增强级（4 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 11 | §52 网络策略主动管理 + §53 Windows 节点支持 | 策略推荐、拓扑可视化、Windows 诊断 |
| 12 | §54 反馈闭环与持续学习 + §55 插件机制 | 反馈收集、RAG 更新、插件框架 |
| 13 | §56 国际化/本地化 + §57 Break-Glass | 多语言支持、紧急通道 |
| 14 | 全量回归测试 + 文档完善 | 测试报告、最终文档 |

## 总工期

**14 周**（约 3.5 个月）

---

# 附录 A：Changelog

## v1.8 → v1.9（2026-06-25）— 生产可靠版 + 企业就绪

### P0 — 阻断级（4 项）
- **§40 Agent 自观测性**：新增 `/metrics` Prometheus 端点、Agent 健康检查、审计写入 failover、崩溃告警
- **§41 密钥轮换与证书生命周期**：新增证书过期巡检、Secret 依赖链分析、ESO 集成、kubeconfig 证书预警
- **§42 灾难恢复与集群级备份**：新增 etcd 诊断、Velero 集成、CSI VolumeSnapshot、控制平面降级模式、跨集群故障转移
- **§43 Agent SLA 与运行时护栏**：新增单会话 token 预算、审计/会话保留策略、CI/CD 并发控制、资源护栏

### P1 — 重要级（8 项）
- **§44 多租户/多团队隔离**：新增团队身份模型、审计日志隔离、配置继承、会话可见性控制、Namespace ACL
- **§45 供应链安全**：新增镜像漏洞扫描（Trivy）、镜像签名验证（Cosign）、准入策略预检、SBOM 生成
- **§46 混沌工程集成**：新增 ChaosMesh/Litmus 集成、故障演练 Runbook 生成、实验结果分析、与告警自动修复联动
- **§47 事件全生命周期管理**：新增 Incident 管理、Timeline 自动生成、War Room 集成、Postmortem 自动起草
- **§48 K8s 集群升级辅助**：新增升级预检（废弃 API、addon 兼容性）、节点滚动升级编排、升级后验证
- **§49 CSI/存储运维深度**：新增 CSI 驱动健康检查、PVC Pending 诊断、VolumeSnapshot 恢复、存储性能诊断
- **§50 容量规划与资源推荐**：新增 Right-sizing 推荐、容量预测、VPA 集成、资源浪费检测
- **§51 合规自动化**：新增 CIS Benchmark 扫描、RBAC 审计、PSS 检查、合规映射报告（PCI-DSS/SOC2/等保2.0）

### P2 — 增强级（6 项）
- **§52 网络策略主动管理**：新增 NetworkPolicy 推荐、策略拓扑可视化、冲突检测
- **§53 Windows 节点支持**：新增 Windows 节点诊断适配、HNS/VFP 检查
- **§54 反馈闭环与持续学习**：新增操作结果反馈、Runbook RAG 自动更新、System Prompt 规则学习
- **§55 插件/扩展机制**：新增插件注册/加载框架、自定义工具注入、权限沙箱
- **§56 国际化/本地化**：新增 TUI/错误消息/审计日志多语言支持（中英）
- **§57 破窗效应防护与紧急通道**：新增 Break-glass 机制、P0 临时权限提升、事后强制审查

### 版本演进全景

```
v1.0 概念验证版  →  v1.1 可交互版  →  v1.2 安全初版  →  v1.3 生产集群实战版
    ↓
v1.4 可交付版  →  v1.5 企业可用版  →  v1.6 企业可推广版  →  v1.7 产品可交付版
    ↓
v1.8 生产安全版  ────────────────────────────────────────────────→  v1.9 生产可靠版 + 企业就绪
    (五级网关 + 爆炸半径 + GitOps + 超时防护 + 多集群安全)        (自观测性 + DR + 多租户 + 合规 + 混沌工程)
```

---

**文档版本**：v1.9
**日期**：2026-06-25
**变更**：基于 PRD v1.8 差距分析，补齐全部 18 项缺口（4 P0 + 8 P1 + 6 P2）
