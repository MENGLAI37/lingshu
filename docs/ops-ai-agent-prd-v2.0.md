# 运维 AI Agent 产品需求文档 (PRD) v2.0

> **文档用途**：面向产品设计师、架构师、开发团队的完整需求规格说明
> **版本**：v2.0 — 规模化生产版（补齐 Agent 高可用、自升级、大规模性能、服务网格、发布策略、变更窗口、告警降噪、CronJob、Agent 调试、Terraform 深度、数据驻留、边缘计算。从"企业就绪"到"规模化生产可靠"。）
> **日期**：2026-06-25
> **变更**：v1.9 → v2.0 补齐了 v1.9 运维视角差距分析中的 12 项缺口（3 P0 + 4 P1 + 5 P2），详见末尾 Changelog
> **前置文档**：本 PRD 基于 v1.9 全部功能（§1-§57）进行增量扩展

---

# 第一部分：给所有人的 Executive Summary

## 我们要做什么

v1.9 实现了"Agent 可靠地融入企业"——自观测性、多租户隔离、合规自动化、事件全生命周期等 18 项缺口全部补齐。v2.0 的核心目标是解决**规模化生产运维**中的最后一层挑战：

1. **"Agent 自身必须和生产服务一样可靠"** — 高可用部署、零停机升级、千节点性能
2. **"Agent 必须懂现代应用架构"** — 服务网格诊断、金丝雀发布辅助、变更窗口管理
3. **"Agent 必须聪明地处理噪音"** — 告警聚类降噪、通知疲劳防护
4. **"Agent 必须覆盖全栈运维"** — CronJob/批处理、Terraform 状态、边缘节点、数据主权

## 一句话定位

> **v1.9 让 Agent 企业就绪；v2.0 让 Agent 规模化生产可靠。**

## v2.0 与 v1.9 的关系

v2.0 是 v1.9 的**增量扩展**。所有 v1.9 的功能分级、安全网关、审计日志、多租户模型、合规框架均保持不变。v2.0 新增第 58-69 部分，解决 v1.9 差距分析中的 12 项缺口。

---

# 第二部分：差距分析 → 解决方案映射

| 优先级 | 编号 | 缺口 | 核心风险 | 解决方案所在章节 |
|--------|------|------|----------|----------------|
| **P0** | 1 | Agent 自身高可用部署 | Agent 挂了 = 所有自动化停摆，而且没人告警 | §58 |
| **P0** | 2 | Agent 自身升级机制 | 自己不能升级自己，每次发版都是手工运维噩梦 | §59 |
| **P0** | 3 | 大规模集群性能瓶颈 | 1000+ 节点集群上一个扫描命令卡死 10 分钟 | §60 |
| **P1** | 4 | 服务网格深度运维 | 50%+ 生产集群用 Istio，Agent 对此完全失明 | §61 |
| **P1** | 5 | 发布策略辅助 | SRE 日常最大工作量就是发布，Agent 帮不上忙 | §62 |
| **P1** | 6 | 变更窗口与维护模式 | 双 11 期间 Agent 自动"修复"导致事故 | §63 |
| **P1** | 7 | 告警降噪与通知疲劳 | 告警风暴直接打爆成本和并发上限 | §64 |
| **P2** | 8 | CronJob / 批处理任务运维 | 批处理任务失败是高频 on-call 来源 | §65 |
| **P2** | 9 | Agent 自身调试能力 | Agent 犯错后无法排查，只能重启 | §66 |
| **P2** | 10 | Terraform / IaC 状态管理 | K8s 和 TF 资源不一致导致"幽灵资源" | §67 |
| **P2** | 11 | 数据驻留与主权合规 | 合规审计时无法证明数据未出境 | §68 |
| **P2** | 12 | 边缘计算 / K3s 支持 | 边缘场景快速增长，Agent 无法覆盖 | §69 |

---

# 第三部分：P0 — 阻断级（不解决无法规模化）

---

## 58. Agent 高可用部署模型（v2.0 新增）

### 58.1 问题场景

v1.9 花了大量篇幅设计 Agent 如何管集群，但**Agent 自身是单点**这个根本问题没有解决：

- Agent 以单 Pod/单进程运行，如果节点故障或 OOMKilled，所有自动化能力（告警自动修复 §26、定时巡检 §22、审计 §17）全部中断
- alertd（§24）是独立守护进程，但文档没有设计 alertd 的多副本部署和 Leader Election
- 审计日志写入 SQLite，单文件在 NFS 或某些存储后端上有锁竞争风险
- **实际运维场景**：凌晨 3 点 Agent 所在节点被驱逐，告警自动修复完全停摆，没人知道，直到早晨运维手动发现

### 58.2 设计目标

- Agent 支持多副本部署，单实例故障不影响服务
- alertd 支持 Leader Election，避免多实例重复处理告警
- 审计日志支持外部数据库后端（PostgreSQL），解决并发和 HA 问题
- 会话状态可持久化到共享存储，新实例可恢复

### 58.3 Go 接口定义

```go
// HAManager Agent 高可用管理器
type HAManager struct {
    k8sClient     kubernetes.Interface
    leaseClient   coordinationv1.LeaseInterface
    identity      string                    // 当前实例唯一标识
    config        HAConfig
}

// AgentInstance Agent 实例状态
type AgentInstance struct {
    ID            string
    PodName       string
    NodeName      string
    StartedAt     time.Time
    LastHeartbeat time.Time
    Role          InstanceRole  // leader | follower | candidate
    Sessions      []string      // 当前承载的会话 ID
}

type InstanceRole string

const (
    InstanceRoleLeader    InstanceRole = "leader"
    InstanceRoleFollower  InstanceRole = "follower"
    InstanceRoleCandidate InstanceRole = "candidate"
)

// LeaderElection Leader 选举
type LeaderElection struct {
    leaseName      string
    leaseNamespace string
    identity       string
    leaseDuration  time.Duration
    renewDeadline  time.Duration
    retryPeriod    time.Duration
}

func (e *LeaderElection) Run(ctx context.Context) error {
    // 使用 K8s Lease API 进行 Leader Election
    // 与 controller-runtime 的 leader election 机制一致
}

// SessionStore 会话持久化存储
type SessionStore interface {
    Save(session *Session) error
    Load(sessionID string) (*Session, error)
    ListActive() ([]*Session, error)
    Delete(sessionID string) error
}

// DistributedSessionStore 分布式会话存储
type DistributedSessionStore struct {
    backend string  // "sqlite" | "postgresql" | "etcd"
    db      *sql.DB
}

// AuditLogHAStore 高可用审计日志存储
type AuditLogHAStore struct {
    primary   AuditSink    // 主存储：PostgreSQL / 云数据库
    fallback  AuditSink    // 降级存储：本地 SQLite
    buffer    *AuditBuffer // 内存缓冲
}
```

### 58.4 部署架构

```
┌─────────────────────────────────────────────────────────────┐
│                    K8s Cluster                               │
│                                                              │
│   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐     │
│   │  ops-ai-0   │   │  ops-ai-1   │   │  ops-ai-2   │     │
│   │  (Leader)   │   │  (Follower) │   │  (Follower) │     │
│   │             │   │  (热备)      │   │  (热备)      │     │
│   └──────┬──────┘   └─────────────┘   └─────────────┘     │
│          │                                                   │
│          │  Lease Lock: ops-ai-leader                        │
│          │  Session Store: PostgreSQL / etcd                 │
│          │  Audit Log: PostgreSQL (主) + 本地 SQLite (备)    │
│          │                                                   │
│   ┌──────┴──────────────────────────────────────────────┐  │
│   │              Shared Storage (PVC / 云盘)              │  │
│   │  - Session snapshots                                  │  │
│   │  - Runbook RAG 向量数据库                             │  │
│   │  - Plugin 目录                                        │  │
│   └───────────────────────────────────────────────────────┘  │
│                                                              │
│   ┌───────────────────────────────────────────────────────┐  │
│   │              alertd StatefulSet (3 replicas)           │  │
│   │  - 只有 Leader 处理告警                                 │  │
│   │  - Follower 监听但不执行                                │  │
│   │  - Leader 故障时 30s 内自动切换                         │  │
│   └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 58.5 Leader Election 实现

```go
func (a *Agent) startHA(ctx context.Context) error {
    // 1. 尝试获取 Lease
    lease := &coordinationv1.Lease{
        ObjectMeta: metav1.ObjectMeta{
            Name:      a.config.HA.LeaseName,
            Namespace: a.config.HA.Namespace,
        },
        Spec: coordinationv1.LeaseSpec{
            HolderIdentity:       &a.identity,
            LeaseDurationSeconds: int32(a.config.HA.LeaseDuration.Seconds()),
            AcquireTime:          &metav1.MicroTime{Time: time.Now()},
            RenewTime:            &metav1.MicroTime{Time: time.Now()},
        },
    }
    
    // 2. 创建或更新 Lease
    for {
        existing, err := a.leaseClient.Get(ctx, lease.Name, metav1.GetOptions{})
        if err != nil {
            // Lease 不存在，创建
            _, err = a.leaseClient.Create(ctx, lease, metav1.CreateOptions{})
            if err == nil {
                a.becomeLeader()
                break
            }
        } else {
            // 检查 Lease 是否过期
            if existing.Spec.RenewTime == nil || 
               time.Since(existing.Spec.RenewTime.Time) > a.config.HA.LeaseDuration {
                // Lease 已过期，尝试接管
                existing.Spec.HolderIdentity = &a.identity
                existing.Spec.AcquireTime = &metav1.MicroTime{Time: time.Now()}
                _, err = a.leaseClient.Update(ctx, existing, metav1.UpdateOptions{})
                if err == nil {
                    a.becomeLeader()
                    break
                }
            }
        }
        
        // 未成为 Leader，进入 Follower 模式
        a.becomeFollower()
        time.Sleep(a.config.HA.RetryPeriod)
    }
    
    // 3. 启动心跳续租
    go a.renewLease(ctx)
    
    return nil
}
```

### 58.6 Follower 模式行为

```go
func (a *Agent) becomeFollower() {
    a.role = InstanceRoleFollower
    
    // Follower 能力限制
    a.capabilities = Capabilities{
        AcceptInteractiveSessions: false,  // 不接受交互式会话
        AcceptCICDSessions:        true,   // 接受 CI/CD 请求（只读诊断）
        ProcessAlerts:             false,  // 不处理告警
        RunScheduledTasks:         false,  // 不执行定时任务
        ServeMetrics:              true,   // 暴露 metrics（用于监控）
    }
    
    // 启动健康检查，随时准备竞选 Leader
    go a.watchLeaderHealth()
}
```

### 58.7 会话故障转移

```go
func (a *Agent) handleLeaderChange(ctx context.Context) {
    // 新 Leader 启动时，恢复所有活跃的会话
    sessions, err := a.sessionStore.ListActive()
    if err != nil {
        log.Printf("恢复会话失败: %v", err)
        return
    }
    
    for _, sess := range sessions {
        // 检查会话最后活跃时间
        if time.Since(sess.LastActivity) > a.config.SessionTimeout {
            // 会话已超时，标记为过期
            a.sessionStore.Expire(sess.ID)
            continue
        }
        
        // 恢复会话上下文
        restored := a.restoreSession(sess)
        
        // 通知用户
        a.notifier.NotifyUser(sess.UserID, fmt.Sprintf(
            "Agent 实例已切换，会话 %s 已恢复。上次操作: %s",
            sess.ID, sess.LastOperation,
        ))
    }
}
```

### 58.8 审计日志高可用存储

```go
func (s *AuditLogHAStore) Write(entry AuditEntry) error {
    // 1. 先写入内存缓冲（永不阻塞主流程）
    s.buffer.Write(entry)
    
    // 2. 异步写入主存储（PostgreSQL）
    go func() {
        err := s.primary.Write(entry)
        if err != nil {
            // 3. 主存储失败，写入降级存储（本地 SQLite）
            s.fallback.Write(entry)
            
            // 4. 标记主存储异常，触发告警
            s.metrics.AuditPrimaryFailures.Inc()
        }
    }()
    
    return nil
}
```

### 58.9 TUI 高可用状态展示

```
  ═══════════════════════════════════════════════════════
  🏛️  Agent 高可用状态
  ═══════════════════════════════════════════════════════

  本实例
  ─────────────────────────────────────────────────────
  ID:         ops-ai-1
  角色:       Leader ✅
  启动时间:    2024-06-25 08:00:00
  心跳:       正常（上次续租: 2s 前）

  集群状态
  ─────────────────────────────────────────────────────
  实例        角色       状态       最后心跳
  ─────────────────────────────────────────────────────
  ops-ai-0    Leader     ✅ 健康    2s
  ops-ai-1    Follower   ✅ 健康    5s
  ops-ai-2    Follower   ✅ 健康    3s

  会话分布
  ─────────────────────────────────────────────────────
  Leader 承载:  3 个活跃会话
  存储后端:     PostgreSQL ✅ 连通
  审计主存储:   PostgreSQL ✅ 正常

  [Q] 关闭面板
```

### 58.10 配置项

```yaml
# ~/.ops-ai/config.yaml
high_availability:
  enabled: true
  
  # Leader Election
  leader_election:
    lease_name: "ops-ai-leader"
    lease_namespace: "ops-ai"
    lease_duration: "15s"
    renew_deadline: "10s"
    retry_period: "2s"
    
  # 会话存储
  session_store:
    backend: "postgresql"               # sqlite | postgresql | etcd
    postgresql:
      host: "postgres.ops-ai.svc"
      port: 5432
      database: "ops_ai_sessions"
      user: "ops_ai"
      password: "${POSTGRES_PASSWORD}"
      ssl_mode: "require"
      max_connections: 20
      
  # 审计日志 HA 存储
  audit_storage:
    primary: "postgresql"
    postgresql:
      host: "postgres.ops-ai.svc"
      port: 5432
      database: "ops_ai_audit"
    fallback: "sqlite"                  # 主存储故障时降级到 SQLite
    
  # 共享存储
  shared_storage:
    type: "pvc"                         # pvc | nfs | s3
    pvc_name: "ops-ai-shared"
    
  # alertd HA
  alertd:
    replicas: 3
    leader_election: true
```

### 58.11 K8s 部署清单

```yaml
# ops-ai-ha-deployment.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ops-ai
  namespace: ops-ai
spec:
  serviceName: ops-ai
  replicas: 3
  selector:
    matchLabels:
      app: ops-ai
  template:
    metadata:
      labels:
        app: ops-ai
    spec:
      serviceAccountName: ops-ai
      containers:
      - name: ops-ai
        image: registry.company.com/ops-ai:v2.0.0
        args:
          - "--ha-enabled"
          - "--ha-lease-name=ops-ai-leader"
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: ops-ai-postgres
              key: password
        resources:
          limits:
            memory: "512Mi"
            cpu: "500m"
        volumeMounts:
        - name: shared-data
          mountPath: /data
      volumes:
      - name: shared-data
        persistentVolumeClaim:
          claimName: ops-ai-shared
  volumeClaimTemplates:
  - metadata:
      name: ops-ai-shared
    spec:
      accessModes: ["ReadWriteMany"]
      resources:
        requests:
          storage: 10Gi
```

### 58.12 System Prompt 补充

```
## Agent 高可用知识

当运维询问 Agent 自身状态或部署相关问题时：

1. **Leader/Follower**：
   - 当前实例角色可在 TUI 状态栏查看
   - 只有 Leader 处理交互式会话和告警
   - Follower 只接受 CI/CD 只读请求和暴露 metrics

2. **故障转移**：
   - Leader 故障后，Follower 在 15-30s 内自动竞选 Leader
   - 活跃会话会从共享存储恢复
   - 用户会收到会话恢复通知

3. **存储后端**：
   - 会话和审计日志默认使用 PostgreSQL（HA 模式）
   - 单实例模式仍可使用 SQLite
   - PostgreSQL 故障时自动降级到本地 SQLite

4. **alertd HA**：
   - alertd 以 3 副本部署，只有 Leader 处理告警
   - 避免多实例重复触发修复
```

---

## 59. Agent 自升级机制（v2.0 新增）

### 59.1 问题场景

v1.9 设计了 K8s 集群升级辅助（§48），但**Agent 自己如何升级**完全没有提及：

- Agent 作为长期运行的进程/DaemonSet，如何做到**零停机升级**？
- 升级过程中正在执行的会话如何处理？中断还是迁移？
- 版本回滚策略是什么？
- **实际运维场景**：v1.9.0 发现 bug，需要紧急升级到 v1.9.1，但正在处理 P0 事故的会话不能中断，否则修复操作半途而废

### 59.2 设计目标

- 滚动升级：先启动新版本 Pod，等待健康检查通过后切换流量，优雅关闭旧版本
- 会话迁移：正在进行的会话序列化到共享存储，新实例恢复
- 版本兼容性矩阵：API/配置/数据库 schema 的向前/向后兼容策略
- 自动回滚：升级后健康检查失败自动回滚到旧版本

### 59.3 Go 接口定义

```go
// SelfUpgradeManager Agent 自升级管理器
type SelfUpgradeManager struct {
    k8sClient      kubernetes.Interface
    currentVersion semver.Version
    targetVersion  semver.Version
    config         UpgradeConfig
}

// UpgradePlan 升级计划
type UpgradePlan struct {
    FromVersion     string
    ToVersion       string
    Strategy        UpgradeStrategy
    PreChecks       []PreCheck
    RollbackPlan    RollbackPlan
    EstimatedDowntime time.Duration
}

type UpgradeStrategy string

const (
    UpgradeStrategyRolling UpgradeStrategy = "rolling"    // 滚动升级（推荐）
    UpgradeStrategyBlueGreen UpgradeStrategy = "blue-green" // 蓝绿部署
    UpgradeStrategyCanary  UpgradeStrategy = "canary"     // 金丝雀（仅交互式）
)

// SessionMigration 会话迁移
type SessionMigration struct {
    sessionStore SessionStore
}

type MigratedSession struct {
    SessionID       string
    SourceInstance  string
    TargetInstance  string
    State           SessionState
    MigratedAt      time.Time
}

type SessionState struct {
    Context       []Message        // 对话历史
    ToolHistory   []ToolCallRecord // 工具调用记录
    PendingAction *PendingAction   // 待确认的操作
    Variables     map[string]interface{} // 会话变量
}

// VersionCompatibility 版本兼容性检查
type VersionCompatibility struct {
    FromVersion string
    ToVersion   string
    Compatible  bool
    BreakingChanges []BreakingChange
    MigrationSteps  []MigrationStep
}

type BreakingChange struct {
    Component   string  // api | config | database | tool-interface
    Description string
    Impact      string
    Mitigation  string
}
```

### 59.4 滚动升级流程

```
运维: "ops-ai upgrade self --version 2.0.1"

Agent:
  1. 检查版本兼容性
  2. 生成升级计划
  3. 迁移活跃会话
  4. 执行滚动升级

  ═══════════════════════════════════════════════════════
  ⬆️   Agent 自升级计划: v2.0.0 → v2.0.1
  ═══════════════════════════════════════════════════════

  兼容性检查
  ─────────────────────────────────────────────────────
  API 兼容性:        ✅ 向后兼容
  配置兼容性:        ✅ 无破坏性变更
  数据库 Schema:      ✅ 无需迁移
  工具接口:          ✅ 兼容

  升级策略: 滚动升级（Rolling）
  预估停机时间: 0s（会话热迁移）

  活跃会话: 3 个
  ─────────────────────────────────────────────────────
  sess-001  alice   payment-api OOM 排查      将自动迁移
  sess-002  bob     ingress 证书过期          将自动迁移
  sess-003  ci-cd   部署后健康检查            将自动迁移

  [S] 开始升级  [P] 预览详情  [Q] 取消
```

### 59.5 会话热迁移

```go
func (m *SessionMigration) MigrateSession(ctx context.Context, sessionID, targetInstance string) error {
    // 1. 获取会话当前状态
    sess, err := m.sessionStore.Load(sessionID)
    if err != nil {
        return err
    }
    
    // 2. 序列化会话状态
    state := SessionState{
        Context:     sess.Messages,
        ToolHistory: sess.ToolCalls,
        PendingAction: sess.PendingAction,
        Variables:   sess.Variables,
    }
    
    stateJSON, err := json.Marshal(state)
    if err != nil {
        return err
    }
    
    // 3. 写入共享存储（带 TTL，防止孤儿状态）
    err = m.sessionStore.SaveMigrationState(sessionID, targetInstance, stateJSON)
    if err != nil {
        return err
    }
    
    // 4. 标记原会话为"迁移中"
    sess.Status = SessionStatusMigrating
    m.sessionStore.Save(sess)
    
    // 5. 目标实例检测并恢复会话
    // 目标实例启动后会扫描共享存储中的迁移状态
    
    return nil
}

func (a *Agent) restoreMigratedSessions(ctx context.Context) {
    migrations, err := a.sessionStore.ListPendingMigrations(a.identity)
    if err != nil {
        log.Printf("列出待恢复会话失败: %v", err)
        return
    }
    
    for _, mig := range migrations {
        state, err := a.sessionStore.LoadMigrationState(mig.SessionID)
        if err != nil {
            continue
        }
        
        // 恢复会话
        session := a.createSessionFromState(mig.SessionID, state)
        
        // 通知用户
        a.tui.Notify(fmt.Sprintf(
            "会话 %s 已从 %s 迁移到当前实例，可继续操作。",
            mig.SessionID, mig.SourceInstance,
        ))
        
        // 清理迁移状态
        a.sessionStore.DeleteMigrationState(mig.SessionID)
    }
}
```

### 59.6 自动回滚

```go
func (u *SelfUpgradeManager) AutoRollback(ctx context.Context, reason string) error {
    // 1. 记录回滚原因
    u.recordRollback(reason)
    
    // 2. 回滚 Deployment 镜像
    deployment, err := u.k8sClient.AppsV1().Deployments(u.config.Namespace).Get(ctx, "ops-ai", metav1.GetOptions{})
    if err != nil {
        return err
    }
    
    deployment.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:%s", u.config.ImageRepo, u.currentVersion)
    
    _, err = u.k8sClient.AppsV1().Deployments(u.config.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
    if err != nil {
        return err
    }
    
    // 3. 通知
    u.notifier.Notify(fmt.Sprintf("Agent 自动回滚到 %s，原因: %s", u.currentVersion, reason))
    
    return nil
}
```

### 59.7 升级后健康检查

```go
func (u *SelfUpgradeManager) PostUpgradeHealthCheck(ctx context.Context) error {
    checks := []HealthCheck{
        {Name: "LLM API 连通性", Check: u.checkLLMConnectivity},
        {Name: "K8s API 连通性", Check: u.checkK8sConnectivity},
        {Name: "数据库连通性", Check: u.checkDatabaseConnectivity},
        {Name: "会话恢复状态", Check: u.checkSessionRestoration},
        {Name: "核心工具可用性", Check: u.checkCoreTools},
    }
    
    for _, check := range checks {
        ok, detail := check.Check(ctx)
        if !ok {
            return fmt.Errorf("升级后健康检查失败 [%s]: %s", check.Name, detail)
        }
    }
    
    return nil
}
```

### 59.8 L0 命令扩展

```
ops-ai version                          # 查看当前版本
ops-ai upgrade check                    # 检查是否有新版本
ops-ai upgrade self --version <ver>     # 升级 Agent 自身（L3）
ops-ai upgrade rollback                 # 回滚到上一个版本（L3）
ops-ai upgrade history                  # 查看升级历史
```

### 59.9 配置项

```yaml
# ~/.ops-ai/config.yaml
self_upgrade:
  enabled: true
  
  # 升级策略
  strategy: "rolling"                   # rolling | blue-green | canary
  image_repo: "registry.company.com/ops-ai"
  
  # 自动检查
  auto_check:
    enabled: true
    interval: "24h"                     # 每天检查一次新版本
    channel: "stable"                   # stable | beta | nightly
    
  # 会话迁移
  session_migration:
    enabled: true
    timeout: "30s"                      # 迁移超时时间
    max_concurrent: 10                  # 最大并发迁移数
    
  # 升级后验证
  post_upgrade:
    health_check: true
    health_check_timeout: "2m"
    auto_rollback: true                 # 健康检查失败自动回滚
    auto_rollback_on_error: true        # panic 自动回滚
    
  # 版本兼容性
  compatibility:
    min_supported_version: "1.9.0"      # 最低可升级版本
    breaking_change_action: "block"     # block | warn | allow
```

### 59.10 System Prompt 补充

```
## Agent 自升级知识

当运维询问 Agent 版本或升级相关问题时：

1. **升级检查**：
   - `ops-ai upgrade check` 检查是否有新版本
   - 支持 stable / beta / nightly 通道
   - 升级前强制检查版本兼容性

2. **滚动升级**：
   - 默认使用滚动升级策略
   - 活跃会话会自动热迁移到新实例
   - 升级过程中用户无感知（除了短暂的 Leader 切换）

3. **自动回滚**：
   - 升级后健康检查失败自动回滚
   - 新版本 panic 自动回滚
   - 回滚时会话再次热迁移

4. **版本兼容性**：
   - 不支持跨大版本升级（如 v1.x → v2.x 需手动）
   - 破坏性变更会阻止升级，直到运维确认
```

---

## 60. 大规模集群性能优化（v2.0 新增）

### 60.1 问题场景

v1.9 新增了大量扫描和分析功能（合规扫描 §51、证书扫描 §41、废弃 API 扫描 §48），但**在 1000+ 节点的集群上性能如何**没有设计：

- `ops-ai cert scan` 扫描全集群证书，在 5000+ Secret 的集群上可能跑几分钟
- CIS Benchmark 扫描全集群资源，没有分页/流式/增量设计
- 审计日志 SQLite 在高压场景下（100+ QPS）会成为瓶颈
- 影响面分析（§6）在大量关联资源的场景下计算复杂度爆炸
- **实际运维场景**：大型电商平台 50+ namespace、2000+ Pod、300+ Node，Agent 一个扫描命令卡死 10 分钟，运维以为 Agent 挂了

### 60.2 设计目标

- 所有全量扫描支持分页、流式、增量更新
- 扫描结果缓存和 TTL，避免重复计算
- 审计日志支持异步批量写入 + 外部数据库后端
- 影响面分析算法优化，限制关联深度和广度
- 并发控制，避免同时启动多个重扫描压垮 API Server

### 60.3 Go 接口定义

```go
// PerformanceOptimizer 性能优化器
type PerformanceOptimizer struct {
    k8sClient     kubernetes.Interface
    cache         *ScanResultCache
    rateLimiter   *APIServerRateLimiter
    config        PerformanceConfig
}

// ScanResultCache 扫描结果缓存
type ScanResultCache struct {
    store map[string]*CachedScan
    mu    sync.RWMutex
}

type CachedScan struct {
    Key       string
    Result    interface{}
    CreatedAt time.Time
    TTL       time.Duration
    Version   string  // 基于 resourceVersion 的增量标记
}

func (c *ScanResultCache) Get(key string) (*CachedScan, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    cached, ok := c.store[key]
    if !ok {
        return nil, false
    }
    
    if time.Since(cached.CreatedAt) > cached.TTL {
        return nil, false // 已过期
    }
    
    return cached, true
}

// APIServerRateLimiter API Server 限流器
type APIServerRateLimiter struct {
    qps       float64
    burst     int
    limiter   *rate.Limiter
}

// IncrementalScanner 增量扫描器
type IncrementalScanner struct {
    k8sClient     kubernetes.Interface
    lastResourceVersion string
}

func (s *IncrementalScanner) ScanDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
    // 使用 resourceVersion 进行增量获取
    opts := metav1.ListOptions{
        ResourceVersion: s.lastResourceVersion,
    }
    
    list, err := s.k8sClient.AppsV1().Deployments(namespace).List(ctx, opts)
    if err != nil {
        return nil, err
    }
    
    s.lastResourceVersion = list.ResourceVersion
    return list.Items, nil
}

// PaginatedLister 分页列表器
type PaginatedLister struct {
    pageSize int32
}

func (l *PaginatedLister) ListAllPods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
    var allPods []corev1.Pod
    var continueToken string
    
    for {
        list, err := l.k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
            Limit:    l.pageSize,
            Continue: continueToken,
        })
        if err != nil {
            return nil, err
        }
        
        allPods = append(allPods, list.Items...)
        
        if list.Continue == "" {
            break
        }
        continueToken = list.Continue
    }
    
    return allPods, nil
}

// ImpactAnalysisLimiter 影响面分析限制器
type ImpactAnalysisLimiter struct {
    maxDepth       int  // 最大关联深度
    maxBreadth     int  // 每层最大关联数
    maxTotalNodes  int  // 最大总节点数
}
```

### 60.4 扫描结果缓存机制

```go
func (o *PerformanceOptimizer) CachedScan(ctx context.Context, scanType string, scanner func() (interface{}, error)) (interface{}, error) {
    cacheKey := fmt.Sprintf("%s:%s", scanType, o.getClusterFingerprint())
    
    // 1. 检查缓存
    if cached, ok := o.cache.Get(cacheKey); ok {
        return cached.Result, nil
    }
    
    // 2. 限流：检查是否已有相同扫描在进行中
    if o.isScanInProgress(cacheKey) {
        return nil, fmt.Errorf("相同扫描正在进行中，请等待完成后查看结果")
    }
    
    // 3. 执行扫描
    o.markScanInProgress(cacheKey)
    defer o.markScanDone(cacheKey)
    
    result, err := scanner()
    if err != nil {
        return nil, err
    }
    
    // 4. 写入缓存
    o.cache.Set(cacheKey, &CachedScan{
        Key:       cacheKey,
        Result:    result,
        CreatedAt: time.Now(),
        TTL:       o.config.CacheTTL,
    })
    
    return result, nil
}
```

### 60.5 TUI 扫描进度展示

```
  ═══════════════════════════════════════════════════════
  🔍  全集群证书扫描（大规模模式）
  ═══════════════════════════════════════════════════════

  集群规模: 2,847 Pods / 312 Nodes / 56 Namespaces
  
  扫描进度
  ─────────────────────────────────────────────────────
  [████████████████████░░░░░░░░░░░░░░░░]  54%
  
  已处理:     1,537 / 2,847 Secrets
  当前 Namespace: payment
  预计剩余:   2m 15s
  
  性能指标
  ─────────────────────────────────────────────────────
  API Server QPS:    8.5 / 20 (限流保护)
  并发协程:          4
  内存占用:          124MB
  
  [C] 取消扫描  [P] 后台运行（结果推送到 Slack）
```

### 60.6 影响面分析算法优化

```go
func (a *ImpactAnalyzer) AnalyzeWithLimit(ctx context.Context, resource ResourceRef, limiter ImpactAnalysisLimiter) (*ImpactResult, error) {
    result := &ImpactResult{}
    visited := make(map[string]bool)
    queue := []ResourceRef{resource}
    depth := 0
    totalNodes := 0
    
    for len(queue) > 0 && depth < limiter.maxDepth && totalNodes < limiter.maxTotalNodes {
        levelSize := len(queue)
        nextLevel := []ResourceRef{}
        
        for i := 0; i < levelSize && i < limiter.maxBreadth; i++ {
            current := queue[i]
            key := fmt.Sprintf("%s/%s/%s", current.Namespace, current.Kind, current.Name)
            
            if visited[key] {
                continue
            }
            visited[key] = true
            totalNodes++
            
            // 分析当前资源的关联
            related := a.findDirectRelated(ctx, current)
            
            // 限制每层关联数
            if len(related) > limiter.maxBreadth {
                related = related[:limiter.maxBreadth]
                result.Truncated = true
            }
            
            result.Related = append(result.Related, related...)
            nextLevel = append(nextLevel, related...)
        }
        
        queue = nextLevel
        depth++
    }
    
    if totalNodes >= limiter.maxTotalNodes {
        result.Truncated = true
        result.TruncationReason = fmt.Sprintf("影响面超过 %d 个节点，已截断。可能存在大规模级联影响。", limiter.maxTotalNodes)
    }
    
    return result, nil
}
```

### 60.7 审计日志批量写入

```go
func (a *AuditLogger) BatchWrite(entries []AuditEntry) error {
    // 1. 收集到缓冲区
    a.batchBuffer = append(a.batchBuffer, entries...)
    
    // 2. 达到批量阈值或超时后写入
    if len(a.batchBuffer) >= a.config.BatchSize {
        return a.flush()
    }
    
    // 3. 定时 flush（避免数据滞留）
    if a.flushTimer == nil {
        a.flushTimer = time.AfterFunc(a.config.FlushInterval, func() {
            a.flush()
        })
    }
    
    return nil
}

func (a *AuditLogger) flush() error {
    if len(a.batchBuffer) == 0 {
        return nil
    }
    
    // 批量写入 PostgreSQL
    err := a.db.BulkInsert(a.batchBuffer)
    if err != nil {
        // 降级到单条写入 SQLite
        for _, entry := range a.batchBuffer {
            a.fallback.Write(entry)
        }
    }
    
    a.batchBuffer = a.batchBuffer[:0]
    if a.flushTimer != nil {
        a.flushTimer.Stop()
        a.flushTimer = nil
    }
    
    return nil
}
```

### 60.8 配置项

```yaml
# ~/.ops-ai/config.yaml
performance:
  enabled: true
  
  # 扫描缓存
  cache:
    enabled: true
    ttl: "5m"                           # 扫描结果缓存 5 分钟
    max_entries: 100                    # 最多缓存 100 个扫描结果
    
  # API Server 限流
  rate_limit:
    qps: 20                             # 每秒最大请求数
    burst: 50                           # 突发请求数
    
  # 分页
  pagination:
    default_page_size: 200              # 默认分页大小
    max_page_size: 500                  # 最大分页大小
    
  # 影响面分析限制
  impact_analysis:
    max_depth: 3                        # 最大关联深度
    max_breadth_per_level: 20           # 每层最大关联数
    max_total_nodes: 100                # 最大总节点数
    
  # 审计日志批量写入
  audit_batch:
    enabled: true
    batch_size: 100                     # 每批 100 条
    flush_interval: "5s"                # 最长 5 秒 flush 一次
    
  # 大规模集群检测
  large_cluster:
    threshold_nodes: 500                # 超过 500 节点触发大规模模式
    threshold_pods: 5000                # 超过 5000 Pod 触发大规模模式
    auto_throttle: true                 # 自动降低并发和 QPS
```

### 60.9 System Prompt 补充

```
## 大规模集群性能知识

当运维在大型集群（>500 节点）上使用 Agent 时：

1. **自动降级**：
   - 超过阈值节点数时自动进入大规模模式
   - 自动降低并发和 API Server QPS
   - 扫描结果自动缓存，避免重复计算

2. **分页和流式**：
   - 所有全量扫描支持分页和后台运行
   - 大扫描结果可以推送到 Slack/文件而非终端展示

3. **影响面分析限制**：
   - 默认限制关联深度 3 层、每层 20 个节点
   - 如果截断，会提示"可能存在大规模级联影响"
   - 可以手动放宽限制，但需要确认

4. **审计日志**：
   - 大型集群建议配置 PostgreSQL 后端
   - 审计日志批量写入，减少数据库压力
```

---

# 第四部分：P1 — 重要级（规模化运维必需）

---

## 61. 服务网格（Service Mesh）深度运维（v2.0 新增）

### 61.1 问题场景

v1.9 全文**零匹配** "Istio/Linkerd/service mesh"。这在现代 K8s 生产环境中是不可接受的：

- 50% 以上的中大型 K8s 集群运行 Istio 或 Linkerd
- 服务网格故障是生产 P0 的高频来源：mTLS 握手失败、Sidecar 注入异常、VirtualService 路由错误、DestinationRule 负载均衡异常、Envoy 配置漂移
- v1.8 的 §38 网络诊断只覆盖到 NetworkPolicy/Pod 级别，**没有深入到 Sidecar/Envoy 层面**
- **实际运维场景**："payment-api 调用 order-service 返回 503，kubectl get pods 显示全部 Running，但 istioctl proxy-config 显示 Envoy 路由指向了已删除的 Pod"

### 61.2 设计目标

- Istio / Linkerd 控制平面健康检查
- Sidecar 诊断：Envoy 配置、证书状态、流量统计
- VirtualService / DestinationRule 冲突检测
- 服务网格流量拓扑可视化
- mTLS 故障诊断

### 61.3 Go 接口定义

```go
// ServiceMeshManager 服务网格管理器
type ServiceMeshManager struct {
    k8sClient     kubernetes.Interface
    istioClient   istiov1beta1.Interface
    linkerdClient linkerdv1alpha1.Interface
    config        MeshConfig
}

// MeshControlPlaneChecker 控制平面检查器
type MeshControlPlaneChecker struct {
    k8sClient kubernetes.Interface
}

type MeshControlPlaneStatus struct {
    Provider      string  // istio | linkerd
    Version       string
    Components    []MeshComponent
    OverallHealth string  // healthy | degraded | unhealthy
}

type MeshComponent struct {
    Name      string
    Namespace string
    Ready     bool
    Replicas  int32
    Issues    []string
}

// SidecarDiagnoser Sidecar 诊断器
type SidecarDiagnoser struct {
    k8sClient kubernetes.Interface
    execTool  *KubectlExecTool
}

type SidecarDiagnosis struct {
    PodName       string
    Namespace     string
    ProxyVersion  string
    ConfigStatus  string  // SYNCED | NOT_SENT | STALE
    CertStatus    CertStatus
    ListenerCount int
    ClusterCount  int
    RouteCount    int
    Issues        []SidecarIssue
}

type CertStatus struct {
    SAN       string
    NotAfter  time.Time
    DaysLeft  int
    Valid     bool
}

type SidecarIssue struct {
    Type        string  // config-stale | cert-expired | listener-mismatch | cluster-unhealthy
    Severity    string
    Description string
    Suggestion  string
}

// VirtualServiceChecker VirtualService 检查器
type VirtualServiceChecker struct {
    istioClient istiov1beta1.Interface
}

type VSCheckResult struct {
    Name        string
    Namespace   string
    Gateways    []string
    Hosts       []string
    Conflicts   []VSConflict
    Issues      []string
}

type VSConflict struct {
    WithVS      string
    Type        string  // host-overlap | gateway-overlap
    Description string
}

// MeshTrafficVisualizer 流量拓扑可视化
type MeshTrafficVisualizer struct {
    promClient promv1.API
}

type TrafficTopology struct {
    Nodes []TrafficNode
    Edges []TrafficEdge
}

type TrafficNode struct {
    Service   string
    Namespace string
    Requests  float64
    Errors    float64
    Latency   float64
}

type TrafficEdge struct {
    Source    string
    Target    string
    Protocol  string
    mTLS      bool
    RPS       float64
    ErrorRate float64
}
```

### 61.4 控制平面健康检查

```
ops-ai mesh status
ops-ai mesh status --provider istio
ops-ai mesh status --provider linkerd
```

```
  ═══════════════════════════════════════════════════════
  🕸️  服务网格状态 — Istio v1.21.0
  ═══════════════════════════════════════════════════════

  控制平面: 🟢 健康

  组件状态
  ─────────────────────────────────────────────────────
  istiod          istio-system   2/2 ✅   
  ingressgateway  istio-system   3/3 ✅
  egressgateway   istio-system   1/1 ✅
  
  数据平面
  ─────────────────────────────────────────────────────
  Sidecar 注入:   已启用（默认 namespace: payment, order）
  已注入 Pod:     234 / 234 ✅
  配置同步状态:    100% SYNCED
  
  证书
  ─────────────────────────────────────────────────────
  根 CA 有效期:    364 天
  工作负载证书 TTL: 24h（自动轮换）

  [D] 诊断详情  [Q] 返回
```

### 61.5 Sidecar 诊断

```
ops-ai mesh sidecar diagnose <pod-name> -n <namespace>
```

```
  ═══════════════════════════════════════════════════════
  🕸️  Sidecar 诊断 — payment-api-7d8f9
  ═══════════════════════════════════════════════════════

  Envoy 代理: istio-proxy:v1.21.0
  配置状态:    ✅ SYNCED (istiod: istiod-abc123)
  
  证书状态
  ─────────────────────────────────────────────────────
  SAN:      spiffe://cluster.local/ns/payment/sa/payment-api
  有效期:    22h / 24h ✅
  自动轮换:   已启用

  监听器 (Listeners)
  ─────────────────────────────────────────────────────
  0.0.0.0:8080    HTTP   inbound  ✅
  0.0.0.0:15001   TCP    virtual  ✅
  0.0.0.0:15006   TCP    virtual  ✅
  0.0.0.0:15090   HTTP   prometheus ✅

  集群 (Clusters)
  ─────────────────────────────────────────────────────
  outbound|5432||payment-db.payment.svc.cluster.local     ✅ HEALTHY
  outbound|6379||redis-cache.order.svc.cluster.local      ⚠️  STALE (上次更新: 15m 前)
  outbound|9092||kafka.kafka.svc.cluster.local            ✅ HEALTHY

  ⚠️  发现 1 个问题:
  redis-cache cluster 状态为 STALE，可能原因:
  1. redis-cache Service Endpoints 最近有变更，Envoy 配置未同步
  2. istiod 与该 Pod 的 xDS 连接异常

  建议: 检查 istiod 日志，或尝试重启该 Pod 的 sidecar

  [L] 查看 istiod 日志  [R] 重启 sidecar  [Q] 返回
```

### 61.6 VirtualService / DestinationRule 冲突检测

```go
func (c *VirtualServiceChecker) CheckConflicts(ctx context.Context, namespace string) ([]VSConflict, error) {
    vss, err := c.istioClient.NetworkingV1beta1().VirtualServices(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, err
    }
    
    var conflicts []VSConflict
    
    // 检测主机重叠
    hostMap := make(map[string][]string)
    for _, vs := range vss.Items {
        for _, host := range vs.Spec.Hosts {
            hostMap[host] = append(hostMap[host], vs.Name)
        }
    }
    
    for host, vss := range hostMap {
        if len(vss) > 1 {
            conflicts = append(conflicts, VSConflict{
                Type:        "host-overlap",
                Description: fmt.Sprintf("主机 %s 被多个 VirtualService 定义: %v", host, vss),
            })
        }
    }
    
    return conflicts, nil
}
```

### 61.7 流量拓扑可视化

```
ops-ai mesh traffic --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  🕸️  流量拓扑 — payment namespace
  ═══════════════════════════════════════════════════════

  ingress-gateway → payment-api (HTTPS/mTLS)
                         │
                         ├─→ payment-db     (TCP/mTLS)  RPS: 45  P99: 12ms  ✅
                         ├─→ redis-cache    (TCP/mTLS)  RPS: 120 P99: 2ms   ✅
                         └─→ order-service  (HTTP/mTLS)  RPS: 30  P99: 45ms  ⚠️
                                                              错误率: 3.2%

  ⚠️  order-service 错误率 3.2% 高于阈值 1%
  可能原因:
  1. order-service 版本 v2.1.0 刚发布，可能存在 bug
  2. order-service → order-db 的连接池配置不合理

  [D] 诊断 order-service  [Q] 返回
```

### 61.8 mTLS 故障诊断

```go
func (d *SidecarDiagnoser) DiagnosemTLS(ctx context.Context, sourcePod, targetPod string) (*mTLSDiagnosis, error) {
    // 1. 检查源 Pod 的证书
    sourceCert, err := d.getCertStatus(ctx, sourcePod)
    
    // 2. 检查目标 Pod 的证书
    targetCert, err := d.getCertStatus(ctx, targetPod)
    
    // 3. 检查 PeerAuthentication 策略
    pa, err := d.getPeerAuthentication(ctx, targetPod)
    
    // 4. 检查 DestinationRule 的 TLS 配置
    dr, err := d.getDestinationRule(ctx, targetPod)
    
    return &mTLSDiagnosis{
        SourceCertValid: sourceCert.Valid,
        TargetCertValid: targetCert.Valid,
        mTLSMode:        pa.Spec.Mtls.Mode.String(),
        DRTLSMode:       dr.Spec.TrafficPolicy.Tls.Mode.String(),
        Issues:          d.checkmTLSMismatch(sourceCert, targetCert, pa, dr),
    }, nil
}
```

### 61.9 L0 命令扩展

```
ops-ai mesh status                        # 服务网格控制平面状态
ops-ai mesh sidecar diagnose <pod>        # Sidecar 诊断
ops-ai mesh sidecar config <pod>          # 查看 Envoy 配置
ops-ai mesh vs check [-n <ns>]            # VirtualService 冲突检测
ops-ai mesh traffic [--namespace <ns>]    # 流量拓扑可视化
ops-ai mesh mtls check <src> <dst>        # mTLS 诊断
ops-ai mesh cert status                   # 证书状态
```

### 61.10 配置项

```yaml
# ~/.ops-ai/config.yaml
service_mesh:
  enabled: true
  
  # 自动检测
  auto_detect: true                     # 自动检测集群中的服务网格
  
  # Istio 配置
  istio:
    namespace: "istio-system"
    istiod_label: "app=istiod"
    ingressgateway_label: "app=istio-ingressgateway"
    
  # Linkerd 配置
  linkerd:
    namespace: "linkerd"
    
  # 诊断
  sidecar_diagnosis:
    exec_timeout: "30s"
    config_dump_max_size: "10MB"
    
  # 流量分析
  traffic_analysis:
    prometheus_metrics:
      - "istio_requests_total"
      - "istio_request_duration_seconds"
      - "istio_tcp_sent_bytes_total"
```

### 61.11 System Prompt 补充

```
## 服务网格运维知识

当运维询问服务网格相关问题时：

1. **控制平面**：
   - 先检查 istiod/linkerd-controller 是否健康
   - 数据平面 100% SYNCED 是健康的标志
   - 如果有 Pod 显示 STALE，通常是 istiod 连接问题

2. **Sidecar 诊断**：
   - Envoy 配置状态: SYNCED | NOT_SENT | STALE
   - 证书有效期通常 24h，自动轮换
   - 集群状态 STALE 意味着 Endpoint 变更未同步

3. **mTLS 故障**：
   - 常见原因: 证书过期、PeerAuthentication 策略冲突、DestinationRule TLS 模式不匹配
   - 检查源和目标 Pod 的证书 SAN 是否匹配

4. **VirtualService 冲突**：
   - 同一主机被多个 VS 定义会导致路由不可预测
   - 建议每个主机只有一个 VS

5. **流量分析**：
   - 通过 Prometheus Istio 指标分析服务间流量
   - 错误率和延迟突增是发布问题的早期信号
```

---

## 62. 发布策略辅助 — 金丝雀/灰度/蓝绿部署（v2.0 新增）

### 62.1 问题场景

v1.9 新增了很多运维能力，但**发布管理**这个 SRE 最核心的场景几乎没有涉及：

- Argo Rollouts、Flagger 是 K8s 生态主流的金丝雀发布工具
- 发布过程中的渐进式流量切换、指标验证、自动回滚需要 Agent 辅助
- **实际运维场景**："帮我将 payment-api 从 v2.3.1 灰度发布到 v2.4.0，5% 流量起，每 10 分钟倍增，错误率 > 1% 自动回滚" — Agent 完全无法处理
- 手工发布的风险：运维容易在流量切换步骤出错，或在指标恶化时反应不及时

### 62.2 设计目标

- Argo Rollouts / Flagger 集成：作为 L3 工具，支持创建/监控/回滚渐进式发布
- 发布健康检查：自动对比发布前后的错误率、延迟、吞吐量
- 发布失败自动回滚：与 §7 回滚策略联动
- 发布 Runbook 生成：根据服务特征推荐最佳发布策略

### 62.3 Go 接口定义

```go
// ReleaseManager 发布管理器
type ReleaseManager struct {
    k8sClient      kubernetes.Interface
    argoClient     argorollouts.Interface
    flaggerClient  flaggerv1beta1.Interface
    config         ReleaseConfig
}

// RolloutStrategy 发布策略
type RolloutStrategy struct {
    Type        string  // canary | blue-green | ab-testing
    Steps       []RolloutStep
    Analysis    *CanaryAnalysis
    AutoRollback bool
}

type RolloutStep struct {
    SetWeight   int32             // 流量百分比
    Pause       *RolloutPause     // 暂停配置
    SetCanaryScale *CanaryScale   // Canary 副本数
}

type RolloutPause struct {
    Duration    string  // 如 "10m"
    UntilApproved bool  // 等待人工审批
}

type CanaryScale struct {
    Weight      int32
    Replicas    int32
}

// CanaryAnalysis 金丝雀分析
type CanaryAnalysis struct {
    Threshold  int32   // 允许的最大失败次数
    Interval   string  // 分析间隔
    SuccessfulRunHistoryLimit int32
    Metrics    []AnalysisMetric
}

type AnalysisMetric struct {
    Name      string
    Interval  string
    ThresholdRange *ThresholdRange
}

type ThresholdRange struct {
    Min *float64
    Max *float64
}

// ReleaseHealthChecker 发布健康检查器
type ReleaseHealthChecker struct {
    promClient promv1.API
}

type ReleaseHealthReport struct {
    Baseline    MetricSnapshot
    Canary      MetricSnapshot
    Comparison  MetricComparison
    Passed      bool
    Issues      []string
}

type MetricComparison struct {
    ErrorRateDelta    float64
    LatencyP99Delta   float64
    ThroughputDelta   float64
}

// RollbackTrigger 自动回滚触发器
type RollbackTrigger struct {
    conditions []RollbackCondition
}

type RollbackCondition struct {
    Metric    string
    Threshold float64
    Duration  time.Duration
}
```

### 62.4 金丝雀发布创建（L3 工具）

```
ops-ai release create --deployment payment-api --image registry.company.com/payment-api:v2.4.0 --strategy canary
```

交互式配置：

```
  ═══════════════════════════════════════════════════════
  🚀  创建金丝雀发布 — payment-api
  ═══════════════════════════════════════════════════════

  当前版本: v2.3.1
  目标版本: v2.4.0

  发布策略选择
  ─────────────────────────────────────────────────────
  [1] 金丝雀 (Canary)      — 渐进式流量切换，推荐
  [2] 蓝绿 (Blue-Green)    — 全量切换，零停机
  [3] A/B 测试              — 基于 Header 的流量分割

  选择: 1

  金丝雀配置
  ─────────────────────────────────────────────────────
  步骤 1: 5%  流量 → 暂停 10 分钟
  步骤 2: 25% 流量 → 暂停 10 分钟
  步骤 3: 50% 流量 → 暂停 10 分钟
  步骤 4: 100% 流量

  自动回滚条件
  ─────────────────────────────────────────────────────
  错误率 > 1% 持续 2 分钟  → 自动回滚
  P99 延迟 > 500ms 持续 2 分钟 → 自动回滚

  [C] 创建 Rollout  [M] 修改配置  [Q] 取消
```

### 62.5 发布健康检查

```go
func (c *ReleaseHealthChecker) Check(ctx context.Context, rollout *argorollouts.Rollout) (*ReleaseHealthReport, error) {
    report := &ReleaseHealthReport{}
    
    // 1. 获取基线指标（稳定版本）
    baselineSelector := fmt.Sprintf("app=%s,rollouts-pod-template-hash=%s", 
        rollout.Spec.Selector.MatchLabels["app"], 
        rollout.Status.StableRS)
    report.Baseline = c.queryMetrics(ctx, baselineSelector)
    
    // 2. 获取 Canary 指标
    canarySelector := fmt.Sprintf("app=%s,rollouts-pod-template-hash=%s",
        rollout.Spec.Selector.MatchLabels["app"],
        rollout.Status.CanaryRS)
    report.Canary = c.queryMetrics(ctx, canarySelector)
    
    // 3. 对比
    report.Comparison = MetricComparison{
        ErrorRateDelta:  report.Canary.ErrorRate - report.Baseline.ErrorRate,
        LatencyP99Delta: report.Canary.LatencyP99 - report.Baseline.LatencyP99,
        ThroughputDelta: report.Canary.Throughput - report.Baseline.Throughput,
    }
    
    // 4. 评估
    report.Passed = true
    if report.Comparison.ErrorRateDelta > 0.01 { // 1%
        report.Passed = false
        report.Issues = append(report.Issues, 
            fmt.Sprintf("错误率上升 %.2f%%", report.Comparison.ErrorRateDelta*100))
    }
    if report.Comparison.LatencyP99Delta > 100 { // 100ms
        report.Passed = false
        report.Issues = append(report.Issues,
            fmt.Sprintf("P99 延迟上升 %.0fms", report.Comparison.LatencyP99Delta))
    }
    
    return report, nil
}
```

### 62.6 发布监控 TUI

```
  ═══════════════════════════════════════════════════════
  🚀  金丝雀发布监控 — payment-api
  ═══════════════════════════════════════════════════════

  状态: 🟡 进行中 (步骤 2/4)
  
  版本对比
  ─────────────────────────────────────────────────────
  指标            基线 (v2.3.1)   Canary (v2.4.0)   变化
  ─────────────────────────────────────────────────────
  错误率           0.1%           0.15%            +0.05% 🟢
  P99 延迟         45ms           48ms             +3ms   🟢
  吞吐量           1,200 RPS      1,180 RPS        -1.7%  🟢
  
  流量分布
  ─────────────────────────────────────────────────────
  稳定版本:  ████████████████████████████████████ 75%
  Canary:    ██████████ 25%
  
  进度
  ─────────────────────────────────────────────────────
  [✓] 5%   流量 (10m)
  [→] 25%  流量 (剩余 6m)
  [ ] 50%  流量
  [ ] 100% 流量

  [A] 中止并回滚  [P] 暂停  [S] 跳过当前步骤  [Q] 返回
```

### 62.7 自动回滚

```go
func (r *RollbackTrigger) Evaluate(ctx context.Context, report *ReleaseHealthReport) (bool, string) {
    for _, cond := range r.conditions {
        value := report.GetMetric(cond.Metric)
        if value > cond.Threshold {
            // 检查持续时间
            if r.durationExceeded(cond, ctx) {
                return true, fmt.Sprintf("%s %.2f > %.2f 持续 %s", 
                    cond.Metric, value, cond.Threshold, cond.Duration)
            }
        }
    }
    return false, ""
}

func (m *ReleaseManager) AutoRollback(ctx context.Context, rolloutName, namespace, reason string) error {
    // 1. 触发 Argo Rollouts 回滚
    rollout, err := m.argoClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, rolloutName, metav1.GetOptions{})
    if err != nil {
        return err
    }
    
    rollout.Spec.RollbackWindow = &int32(10) // 保留 10 个修订版用于回滚
    _, err = m.argoClient.ArgoprojV1alpha1().Rollouts(namespace).Update(ctx, rollout, metav1.UpdateOptions{})
    
    // 2. 通知
    m.notifier.Notify(fmt.Sprintf("发布 %s 自动回滚，原因: %s", rolloutName, reason))
    
    return err
}
```

### 62.8 发布策略推荐

```
运维: "为 payment-api 推荐发布策略"

Agent:
  分析服务特征:
  - 流量模式: 电商支付，高峰时段流量大
  - 依赖: payment-db, redis-cache, order-service
  - 历史发布问题: v2.2.0 发布时数据库连接池配置错误导致 5 分钟故障
  - 当前 HPA: 3-10 replicas
  - PDB: minAvailable: 2

  推荐策略: 金丝雀 (Canary)
  原因:
  - 支付服务高风险，需要渐进式验证
  - 有历史发布故障记录，需要谨慎
  - 当前有 HPA 和 PDB，支持金丝雀副本弹性伸缩

  配置建议:
  - 步骤: 5% → 25% → 50% → 100%
  - 每步暂停: 10 分钟
  - 自动回滚: 错误率 > 0.5% 或 P99 > 100ms
```

### 62.9 L0 命令扩展

```
ops-ai release list                       # 查看进行中的发布
ops-ai release status <name>              # 查看发布状态
ops-ai release health <name>              # 发布健康检查（L0）
ops-ai release create ...                 # 创建发布（L3）
ops-ai release promote <name>             # 手动推进到下一步（L3）
ops-ai release abort <name>               # 中止发布并回滚（L3）
ops-ai release recommend --deployment <d> # 推荐发布策略（L0）
```

### 62.10 配置项

```yaml
# ~/.ops-ai/config.yaml
release_management:
  enabled: true
  
  # 支持的发布工具
  providers:
    - "argo-rollouts"
    - "flagger"
    
  # 默认金丝雀配置
  default_canary:
    steps:
      - setWeight: 5
        pause: { duration: "10m" }
      - setWeight: 25
        pause: { duration: "10m" }
      - setWeight: 50
        pause: { duration: "10m" }
      - setWeight: 100
    analysis:
      threshold: 5
      interval: "1m"
      metrics:
        - name: error-rate
          thresholdRange: { max: 0.01 }
        - name: latency-p99
          thresholdRange: { max: 0.5 }
          
  # 自动回滚
  auto_rollback:
    enabled: true
    conditions:
      - metric: "error-rate"
        threshold: 0.01
        duration: "2m"
      - metric: "latency-p99"
        threshold: 0.5
        duration: "2m"
        
  # 审批
  approval:
    require_approval_for_full_promotion: true  # 100% 流量需要审批
```

### 62.11 System Prompt 补充

```
## 发布策略知识

当运维询问发布相关问题时：

1. **策略选择**：
   - 金丝雀: 适用于大多数服务，渐进式验证
   - 蓝绿: 适用于需要全量切换、快速回滚的场景
   - A/B 测试: 适用于功能验证，基于 Header 路由

2. **健康检查**：
   - 对比基线（稳定版本）和 Canary 的指标
   - 关注错误率、延迟、吞吐量三大指标
   - 基线对比比绝对阈值更可靠

3. **自动回滚**：
   - 默认启用自动回滚
   - 回滚条件可配置
   - 回滚后通知相关团队

4. **发布推荐**：
   - 基于服务特征（流量模式、依赖、历史问题）推荐策略
   - 高风险服务建议更保守的发布步骤
```

---

## 63. 变更窗口与维护模式（v2.0 新增）

### 63.1 问题场景

生产运维中，**不是所有时间都能随意操作**。v1.9 没有任何变更窗口（Change Window）或维护模式（Maintenance Mode）的设计：

- 银行/金融系统通常只允许在 02:00-06:00 做变更
- 电商系统在双 11 大促期间完全冻结变更
- Agent 的告警自动修复（§26）在冻结期不应该触发
- **实际运维场景**：双 11 期间 Agent 自动修复了一个"假告警"，导致服务重启，引发生产事故

### 63.2 设计目标

- 变更窗口配置：允许/禁止操作的时间段
- 维护模式：一键冻结所有变更操作
- 冻结期的 Agent 行为：告警只通知不修复、拒绝所有 L2+ 操作
- 变更日历集成：与 PagerDuty/OpsGenie 等 on-call 系统集成

### 63.3 Go 接口定义

```go
// ChangeWindowManager 变更窗口管理器
type ChangeWindowManager struct {
    store     ChangeWindowStore
    calendar  OnCallCalendarIntegration
    config    ChangeWindowConfig
}

// ChangeWindow 变更窗口
type ChangeWindow struct {
    ID          string
    Name        string
    Schedule    ChangeWindowSchedule
    Recurrence  string  // daily | weekly | monthly | once
    Timezone    string
    AllowedActions []ActionLevel  // 允许的操作级别
    Frozen      bool
}

type ChangeWindowSchedule struct {
    StartTime string  // "02:00"
    EndTime   string  // "06:00"
    Days      []string  // ["mon", "tue", "wed", "thu", "fri"]
    StartDate *time.Time
    EndDate   *time.Time
}

// MaintenanceMode 维护模式
type MaintenanceMode struct {
    Enabled     bool
    Reason      string
    EnabledBy   string
    EnabledAt   time.Time
    ExpiresAt   *time.Time
    Scope       MaintenanceScope
}

type MaintenanceScope struct {
    Namespaces  []string  // 空表示全集群
    Clusters    []string
    Exceptions  []string  // 例外规则
}

// OperationGate 操作门禁
type OperationGate struct {
    windowMgr *ChangeWindowManager
}

func (g *OperationGate) CanExecute(action ActionLevel, resource ResourceRef) (bool, string) {
    // 1. 检查维护模式
    if g.windowMgr.IsMaintenanceMode(resource) {
        return false, "当前处于维护模式，禁止所有变更操作"
    }
    
    // 2. 检查变更窗口
    if !g.windowMgr.IsInChangeWindow(time.Now()) {
        if action >= ActionLevelL2 {
            return false, fmt.Sprintf("当前不在变更窗口内（允许时间: %s），L2+ 操作被拒绝", 
                g.windowMgr.NextChangeWindow())
        }
    }
    
    // 3. 检查 on-call 状态
    if action >= ActionLevelL3 && !g.windowMgr.IsOnCallActive() {
        return false, "L3+ 操作需要当前有活跃的 on-call 人员"
    }
    
    return true, ""
}

// OnCallCalendarIntegration on-call 日历集成
type OnCallCalendarIntegration interface {
    IsOnCall(userID string) (bool, error)
    GetCurrentOnCall() ([]string, error)
}
```

### 63.4 维护模式操作

```
ops-ai maintenance on --reason "双11大促" --until "2024-11-12 02:00" --namespace payment
ops-ai maintenance on --reason "数据库迁移" --cluster prod
ops-ai maintenance status
ops-ai maintenance off
```

```
  ═══════════════════════════════════════════════════════
  🔒  维护模式已启用
  ═══════════════════════════════════════════════════════

  启用者:    alice
  时间:      2024-11-11 00:00:00
  预计解除:   2024-11-12 02:00:00
  原因:      双11大促

  影响范围
  ─────────────────────────────────────────────────────
  集群:     prod
  Namespace: payment, order, inventory

  限制行为
  ─────────────────────────────────────────────────────
  ❌ L2+ 操作（apply, delete, scale, patch）
  ❌ 告警自动修复
  ❌ 定时任务执行
  ✅ L0 诊断（get, describe, logs）
  ✅ 告警通知（不修复）

  [O] 解除维护模式（需要双人确认）  [Q] 返回
```

### 63.5 变更窗口配置

```yaml
# ~/.ops-ai/config.yaml
change_windows:
  - name: "daily-maintenance"
    schedule:
      start_time: "02:00"
      end_time: "06:00"
      days: ["mon", "tue", "wed", "thu", "fri"]
    timezone: "Asia/Shanghai"
    allowed_actions: ["L0", "L1", "L2", "L3", "L4"]
    
  - name: "weekend-restricted"
    schedule:
      start_time: "02:00"
      end_time: "04:00"
      days: ["sat", "sun"]
    timezone: "Asia/Shanghai"
    allowed_actions: ["L0", "L1", "L2"]  # 周末只允许 L2 及以下
```

### 63.6 操作门禁拦截

```go
func (a *Agent) ExecuteWithGate(ctx context.Context, action ActionLevel, resource ResourceRef, operation func() error) error {
    // 1. 检查操作门禁
    allowed, reason := a.operationGate.CanExecute(action, resource)
    if !allowed {
        return fmt.Errorf("操作被拒绝: %s", reason)
    }
    
    // 2. 记录操作意图（审计）
    a.auditLog.RecordIntent(action, resource)
    
    // 3. 执行操作
    return operation()
}
```

TUI 拦截提示：

```
  ═══════════════════════════════════════════════════════
  ⛔  操作被拒绝
  ═══════════════════════════════════════════════════════

  操作: scale deployment/payment-api --replicas=10 (L2)

  原因: 当前处于维护模式（双11大促，预计 2024-11-12 02:00 解除）

  你的选项:
  ─────────────────────────────────────────────────────
  [R] 请求紧急变更（需要审批人确认）
  [S] 预约变更窗口（2024-11-12 02:00 自动执行）
  [C] 取消
```

### 63.7 on-call 日历集成

```yaml
# ~/.ops-ai/config.yaml
on_call:
  provider: "pagerduty"               # pagerduty | opsgenie | custom
  pagerduty:
    api_key: "${PAGERDUTY_API_KEY}"
    service_id: "PAYMENT_SERVICE"
    
  # on-call 相关规则
  rules:
    require_oncall_for_l3: true         # L3+ 需要 on-call 在场
    allow_oncall_override: true         # on-call 人员可覆盖变更窗口
```

### 63.8 配置项

```yaml
# ~/.ops-ai/config.yaml
maintenance:
  enabled: true
  
  # 默认维护模式行为
  default_behavior:
    block_l2_plus: true
    disable_auto_remediation: true
    disable_scheduled_tasks: true
    allow_l0_diagnostics: true
    
  # 紧急变更
  emergency_change:
    require_approval: true
    approvers: ["sre-manager", "on-call-lead"]
    
  # 预约执行
  scheduled_execution:
    enabled: true
    queue_max_size: 50                  # 最多排队 50 个操作
```

### 63.9 System Prompt 补充

```
## 变更窗口与维护模式知识

当运维尝试在限制时段执行操作时：

1. **维护模式**：
   - 全集群或指定 namespace 可进入维护模式
   - 维护模式下只允许 L0 诊断，禁止所有变更
   - 解除维护模式需要双人确认

2. **变更窗口**：
   - 非变更窗口期间，L2+ 操作被拒绝
   - 可以预约操作到下一个变更窗口自动执行
   - 紧急变更需要审批人确认

3. **on-call 集成**：
   - L3+ 操作需要当前有活跃的 on-call 人员
   - on-call 人员可以覆盖变更窗口限制
   - 与 PagerDuty/OpsGenie 集成自动获取 on-call 状态

4. **告警自动修复**：
   - 维护模式下自动修复完全禁用
   - 告警仍会通知，但不会执行修复操作
```

---

## 64. 告警降噪与通知疲劳管理（v2.0 新增）

### 64.1 问题场景

v1.9 设计了告警自动修复（§26）和事件管理（§47），但**告警本身的质量管理**没有涉及：

- 生产环境常见场景：一个根因触发 50+ 条相关告警（级联告警风暴）
- Agent 收到 50 条告警创建 50 个修复会话，直接打爆 LLM API 和成本预算（§43 的单会话预算被瞬间耗尽）
- 没有告警聚类（alert grouping）、去重（deduplication）、静默（silencing）设计
- **实际运维场景**：网络分区导致 100+ Pod 同时告警，Agent 同时启动 100 个会话，成本 $500+，全部做的是同一个修复

### 64.2 设计目标

- 告警聚类：基于根因相似度将告警分组，同一根因只处理一次
- 告警静默：已知问题或变更窗口期间的告警自动静默
- 告警质量评分：识别"狼来了"式低质量告警，推动规则优化
- 级联告警抑制：父故障触发时自动抑制子告警

### 64.3 Go 接口定义

```go
// AlertNoiseReducer 告警降噪器
type AlertNoiseReducer struct {
    store        AlertStore
    clusterer    *AlertClusterer
    deduper      *AlertDeduplicator
    silencer     *AlertSilencer
    scorer       *AlertQualityScorer
}

// AlertClusterer 告警聚类器
type AlertClusterer struct {
    similarityThreshold float64
}

type AlertCluster struct {
    ID            string
    RootCause     string
    Alerts        []Alert
    CreatedAt     time.Time
    ResolvedAt    *time.Time
    Representative  Alert  // 代表性告警
}

func (c *AlertClusterer) Cluster(alerts []Alert) []AlertCluster {
    var clusters []AlertCluster
    
    for _, alert := range alerts {
        assigned := false
        for i := range clusters {
            if c.isSimilar(alert, clusters[i].Representative) {
                clusters[i].Alerts = append(clusters[i].Alerts, alert)
                assigned = true
                break
            }
        }
        if !assigned {
            clusters = append(clusters, AlertCluster{
                ID:             generateClusterID(),
                Representative: alert,
                Alerts:         []Alert{alert},
                CreatedAt:      time.Now(),
            })
        }
    }
    
    return clusters
}

func (c *AlertClusterer) isSimilar(a, b Alert) bool {
    // 基于多个维度计算相似度
    score := 0.0
    
    // 1. 同一 namespace + 相似资源名
    if a.Namespace == b.Namespace {
        score += 0.3
    }
    
    // 2. 相同告警名称/规则
    if a.RuleName == b.RuleName {
        score += 0.4
    }
    
    // 3. 时间窗口内（5 分钟内）
    if abs(a.FiredAt.Sub(b.FiredAt).Minutes()) < 5 {
        score += 0.2
    }
    
    // 4. 相同标签（如 app, component）
    commonLabels := intersection(a.Labels, b.Labels)
    score += float64(len(commonLabels)) * 0.05
    
    return score >= c.similarityThreshold
}

// AlertDeduplicator 告警去重器
type AlertDeduplicator struct {
    window time.Duration
}

func (d *AlertDeduplicator) Deduplicate(alerts []Alert) []Alert {
    seen := make(map[string]time.Time)
    var unique []Alert
    
    for _, alert := range alerts {
        key := fmt.Sprintf("%s:%s:%s", alert.RuleName, alert.Namespace, alert.ResourceName)
        if lastSeen, ok := seen[key]; ok && time.Since(lastSeen) < d.window {
            continue // 在窗口期内，去重
        }
        seen[key] = alert.FiredAt
        unique = append(unique, alert)
    }
    
    return unique
}

// AlertSilencer 告警静默器
type AlertSilencer struct {
    silences []SilenceRule
}

type SilenceRule struct {
    ID          string
    Matchers    map[string]string  // 标签匹配
    StartTime   time.Time
    EndTime     time.Time
    Reason      string
    CreatedBy   string
}

func (s *AlertSilencer) IsSilenced(alert Alert) (*SilenceRule, bool) {
    for _, rule := range s.silences {
        if time.Now().Before(rule.StartTime) || time.Now().After(rule.EndTime) {
            continue
        }
        if s.matches(alert, rule.Matchers) {
            return &rule, true
        }
    }
    return nil, false
}

// AlertQualityScorer 告警质量评分器
type AlertQualityScorer struct{}

type AlertQualityReport struct {
    RuleName      string
    TotalFires    int
    FalsePositives int
    TruePositives  int
    AutoResolved   int  // 自动修复成功
    Score         float64  // 0-100
    Suggestions   []string
}

func (s *AlertQualityScorer) Score(ruleName string, alerts []Alert) *AlertQualityReport {
    report := &AlertQualityReport{RuleName: ruleName}
    
    for _, alert := range alerts {
        report.TotalFires++
        if alert.Status == "resolved" && alert.ResolutionTime < 2*time.Minute {
            report.FalsePositives++ // 2 分钟内自动恢复，可能是假告警
        } else if alert.AutoRemediated {
            report.AutoResolved++
            report.TruePositives++
        } else {
            report.TruePositives++
        }
    }
    
    // 计算质量分
    if report.TotalFires > 0 {
        fpRate := float64(report.FalsePositives) / float64(report.TotalFires)
        report.Score = (1 - fpRate) * 100
    }
    
    if report.Score < 50 {
        report.Suggestions = append(report.Suggestions, 
            "告警规则阈值可能过低，建议上调")
    }
    
    return report
}
```

### 64.4 告警聚类与降噪流程

```go
func (r *AlertNoiseReducer) Process(alerts []Alert) ([]AlertCluster, error) {
    // 1. 去重
    unique := r.deduper.Deduplicate(alerts)
    
    // 2. 静默过滤
    var active []Alert
    for _, alert := range unique {
        if rule, silenced := r.silencer.IsSilenced(alert); silenced {
            alert.Silenced = true
            alert.SilenceRuleID = rule.ID
            continue
        }
        active = append(active, alert)
    }
    
    // 3. 聚类
    clusters := r.clusterer.Cluster(active)
    
    // 4. 每个聚类只生成一个修复会话
    for _, cluster := range clusters {
        if len(cluster.Alerts) > 1 {
            log.Printf("告警聚类 %s: 将 %d 个告警合并为 1 个修复会话", 
                cluster.ID, len(cluster.Alerts))
        }
    }
    
    return clusters, nil
}
```

### 64.5 TUI 告警聚类展示

```
  ═══════════════════════════════════════════════════════
  🔔  告警降噪面板 — 最近 1 小时
  ═══════════════════════════════════════════════════════

  原始告警: 47 个
  去重后:   32 个
  静默后:   28 个
  聚类后:   5 个聚类（将处理 5 个会话）

  聚类列表
  ─────────────────────────────────────────────────────
  
  聚类 #1 (12 个告警) 🔴
  ─────────────────────────────────────────────────────
  根因: network-partition on node worker-05
  告警: PodNotReady (payment-api-xxx * 5, order-api-xxx * 4, ...)
  操作: 1 个修复会话已创建

  聚类 #2 (8 个告警) 🟠
  ─────────────────────────────────────────────────────
  根因: HighErrorRate-payment-api
  告警: ErrorRateHigh (payment-api * 8)
  操作: 1 个修复会话已创建

  聚类 #3 (4 个告警) 🟡
  ─────────────────────────────────────────────────────
  根因: DiskPressure on node worker-12
  告警: NodeDiskPressure + PodEvicted
  状态: 已静默（维护模式期间）

  [D] 查看详情  [S] 管理静默规则  [Q] 返回
```

### 64.6 告警质量报告

```
ops-ai alert quality --since 7d
```

```
  ═══════════════════════════════════════════════════════
  📊  告警质量报告 — 最近 7 天
  ═══════════════════════════════════════════════════════

  总体质量: 72/100

  低质量告警规则
  ─────────────────────────────────────────────────────
  规则名                        触发次数  假阳性  质量分
  ─────────────────────────────────────────────────────
  HighCPUUsage-worker           342      298     13 🔴
  PodMemoryUsageHigh            156      89      43 🔴
  DiskIOWaitHigh                89       23      74 🟡

  高质量告警规则
  ─────────────────────────────────────────────────────
  PaymentAPIErrorRate           12       0       100 🟢
  DatabaseConnectionPoolExhaust 5        0       100 🟢

  建议优化
  ─────────────────────────────────────────────────────
  1. HighCPUUsage-worker: 阈值 70% 过低，建议上调至 85%
  2. PodMemoryUsageHigh: 缺少容器名标签，大量重复告警

  [E] 导出完整报告  [Q] 返回
```

### 64.7 静默规则管理

```
ops-ai alert silence create --matcher "severity=warning" --duration 2h --reason "已知问题 INC-001"
ops-ai alert silence list
ops-ai alert silence delete <id>
```

### 64.8 配置项

```yaml
# ~/.ops-ai/config.yaml
alert_noise_reduction:
  enabled: true
  
  # 去重
  deduplication:
    enabled: true
    window: "5m"                        # 5 分钟内相同告警去重
    
  # 聚类
  clustering:
    enabled: true
    similarity_threshold: 0.6           # 相似度阈值
    max_cluster_size: 50                # 单个聚类最多 50 个告警
    
  # 静默
  silencing:
    enabled: true
    auto_silence_maintenance: true      # 维护模式自动静默
    max_silence_duration: "24h"         # 最长静默 24 小时
    
  # 质量评分
  quality_scoring:
    enabled: true
    false_positive_window: "2m"         # 2 分钟内自动恢复视为假阳性
    min_samples: 10                     # 最少 10 个样本才评分
    report_interval: "7d"               # 每周生成质量报告
    
  # 成本保护
  cost_protection:
    max_sessions_per_minute: 5          # 每分钟最多启动 5 个修复会话
    max_cost_per_alert_burst: 10.0      # 单次告警风暴成本上限 $10
```

### 64.9 System Prompt 补充

```
## 告警降噪知识

当运维处理告警风暴或询问告警质量时：

1. **告警聚类**：
   - 同一根因的告警会自动聚类为一个修复会话
   - 聚类基于 namespace、规则名、时间窗口、标签相似度
   - 聚类后只会创建 1 个修复会话，避免成本爆炸

2. **告警去重**：
   - 5 分钟内相同规则的相同资源告警自动去重
   - 去重后的告警只保留第一次触发的时间

3. **静默规则**：
   - 已知问题或维护期间可以创建静默规则
   - 静默的告警不会触发修复，但仍会记录
   - 静默规则最长 24 小时，过期自动解除

4. **质量评分**：
   - 2 分钟内自动恢复的告警视为假阳性
   - 质量分 < 50 的告警规则建议优化阈值或标签
   - 每周自动生成告警质量报告

5. **成本保护**：
   - 告警风暴时限制每分钟最多 5 个修复会话
   - 单次告警风暴成本上限 $10
   - 超过上限时转为只通知不修复
```

---

# 第五部分：P2 — 增强级（锦上添花）

---

## 65. CronJob / 批处理任务运维（v2.0 新增）

### 65.1 问题场景

v1.9 全文**零匹配** "CronJob/Job/批处理"。但批处理任务是 K8s 运维的重要场景：

- CronJob 失败后的重试策略、历史清理、并发控制
- Job 的并行执行、完成度监控、日志聚合
- Spark/Flink on K8s 的作业管理
- **实际运维场景**："每天凌晨 2 点的数据同步 CronJob 连续 3 天失败，失败 Pod 堆积占满节点资源，导致其他服务被驱逐"

### 65.2 设计目标

- CronJob 运行状态监控和历史趋势
- Job 失败诊断：日志分析、资源不足检测、依赖服务检查
- 自动清理失败的 Job 历史，防止资源泄漏
- 批处理作业资源优化推荐

### 65.3 Go 接口定义

```go
// BatchJobManager 批处理任务管理器
type BatchJobManager struct {
    k8sClient     kubernetes.Interface
    config        BatchJobConfig
}

// CronJobMonitor CronJob 监控器
type CronJobMonitor struct {
    k8sClient kubernetes.Interface
}

type CronJobStatus struct {
    Name          string
    Namespace     string
    Schedule      string
    LastRun       *JobRun
    NextRun       time.Time
    History       []JobRun
    SuccessRate   float64
    Issues        []string
}

type JobRun struct {
    Name        string
    StartTime   time.Time
    CompletionTime *time.Time
    Status      string  // Running | Succeeded | Failed
    Duration    time.Duration
    Pods        []PodStatus
}

// JobDiagnoser Job 诊断器
type JobDiagnoser struct {
    k8sClient kubernetes.Interface
}

type JobDiagnosis struct {
    JobName       string
    Namespace     string
    Status        string
    FailedPods    []PodStatus
    RootCause     string
    Suggestions   []string
}

// CronJobCleaner CronJob 历史清理器
type CronJobCleaner struct {
    k8sClient kubernetes.Interface
}

type CleanupPolicy struct {
    MaxSuccessfulJobs int  // 保留最近 N 个成功的 Job
    MaxFailedJobs     int  // 保留最近 N 个失败的 Job
    MaxAge            time.Duration
}
```

### 65.4 CronJob 状态监控

```
ops-ai cronjob status --namespace data-pipeline
```

```
  ═══════════════════════════════════════════════════════
  ⏰  CronJob 状态 — data-pipeline namespace
  ═══════════════════════════════════════════════════════

  名称              调度        上次运行      成功率    下次运行
  ─────────────────────────────────────────────────────
  data-sync         0 2 * * *   3h ago       67% 🟡    21h
  report-gen        0 6 * * 1   2d ago       100% 🟢   5d
  cleanup-task      */30 * * *  12m ago      95% 🟢    18m

  问题 CronJob
  ─────────────────────────────────────────────────────
  data-sync:
    - 最近 7 天失败 2 次（OOMKilled、ImagePullBackOff）
    - 失败 Pod 堆积: 6 个（占用节点资源）
    - 建议: 增加内存限制，修复镜像标签

  [D] 诊断  [C] 清理历史  [Q] 返回
```

### 65.5 Job 失败诊断

```go
func (d *JobDiagnoser) Diagnose(ctx context.Context, jobName, namespace string) (*JobDiagnosis, error) {
    job, err := d.k8sClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }
    
    diagnosis := &JobDiagnosis{
        JobName:   jobName,
        Namespace: namespace,
        Status:    string(getJobStatus(job)),
    }
    
    // 1. 获取失败 Pod
    pods, _ := d.k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
        LabelSelector: fmt.Sprintf("job-name=%s", jobName),
    })
    
    for _, pod := range pods.Items {
        if pod.Status.Phase == corev1.PodFailed {
            diagnosis.FailedPods = append(diagnosis.FailedPods, PodStatus{
                Name:   pod.Name,
                Reason: pod.Status.Reason,
            })
            
            // 2. 分析失败原因
            for _, containerStatus := range pod.Status.ContainerStatuses {
                if containerStatus.State.Terminated != nil {
                    exitCode := containerStatus.State.Terminated.ExitCode
                    switch exitCode {
                    case 137:
                        diagnosis.RootCause = "OOMKilled（内存不足）"
                        diagnosis.Suggestions = append(diagnosis.Suggestions, 
                            fmt.Sprintf("增加内存限制: current %s → recommend %s", 
                                getCurrentMemoryLimit(&pod), recommendMemoryLimit(&pod)))
                    case 1:
                        diagnosis.RootCause = "应用错误（exit code 1）"
                        diagnosis.Suggestions = append(diagnosis.Suggestions, 
                            "查看容器日志定位应用错误")
                    }
                }
            }
        }
    }
    
    return diagnosis, nil
}
```

### 65.6 自动清理策略

```go
func (c *CronJobCleaner) Cleanup(ctx context.Context, policy CleanupPolicy) error {
    cronjobs, _ := c.k8sClient.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
    
    for _, cj := range cronjobs.Items {
        // 获取该 CronJob 的所有 Job 历史
        jobs, _ := c.k8sClient.BatchV1().Jobs(cj.Namespace).List(ctx, metav1.ListOptions{
            LabelSelector: fmt.Sprintf("cronjob-name=%s", cj.Name),
        })
        
        var successful, failed []batchv1.Job
        for _, job := range jobs.Items {
            if isJobSuccessful(&job) {
                successful = append(successful, job)
            } else {
                failed = append(failed, job)
            }
        }
        
        // 清理过多的成功 Job
        if len(successful) > policy.MaxSuccessfulJobs {
            toDelete := successful[:len(successful)-policy.MaxSuccessfulJobs]
            for _, job := range toDelete {
                c.k8sClient.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{
                    PropagationPolicy: strPtr(string(metav1.DeletePropagationBackground)),
                })
            }
        }
        
        // 清理过多的失败 Job
        if len(failed) > policy.MaxFailedJobs {
            toDelete := failed[:len(failed)-policy.MaxFailedJobs]
            for _, job := range toDelete {
                c.k8sClient.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{
                    PropagationPolicy: strPtr(string(metav1.DeletePropagationBackground)),
                })
            }
        }
    }
    
    return nil
}
```

### 65.7 L0 命令扩展

```
ops-ai cronjob list [-n <ns>]             # 列出 CronJob
ops-ai cronjob status <name> [-n <ns>]    # CronJob 状态
ops-ai cronjob history <name> [-n <ns>]   # 运行历史
ops-ai cronjob diagnose <name> [-n <ns>]  # 失败诊断
ops-ai cronjob cleanup [--dry-run]        # 清理历史 Job
ops-ai job status <name> [-n <ns>]        # Job 状态
ops-ai job logs <name> [-n <ns>]          # Job 日志
```

### 65.8 配置项

```yaml
# ~/.ops-ai/config.yaml
batch_jobs:
  enabled: true
  
  # 自动清理
  cleanup:
    enabled: true
    max_successful_jobs: 3              # 保留 3 个成功的 Job
    max_failed_jobs: 5                  # 保留 5 个失败的 Job
    max_age: "168h"                     # 保留 7 天内的 Job
    schedule: "0 3 * * *"               # 每天凌晨 3 点清理
    
  # 监控
  monitoring:
    check_interval: "5m"
    alert_on_failure: true
    alert_on_stuck: true                # Job 运行超过预期时间告警
    
  # 资源优化
  resource_optimization:
    enabled: true
    analyze_history: 10                 # 分析最近 10 次运行
```

### 65.9 System Prompt 补充

```
## 批处理任务运维知识

当运维询问 CronJob 或 Job 相关问题时：

1. **CronJob 监控**：
   - 关注成功率和最近失败原因
   - 失败 Pod 堆积会占用节点资源，需要定期清理

2. **Job 失败诊断**：
   - Exit Code 137 = OOMKilled，建议增加内存限制
   - Exit Code 1 = 应用错误，查看日志定位
   - ImagePullBackOff = 镜像不存在或拉取失败

3. **自动清理**：
   - 默认保留 3 个成功 Job + 5 个失败 Job
   - 超过限制的旧 Job 自动删除（包括关联 Pod）
   - 可以配置保留策略

4. **资源优化**：
   - 基于历史运行数据推荐 CPU/内存 request/limit
   - 长时间运行的 Job 可能有资源泄漏
```

---

## 66. Agent 自身调试能力（v2.0 新增）

### 66.1 问题场景

当 Agent 行为异常（给出错误建议、陷入死循环、误删资源）时，**如何排查 Agent 本身**？v1.9 没有设计：

- Agent 的决策链路追踪（为什么 Agent 认为应该删除这个 Pod？）
- LLM 调用链的可视化（哪一步工具调用导致了错误结论？）
- 会话回放：完整重现 Agent 的思考和操作过程
- **实际运维场景**："Agent 刚才错误地建议删除了一个生产数据库 Pod，我想知道它为什么会做出这个决定，以便修复 Prompt 或规则"

### 66.2 设计目标

- 决策链路追踪：记录 Agent 每一步的思考过程和工具调用
- 会话回放：完整重现 Agent 的操作序列
- LLM 调用分析：token 消耗、延迟、响应质量
- 调试模式：允许运维手动干预 Agent 的决策过程

### 66.3 Go 接口定义

```go
// AgentDebugger Agent 调试器
type AgentDebugger struct {
    traceStore    TraceStore
    sessionStore  SessionStore
    llmAnalyzer   *LLMAnalyzer
}

// DecisionTrace 决策链路追踪
type DecisionTrace struct {
    SessionID     string
    Timestamp     time.Time
    Steps         []DecisionStep
}

type DecisionStep struct {
    Order         int
    Type          string  // thought | tool_call | tool_result | user_input
    Content       string
    Latency       time.Duration
    TokensUsed    int
    Confidence    float64
}

// SessionReplayer 会话回放器
type SessionReplayer struct {
    sessionStore SessionStore
}

func (r *SessionReplayer) Replay(sessionID string) (*ReplayResult, error) {
    session, err := r.sessionStore.Load(sessionID)
    if err != nil {
        return nil, err
    }
    
    return &ReplayResult{
        SessionID: sessionID,
        Steps:     session.DecisionTrace.Steps,
        CanReplay: true,
    }, nil
}

// LLMAnalyzer LLM 调用分析器
type LLMAnalyzer struct{}

type LLMAnalysis struct {
    CallID        string
    Prompt        string
    Response      string
    TokensInput   int
    TokensOutput  int
    Latency       time.Duration
    QualityScore  float64  // 基于响应是否有效解决问题
    Hallucination bool     // 是否检测到幻觉
}

// DebugMode 调试模式
type DebugMode struct {
    Enabled       bool
    Breakpoints   []DebugBreakpoint
    CurrentStep   int
}

type DebugBreakpoint struct {
    StepType      string  // tool_call | before_action
    Condition     string
}
```

### 66.4 决策链路追踪

```
ops-ai debug trace --session sess-abc123
```

```
  ═══════════════════════════════════════════════════════
  🔍  决策链路追踪 — sess-abc123
  ═══════════════════════════════════════════════════════

  Step 1 [thought] (234 tokens, 1.2s)
  ─────────────────────────────────────────────────────
  用户报告 payment-api 不可用。
  我需要先检查 Pod 状态，然后查看日志。

  Step 2 [tool_call] (kubectl get pods -n payment)
  ─────────────────────────────────────────────────────
  结果: payment-api-7d8f9 状态 CrashLoopBackOff

  Step 3 [thought] (189 tokens, 0.8s)
  ─────────────────────────────────────────────────────
  Pod 处于 CrashLoopBackOff，可能是应用错误或资源不足。
  我需要查看日志和事件。

  Step 4 [tool_call] (kubectl logs payment-api-7d8f9 -n payment)
  ─────────────────────────────────────────────────────
  结果: OutOfMemoryError: Java heap space

  Step 5 [thought] (312 tokens, 1.5s)
  ─────────────────────────────────────────────────────
  应用因为内存不足崩溃。建议增加内存限制。
  但需要注意：增加内存可能导致节点资源不足。

  Step 6 [tool_call] (kubectl get nodes)
  ─────────────────────────────────────────────────────
  结果: 节点内存使用率 45%，有足够空间

  Step 7 [suggestion] (L2)
  ─────────────────────────────────────────────────────
  建议: 将 payment-api 内存限制从 512Mi 增加到 1Gi
  原因: 应用 OOM，节点有足够资源

  [P] 播放回放  [E] 导出 JSON  [Q] 返回
```

### 66.5 会话回放

```go
func (r *SessionReplayer) ReplayInteractive(sessionID string) error {
    result, err := r.Replay(sessionID)
    if err != nil {
        return err
    }
    
    for i, step := range result.Steps {
        fmt.Printf("Step %d [%s]:\n%s\n\n", i+1, step.Type, step.Content)
        
        if step.Type == "tool_call" {
            fmt.Println("按 Enter 继续，或输入 'skip' 跳到下一步...")
            // 等待用户输入
        }
    }
    
    return nil
}
```

### 66.6 LLM 调用分析

```
ops-ai debug llm --session sess-abc123
```

```
  ═══════════════════════════════════════════════════════
  🤖  LLM 调用分析 — sess-abc123
  ═══════════════════════════════════════════════════════

  调用次数: 5
  总 tokens: 8,432 input / 3,210 output
  总成本:   $1.42
  平均延迟: 1.1s

  详细调用
  ─────────────────────────────────────────────────────
  #1  thought   234t/156t  1.2s  ✅
  #2  thought   189t/98t   0.8s  ✅
  #3  thought   312t/245t  1.5s  ✅
  #4  thought   456t/312t  2.1s  ⚠️  延迟偏高
  #5  thought   234t/89t   0.9s  ✅

  质量分析
  ─────────────────────────────────────────────────────
  幻觉检测:   未发现
  置信度:     平均 0.87
  建议质量:   4/5 有效

  [Q] 返回
```

### 66.7 调试模式

```
运维: "ops-ai debug --session sess-abc123 --breakpoint tool_call"

Agent:
  进入调试模式，在每个工具调用前暂停。

  Step 3 [breakpoint] before tool_call
  ─────────────────────────────────────────────────────
  即将执行: kubectl logs payment-api-7d8f9 -n payment
  
  [C] 继续执行  [S] 跳过此步骤  [M] 修改参数  [A] 中止会话
```

### 66.8 L0 命令扩展

```
ops-ai debug trace --session <id>           # 查看决策链路
ops-ai debug replay --session <id>          # 回放会话
ops-ai debug llm --session <id>             # LLM 调用分析
ops-ai debug sessions --since 1h            # 列出最近会话
```

### 66.9 配置项

```yaml
# ~/.ops-ai/config.yaml
debug:
  enabled: true
  
  # 决策追踪
  decision_trace:
    enabled: true
    max_steps: 100                      # 最多追踪 100 步
    retention: "168h"                   # 保留 7 天
    
  # LLM 分析
  llm_analysis:
    enabled: true
    track_tokens: true
    track_latency: true
    hallucination_detection: true
    
  # 调试模式
  debug_mode:
    enabled: true
    default_breakpoints: []             # 默认断点
```

### 66.10 System Prompt 补充

```
## Agent 调试知识

当 Agent 行为异常或需要排查时：

1. **决策追踪**：
   - 每个会话的完整决策链路可查询
   - 包含每一步的思考过程、工具调用、结果
   - 用于定位 Agent 为什么做出错误决定

2. **会话回放**：
   - 可以按步骤回放 Agent 的操作序列
   - 支持交互式回放（每步暂停）
   - 回放不会实际执行操作

3. **LLM 分析**：
   - 分析每次 LLM 调用的 token 消耗、延迟、质量
   - 检测幻觉（生成不存在的信息）
   - 帮助优化 Prompt 和模型选择

4. **调试模式**：
   - 可以在特定步骤设置断点
   - 允许手动修改工具调用参数
   - 用于测试和验证 Agent 行为
```

---

## 67. Terraform / IaC 状态管理深度集成（v2.0 新增）

### 67.1 问题场景

v1.8 提到跨工具链（K8s + TF + Cloud），但 v1.9 没有任何 Terraform 相关的深度设计：

- Terraform state 冲突检测：多个运维同时 terraform apply 导致 state 锁定或冲突
- K8s 资源与 Terraform 管理的资源关联（避免"幽灵资源"——K8s 中存在但 TF state 中没有，或反之）
- Terraform plan 预检：Agent 在执行 K8s 操作前检查是否与 TF 配置冲突
- **实际运维场景**：运维通过 Agent 删除了一个 Terraform 管理的 namespace，下次 Terraform apply 时报错 "resource not found"，需要手动 taint 或 import

### 67.2 设计目标

- 识别 K8s 资源是否由 Terraform 管理
- Terraform state 与 K8s 实际资源的差异检测（drift detection）
- K8s 操作前检查是否与 TF 配置冲突
- Terraform plan 预检辅助

### 67.3 Go 接口定义

```go
// TerraformIntegrationManager Terraform 集成管理器
type TerraformIntegrationManager struct {
    k8sClient     kubernetes.Interface
    tfStateReader TFStateReader
    config        TerraformConfig
}

// TFStateReader Terraform State 读取器
type TFStateReader interface {
    ReadState(backend string) (*TFState, error)
}

type TFState struct {
    Version   int
    Resources []TFResource
}

type TFResource struct {
    Type      string
    Name      string
    Provider  string
    Instances []TFResourceInstance
}

type TFResourceInstance struct {
    ID        string
    Attributes map[string]interface{}
}

// DriftDetector 差异检测器
type DriftDetector struct {
    k8sClient     kubernetes.Interface
    tfStateReader TFStateReader
}

type DriftResult struct {
    ResourceType string
    ResourceName string
    Namespace    string
    TFState      map[string]interface{}
    K8sActual    map[string]interface{}
    Differences  []FieldDiff
    Severity     string  // critical | warning | info
}

type FieldDiff struct {
    Field     string
    TFValue   interface{}
    K8sValue  interface{}
    Action    string  // added | removed | changed
}

// K8sTerraformMapper K8s-Terraform 资源映射器
type K8sTerraformMapper struct{}

func (m *K8sTerraformMapper) IsTerraformManaged(resource ResourceRef, tfState *TFState) (bool, *TFResource) {
    for _, tfRes := range tfState.Resources {
        if tfRes.Type == fmt.Sprintf("kubernetes_%s", resource.Kind) {
            for _, inst := range tfRes.Instances {
                if inst.Attributes["metadata.name"] == resource.Name &&
                   inst.Attributes["metadata.namespace"] == resource.Namespace {
                    return true, &tfRes
                }
            }
        }
    }
    return false, nil
}
```

### 67.4 差异检测（Drift Detection）

```
ops-ai terraform drift --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  🏗️  Terraform 差异检测 — payment namespace
  ═══════════════════════════════════════════════════════

  扫描 Terraform State: s3://company-terraform-state/prod.tfstate

  差异列表
  ─────────────────────────────────────────────────────
  
  🔴 Critical (2)
  ─────────────────────────────────────────────────────
  deployment/payment-api
    TF 中:   replicas=3, image=v2.3.1
    K8s 中:  replicas=5, image=v2.4.0
    原因:    手动 scale 和镜像更新，与 TF 配置不一致
    风险:    下次 TF apply 会回滚到 v2.3.1
    
  configmap/payment-config
    TF 中:   存在
    K8s 中:  不存在
    原因:    手动删除
    风险:    下次 TF apply 会重新创建

  🟡 Warning (1)
  ─────────────────────────────────────────────────────
  service/payment-api
    TF 中:   type=ClusterIP
    K8s 中:  type=NodePort
    原因:    手动修改了 Service 类型

  [A] 自动对齐（L3，需确认）  [E] 导出报告  [Q] 返回
```

### 67.5 K8s 操作前 Terraform 检查

```go
func (a *Agent) checkTerraformBeforeAction(ctx context.Context, action ActionLevel, resource ResourceRef) (*TerraformCheckResult, error) {
    // 1. 读取 Terraform State
    tfState, err := a.terraformManager.tfStateReader.ReadState(a.config.Terraform.Backend)
    if err != nil {
        return nil, err
    }
    
    // 2. 检查资源是否由 Terraform 管理
    isManaged, tfRes := a.terraformManager.mapper.IsTerraformManaged(resource, tfState)
    if !isManaged {
        return &TerraformCheckResult{Managed: false}, nil
    }
    
    // 3. 如果是 L2+ 操作，警告运维
    if action >= ActionLevelL2 {
        return &TerraformCheckResult{
            Managed:     true,
            TFResource:  tfRes,
            Warning:     fmt.Sprintf("该资源由 Terraform 管理 (%s.%s)，直接修改会导致 TF drift", tfRes.Type, tfRes.Name),
            Suggestion:  "建议通过 Terraform 修改，或使用 `terraform taint` 后重新 apply",
        }, nil
    }
    
    return &TerraformCheckResult{Managed: true, TFResource: tfRes}, nil
}
```

TUI 拦截提示：

```
  ═══════════════════════════════════════════════════════
  ⚠️  Terraform 管理资源警告
  ═══════════════════════════════════════════════════════

  操作: scale deployment/payment-api --replicas=10 (L2)

  警告: 该资源由 Terraform 管理
  TF 地址: kubernetes_deployment.payment_api
  TF 文件: modules/payment/main.tf

  直接修改会导致:
  1. Terraform state 与实际状态不一致
  2. 下次 `terraform apply` 可能回滚此变更
  3. 团队其他成员可能困惑

  建议:
  ─────────────────────────────────────────────────────
  [T] 通过 Terraform 修改（打开 TF 文件并指导修改）
  [F] 强制修改（将记录 drift，后续需手动同步）
  [C] 取消
```

### 67.6 Terraform Plan 预检辅助

```
运维: "帮我检查当前 Terraform plan 是否安全"

Agent:
  1. 执行 terraform plan -out=tfplan
  2. 解析 plan 输出
  3. 分析变更影响

  ═══════════════════════════════════════════════════════
  🏗️  Terraform Plan 预检
  ═══════════════════════════════════════════════════════

  计划文件: tfplan
  变更资源: 5

  变更分析
  ─────────────────────────────────────────────────────
  + kubernetes_deployment.new_service  (新建)
    风险: 低
    
  ~ kubernetes_deployment.payment_api  (修改)
    变更: replicas 3 → 5, image v2.3.1 → v2.4.0
    风险: 中（镜像升级）
    建议: 确认镜像已通过金丝雀验证
    
  - kubernetes_configmap.old_config  (删除)
    风险: 高 🔴
    问题: 该 ConfigMap 仍被 deployment/cache-service 引用
    后果: 删除后 cache-service 启动失败
    建议: 先更新 cache-service 的引用，再删除

  [C] 继续 apply（L3）  [A] 中止  [Q] 返回
```

### 67.7 L0 命令扩展

```
ops-ai terraform drift [--namespace <ns>]     # 差异检测
ops-ai terraform status <resource>            # 查看资源 TF 状态
ops-ai terraform plan-check                   # Plan 预检辅助
ops-ai terraform import-guidance <resource>   # 生成 import 命令
```

### 67.8 配置项

```yaml
# ~/.ops-ai/config.yaml
terraform:
  enabled: true
  
  # State 后端
  state_backend:
    type: "s3"                          # s3 | gcs | azure | local | remote
    s3:
      bucket: "company-terraform-state"
      key: "prod/terraform.tfstate"
      region: "us-east-1"
      
  # 检测范围
  detection:
    check_before_l2: true               # L2+ 操作前检查
    auto_detect_backend: true           # 自动检测 TF 后端
    
  # Drift 检测
  drift:
    enabled: true
    schedule: "0 6 * * *"               # 每天早 6 点检测
    alert_on_drift: true
    ignore_fields:                      # 忽略的差异字段
      - "metadata.annotations kubectl.kubernetes.io/last-applied-configuration"
      
  # Plan 预检
  plan_check:
    enabled: true
    terraform_path: "terraform"         # terraform 可执行文件路径
    workdir: "/infra/terraform"
```

### 67.9 System Prompt 补充

```
## Terraform 集成知识

当运维操作可能涉及 Terraform 管理的资源时：

1. **资源识别**：
   - Agent 会自动检测 K8s 资源是否由 Terraform 管理
   - Terraform 管理的资源在 TUI 中显示 🏗️ 标记

2. **Drift 检测**：
   - 定期对比 TF state 和 K8s 实际状态
   - Critical drift（如镜像版本不一致）需要优先处理
   - 建议通过 Terraform 修改而非直接操作 K8s

3. **操作前检查**：
   - L2+ 操作前检查资源是否由 TF 管理
   - 如果是，提供通过 TF 修改的选项
   - 强制修改会记录 drift，后续需手动同步

4. **Plan 预检**：
   - 帮助分析 terraform plan 的安全性和风险
   - 识别删除仍在引用的资源等危险操作
   - 提供修复建议后再执行 apply
```

---

## 68. 数据驻留与主权合规（v2.0 新增）

### 68.1 问题场景

v1.9 有审计日志，但**数据存储位置**没有设计：

- 某些行业要求审计数据必须存储在特定区域（中国金融数据不出境）
- LLM API 调用是否会导致数据跨境传输？（Prompt 中包含 Pod 名称、日志内容可能被视为敏感数据）
- Agent 配置中缺少数据驻留策略
- **实际运维场景**：金融机构合规审计时要求证明"所有运维数据未离开中国大陆"，Agent 无法提供证据

### 68.2 设计目标

- 数据分类和标记：识别哪些数据可以出境，哪些不可以
- LLM 数据脱敏：向外部 LLM API 发送前自动脱敏敏感信息
- 存储位置控制：审计日志、会话数据、缓存的存储区域
- 合规报告：生成数据流向报告

### 68.3 Go 接口定义

```go
// DataResidencyManager 数据驻留管理器
type DataResidencyManager struct {
    classifier   *DataClassifier
    sanitizer    *DataSanitizer
    storage      *ResidencyAwareStorage
    config       DataResidencyConfig
}

// DataClassifier 数据分类器
type DataClassifier struct {
    rules []ClassificationRule
}

type ClassificationRule struct {
    Pattern     string  // 正则表达式
    Category    string  // PII | Financial | Healthcare | Internal | Public
    CanLeaveRegion bool
}

type DataClassification struct {
    Content     string
    Category    string
    Sensitive   bool
    CanLeaveRegion bool
}

// DataSanitizer 数据脱敏器
type DataSanitizer struct {
    rules []SanitizationRule
}

type SanitizationRule struct {
    Pattern     string
    Replacement string
    Description string
}

func (s *DataSanitizer) SanitizeForLLM(input string) string {
    result := input
    
    // 1. 替换 Pod 名称中的敏感信息
    result = regexp.MustCompile(`[a-z0-9-]+-api-[a-z0-9]{5}`).ReplaceAllString(result, "<pod-name>")
    
    // 2. 替换 IP 地址
    result = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).ReplaceAllString(result, "<ip-address>")
    
    // 3. 替换邮箱
    result = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`).ReplaceAllString(result, "<email>")
    
    // 4. 替换 Secret 值（双重保护）
    result = regexp.MustCompile(`(password|token|key|secret):\s*\S+`).ReplaceAllString(result, "$1: <redacted>")
    
    return result
}

// ResidencyAwareStorage 驻留感知存储
type ResidencyAwareStorage struct {
    primary   StorageBackend    // 主存储（必须满足驻留要求）
    cache     StorageBackend    // 缓存（可本地）
}

type StorageBackend struct {
    Type     string  // s3 | gcs | azure | local
    Region   string
    Endpoint string
}
```

### 68.4 LLM 数据脱敏

```go
func (a *Agent) sendToLLM(ctx context.Context, prompt string) (string, error) {
    // 1. 分类数据
    classification := a.dataResidency.classifier.Classify(prompt)
    
    // 2. 如果数据不能出境，检查是否有本地模型
    if !classification.CanLeaveRegion {
        if a.config.LLM.LocalModel.Enabled {
            return a.localModel.Complete(ctx, prompt)
        }
        return "", fmt.Errorf("数据包含不能出境的敏感信息，且未配置本地模型")
    }
    
    // 3. 脱敏
    sanitized := a.dataResidency.sanitizer.SanitizeForLLM(prompt)
    
    // 4. 记录数据流向（审计）
    a.auditLog.RecordDataFlow(DataFlow{
        Direction:   "outbound",
        Destination: a.config.LLM.Provider,
        Size:        len(sanitized),
        Sanitized:   true,
    })
    
    // 5. 发送
    return a.llmClient.Complete(ctx, sanitized)
}
```

### 68.5 数据流向报告

```
ops-ai compliance data-flow --since 30d
```

```
  ═══════════════════════════════════════════════════════
  🌐  数据流向报告 — 最近 30 天
  ═══════════════════════════════════════════════════════

  数据出境统计
  ─────────────────────────────────────────────────────
  目的地           数据类型      数据量    是否脱敏
  ─────────────────────────────────────────────────────
  OpenAI API       诊断上下文    45.2MB    ✅ 已脱敏
  OpenAI API       日志片段      12.1MB    ✅ 已脱敏
  
  存储位置
  ─────────────────────────────────────────────────────
  数据类型         存储位置      区域
  ─────────────────────────────────────────────────────
  审计日志         PostgreSQL    cn-beijing ✅
  会话数据         PostgreSQL    cn-beijing ✅
  缓存             本地磁盘      cn-beijing ✅
  快照             S3            cn-beijing ✅

  合规状态: ✅ 所有数据存储在指定区域内

  [E] 导出报告  [Q] 返回
```

### 68.6 配置项

```yaml
# ~/.ops-ai/config.yaml
data_residency:
  enabled: true
  
  # 数据分类规则
  classification:
    rules:
      - pattern: "\\b\\d{18}\\b"           # 身份证号
        category: "PII"
        can_leave_region: false
      - pattern: "\\b\\d{16,19}\\b"        # 银行卡号
        category: "Financial"
        can_leave_region: false
      - pattern: "password|token|secret"
        category: "Internal"
        can_leave_region: false
        
  # LLM 脱敏
  llm_sanitization:
    enabled: true
    rules:
      - pattern: "[a-z0-9-]+-api-[a-z0-9]{5}"
        replacement: "<pod-name>"
      - pattern: "\\d+\\.\\d+\\.\\d+\\.\\d+"
        replacement: "<ip-address>"
      - pattern: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
        replacement: "<email>"
        
  # 存储位置
  storage:
    required_region: "cn-beijing"
    audit_log_region: "cn-beijing"
    session_data_region: "cn-beijing"
    
  # 本地模型（用于不能出境的数据）
  local_model:
    enabled: true
    model_path: "/models/llama-3-8b"
    max_context: 8192
```

### 68.7 System Prompt 补充

```
## 数据驻留与合规知识

当处理可能涉及敏感数据的问题时：

1. **数据分类**：
   - 自动识别 PII、金融数据、健康数据等敏感类型
   - 敏感数据默认不能出境

2. **LLM 脱敏**：
   - 向外部 LLM 发送前自动脱敏
   - 脱敏规则可配置
   - 脱敏操作会记录在审计日志中

3. **本地模型**：
   - 敏感数据优先使用本地模型处理
   - 本地模型效果可能不如云端模型
   - 可以配置哪些场景强制使用本地模型

4. **合规报告**：
   - 定期生成数据流向报告
   - 证明数据未离开指定区域
   - 报告可用于合规审计
```

---

## 69. 边缘计算 / K3s 支持（v2.0 新增）

### 69.1 问题场景

v1.9 没有提及边缘场景：

- K3s、MicroK8s、Kind 等轻量级 K8s 的适配
- 边缘节点网络不稳定时的 Agent 行为
- 资源受限环境（ARM 小节点）的性能优化
- **实际运维场景**："工厂边缘部署了 50 个 K3s 节点，网络连接不稳定，Agent 频繁超时和重连，导致大量误告警"

### 69.2 设计目标

- K3s / MicroK8s 自动检测和适配
- 网络不稳定环境下的重试和退避策略
- ARM 架构支持
- 资源受限环境下的轻量级模式

### 69.3 Go 接口定义

```go
// EdgeComputingManager 边缘计算管理器
type EdgeComputingManager struct {
    k8sClient     kubernetes.Interface
    detector      *EdgeClusterDetector
    config        EdgeConfig
}

// EdgeClusterDetector 边缘集群检测器
type EdgeClusterDetector struct{}

type ClusterType string

const (
    ClusterTypeStandard ClusterType = "standard"   // 标准 K8s
    ClusterTypeK3s      ClusterType = "k3s"        // K3s
    ClusterTypeMicroK8s ClusterType = "microk8s"   // MicroK8s
    ClusterTypeKind     ClusterType = "kind"       // Kind
    ClusterTypeMinikube ClusterType = "minikube"   // Minikube
)

type EdgeClusterInfo struct {
    Type          ClusterType
    Version       string
    NodeCount     int
    Arch          string  // amd64 | arm64 | arm
    ResourceLevel string  // rich | limited | constrained
    NetworkStable bool
}

// UnstableNetworkAdapter 不稳定网络适配器
type UnstableNetworkAdapter struct {
    maxRetries      int
    baseBackoff     time.Duration
    maxBackoff      time.Duration
    circuitBreaker  *CircuitBreaker
}

type CircuitBreaker struct {
    failureThreshold int
    successThreshold int
    timeout          time.Duration
    state            string  // closed | open | half-open
}

// LightweightMode 轻量级模式
type LightweightMode struct {
    enabled           bool
    disabledFeatures  []string
    reducedConcurrency int
}
```

### 69.4 边缘集群检测

```go
func (d *EdgeClusterDetector) Detect(ctx context.Context, k8sClient kubernetes.Interface) (*EdgeClusterInfo, error) {
    info := &EdgeClusterInfo{}
    
    // 1. 检测节点标签
    nodes, _ := k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if len(nodes.Items) > 0 {
        // 检查 K3s 标签
        if _, ok := nodes.Items[0].Labels["k3s.io/hostname"]; ok {
            info.Type = ClusterTypeK3s
        }
        // 检查 MicroK8s 标签
        if _, ok := nodes.Items[0].Labels["microk8s.io/cluster"]; ok {
            info.Type = ClusterTypeMicroK8s
        }
        
        // 检测架构
        info.Arch = nodes.Items[0].Status.NodeInfo.Architecture
        
        // 检测资源水平
        totalCPU := int64(0)
        totalMem := int64(0)
        for _, node := range nodes.Items {
            totalCPU += node.Status.Allocatable.Cpu().MilliValue()
            totalMem += node.Status.Allocatable.Memory().Value()
        }
        
        if totalCPU < 4000 || totalMem < 8*1024*1024*1024 {
            info.ResourceLevel = "constrained"
        } else if totalCPU < 16000 || totalMem < 32*1024*1024*1024 {
            info.ResourceLevel = "limited"
        } else {
            info.ResourceLevel = "rich"
        }
    }
    
    info.NodeCount = len(nodes.Items)
    
    return info, nil
}
```

### 69.5 轻量级模式

```go
func (a *Agent) enableLightweightMode(info *EdgeClusterInfo) {
    if info.ResourceLevel != "constrained" {
        return
    }
    
    a.lightweightMode = &LightweightMode{
        enabled: true,
        disabledFeatures: []string{
            "runbook_rag",           // 禁用向量搜索
            "compliance_scan",       // 禁用合规扫描
            "chaos_engineering",     // 禁用混沌工程
            "capacity_planning",     // 禁用容量规划
        },
        reducedConcurrency: 1,
    }
    
    // 调整内存限制
    a.slaManager.resourceGuard.maxMemoryMB = 128
    
    // 禁用 Prometheus 查询（可能没有 Prometheus）
    a.prometheusClient = nil
    
    log.Println("已启用轻量级模式，部分功能已禁用")
}
```

轻量级模式 TUI：

```
  ═══════════════════════════════════════════════════════
  🪶  轻量级模式 — K3s 边缘集群
  ═══════════════════════════════════════════════════════

  检测到的环境
  ─────────────────────────────────────────────────────
  集群类型:    K3s v1.28.5+k3s1
  架构:        arm64
  资源水平:    受限 (2 CPU / 4GB 内存)
  网络状态:    不稳定

  已自动调整
  ─────────────────────────────────────────────────────
  内存限制:    128MB
  并发:        1
  功能禁用:    Runbook RAG, 合规扫描, 混沌工程, 容量规划
  
  可用功能
  ─────────────────────────────────────────────────────
  ✅ Pod/Node 诊断
  ✅ 日志查看
  ✅ 基础 kubectl 操作
  ✅ 告警通知

  [Q] 返回
```

### 69.6 不稳定网络适配

```go
func (a *UnstableNetworkAdapter) CallWithRetry(ctx context.Context, operation func() error) error {
    backoff := a.baseBackoff
    
    for i := 0; i < a.maxRetries; i++ {
        // 检查断路器
        if a.circuitBreaker.state == "open" {
            return fmt.Errorf("断路器已打开，服务暂时不可用")
        }
        
        err := operation()
        if err == nil {
            a.circuitBreaker.RecordSuccess()
            return nil
        }
        
        // 判断是否为网络错误
        if isNetworkError(err) {
            a.circuitBreaker.RecordFailure()
            
            if i < a.maxRetries-1 {
                log.Printf("网络错误，%v 后重试 (%d/%d)", backoff, i+1, a.maxRetries)
                time.Sleep(backoff)
                backoff = min(backoff*2, a.maxBackoff)
            }
        } else {
            return err // 非网络错误，直接返回
        }
    }
    
    return fmt.Errorf("经过 %d 次重试后仍然失败", a.maxRetries)
}
```

### 69.7 配置项

```yaml
# ~/.ops-ai/config.yaml
edge_computing:
  enabled: true
  
  # 自动检测
  auto_detect: true
  
  # 轻量级模式阈值
  lightweight_mode:
    cpu_threshold_millicores: 4000       # < 4 CPU 核心
    memory_threshold_bytes: 8589934592   # < 8GB 内存
    
  # 网络不稳定适配
  unstable_network:
    max_retries: 5
    base_backoff: "1s"
    max_backoff: "30s"
    circuit_breaker:
      failure_threshold: 3
      timeout: "60s"
      
  # 功能开关
  features:
    runbook_rag: true                   # 在受限环境自动禁用
    compliance_scan: true
    chaos_engineering: false            # 边缘环境默认禁用
    capacity_planning: false
```

### 69.8 System Prompt 补充

```
## 边缘计算支持知识

当运维在边缘/K3s 环境中使用 Agent 时：

1. **自动检测**：
   - Agent 启动时自动检测集群类型（K3s/MicroK8s/标准 K8s）
   - 根据资源水平自动启用轻量级模式

2. **轻量级模式**：
   - 内存限制降低至 128MB
   - 禁用高资源消耗功能（RAG、合规扫描、混沌工程）
   - 保留核心诊断和操作能力

3. **网络不稳定**：
   - 自动重试和指数退避
   - 断路器模式防止级联失败
   - 网络恢复后自动恢复正常操作

4. **ARM 支持**：
   - Agent 镜像支持 arm64 架构
   - 工具链适配 ARM 环境
```

---

# 第六部分：开发路线图 v2.0

## Phase 1: P0 阻断级（3 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 1 | §58 Agent 高可用部署 | StatefulSet + Leader Election + 会话迁移 |
| 1 | §59 Agent 自升级机制 | 滚动升级 + 会话热迁移 + 自动回滚 |
| 2 | §60 大规模集群性能优化 | 缓存 + 分页 + 批量写入 + 影响面限制 |
| 3 | P0 集成测试 + 压力测试 | 千节点集群测试报告 |

## Phase 2: P1 重要级（4 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 4 | §61 服务网格深度运维 | Istio/Linkerd 集成 + Sidecar 诊断 |
| 4 | §62 发布策略辅助 | Argo Rollouts + Flagger 集成 |
| 5 | §63 变更窗口与维护模式 | 维护模式 + 变更窗口 + on-call 集成 |
| 5 | §64 告警降噪与通知疲劳 | 聚类 + 去重 + 静默 + 质量评分 |
| 6-7 | P1 集成测试 + 场景验证 | 发布场景 + 告警风暴场景测试 |

## Phase 3: P2 增强级（3 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 8 | §65 CronJob/批处理 + §66 Agent 调试 | 批处理运维 + 决策追踪 |
| 9 | §67 Terraform 深度 + §68 数据驻留 | TF drift 检测 + 数据脱敏 |
| 9 | §69 边缘计算/K3s | K3s 适配 + 轻量级模式 |
| 10 | 全量回归测试 + 文档完善 | 最终测试报告 |

## 总工期

**10 周**（约 2.5 个月）

---

# 附录 A：Changelog

## v1.9 → v2.0（2026-06-25）— 规模化生产版

### P0 — 阻断级（3 项）
- **§58 Agent 高可用部署模型**：新增 StatefulSet 多副本 + Leader Election、会话故障转移、alertd HA、审计日志 PostgreSQL 后端
- **§59 Agent 自升级机制**：新增滚动升级、会话热迁移、版本兼容性检查、升级后健康验证、自动回滚
- **§60 大规模集群性能优化**：新增扫描结果缓存、API Server 限流、分页列表、增量扫描、影响面分析限制、审计批量写入

### P1 — 重要级（4 项）
- **§61 服务网格深度运维**：新增 Istio/Linkerd 控制平面检查、Sidecar 诊断、VirtualService 冲突检测、流量拓扑可视化、mTLS 故障诊断
- **§62 发布策略辅助**：新增 Argo Rollouts/Flagger 集成、金丝雀/蓝绿/A/B 测试、发布健康检查（基线对比）、自动回滚、策略推荐
- **§63 变更窗口与维护模式**：新增维护模式、变更窗口配置、操作门禁（L2+ 拦截）、on-call 日历集成、预约执行
- **§64 告警降噪与通知疲劳管理**：新增告警聚类（相似度算法）、去重、静默规则、质量评分、成本保护（告警风暴限流）

### P2 — 增强级（5 项）
- **§65 CronJob / 批处理任务运维**：新增 CronJob 监控、Job 失败诊断（退出码分析）、自动历史清理、资源优化
- **§66 Agent 自身调试能力**：新增决策链路追踪、会话回放、LLM 调用分析、幻觉检测、交互式调试模式
- **§67 Terraform / IaC 状态管理深度集成**：新增 TF drift 检测、K8s-TF 资源映射、操作前 TF 检查、Plan 预检辅助
- **§68 数据驻留与主权合规**：新增数据分类、LLM 脱敏、本地模型 fallback、数据流向报告、存储位置控制
- **§69 边缘计算 / K3s 支持**：新增 K3s/MicroK8s 自动检测、轻量级模式、不稳定网络适配（重试+断路器）、ARM 支持

### 版本演进全景

```
v1.0 概念验证版  →  v1.1 可交互版  →  v1.2 安全初版  →  v1.3 生产集群实战版
    ↓
v1.4 可交付版  →  v1.5 企业可用版  →  v1.6 企业可推广版  →  v1.7 产品可交付版
    ↓
v1.8 生产安全版  →  v1.9 生产可靠版 + 企业就绪
    ↓
v2.0 规模化生产版
    (高可用 + 自升级 + 大规模性能 + 服务网格 + 发布策略 + 变更窗口 + 告警降噪)
```

---

**文档版本**：v2.0
**日期**：2026-06-25
**变更**：基于 v1.9 运维视角差距分析，补齐 12 项缺口（3 P0 + 4 P1 + 5 P2）