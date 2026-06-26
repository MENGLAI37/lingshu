# 运维 AI Agent 产品需求文档 (PRD) v1.8

> **文档用途**：面向产品设计师、架构师、开发团队的完整需求规格说明
> **版本**：v1.8 — 生产安全版（补齐会话级爆炸半径控制、GitOps冲突感知、Agent Loop超时防护、多集群上下文安全、变更后验证、依赖服务诊断、集群内网络诊断。从"功能完整"到"生产级安全"的最后闭环。）
> **日期**：2026-06-25
> **变更**：v1.7 → v1.8 补齐了 7 项生产安全缺口（3 阻断 + 4 重要），详见末尾 Changelog

---

# 第一部分：给所有人的 Executive Summary

## 我们要做什么

做一个**终端原生的运维 AI 副驾驶**——像 Cursor/Claude Code 之于开发者一样，让运维工程师在终端里用自然语言完成跨工具链的排查、诊断、修复操作。

## 为什么现在做

| 信号 | 证据 |
|------|------|
| 市场空白 | 市场上 459 个 DevOps AI 工具，没有任何一个同时做到"终端原生 + 跨工具链执行 + 完善安全护栏" |
| 最接近的竞品 | kubectl-ai (Google) 只做 K8s，安全模型有多个绕过漏洞；HolmesGPT 执行能力有限，仅 K8s 生态 |
| 用户痛点 | 运维工程师单次跨工具排查耗时 15-25 分钟，每月累计 80+ 小时花在手动切换工具上 |
| 商业模式验证 | Cursor ($100M+ ARR)、Claude Code 已验证"开源核心 + 企业订阅"路径可行 |

## 一句话定位

> **在终端里用一句话替代 5 个 Dashboard 和 3 个 CLI 工具。**

---

# 第二部分：市场格局 & 竞品矩阵

## 2.1 四象限竞品格局

```
                     可执行 (能改)
                         │
    ┌────────────────────┼────────────────────┐
    │                    │                    │
    │  HolmesGPT         │  SRE.ai            │
    │  (K8s 诊断+修复)    │  ($7.2M, 企业SaaS)  │
    │  CNCF Sandbox      │  Salesforce 中心    │
    │                    │                    │
────┼────────────────────┼────────────────────┤──
    │                    │                    │
    │  K8sGPT            │  kubectl-ai   ★    │
    │  (只诊断，不修复)    │  (只 K8s, 安全弱)   │
    │  31 分析器          │  Google 官方       │
    │                    │  ← 我们在这里打     │
    │                    │                    │
    └────────────────────┴────────────────────┘
                     终端原生 (CLI)
```

## 2.2 竞品详细对比

| 维度 | kubectl-ai | K8sGPT | HolmesGPT | SRE.ai | **我们** |
|------|-----------|--------|-----------|--------|---------|
| 终端原生 | ✅ Go CLI | ✅ Go CLI | ✅ Python CLI | ❌ Web SaaS | ✅ CLI First |
| 操作能力 | ✅ kubectl 读写 | ❌ 只诊断 | ✅ K8s 修复 | ✅ 企业 DevOps | ✅ 全栈读写 |
| 跨工具链 | ❌ 仅 kubectl | ❌ 仅 K8s 诊断 | ❌ K8s+可观测性 | ❌ Salesforce 核心 | ✅ K8s+TF+Cloud+DB+监控 |
| 安全模型 | ⚠️ 二元确认 | ⚠️ 仅脱敏 | ✅ 审批分层 | ❓ 不可见 | ✅ 五级+环境上下文 |
| 爆炸半径预判 | ❌ 无 | ❌ 无 | ❌ 无 | ❓ 不可见 | ✅ 核心特性 |
| 自动快照/回滚 | ❌ 无 | ❌ 无 | ❌ 无 | ❓ 不可见 | ✅ 核心特性 |
| CMDB 上下文 | ❌ 无 | ❌ 无 | ❌ 无 | ❓ | ✅ 核心特性 |
| Runbook RAG | ❌ 无 | ❌ 无 | ❌ 无 | ❌ | ✅ 核心特性 |
| 开源 | ✅ Apache 2.0 | ✅ Apache 2.0 | ✅ Apache 2.0 | ❌ 闭源 SaaS | ✅ 开源核心 |

---

# 第三部分：用户画像 & 核心场景

## 3.1 核心用户画像

### 画像 A：SRE/DevOps 工程师（主力用户）
- 终端占比 80%+，技术栈 K8s + Terraform + Prometheus/Grafana
- 核心痛点：故障排查要在 5 个工具之间切 15-25 分钟，半夜 on-call 容易操作失误
- **人效数据**：平均每 100 人规模的产研团队配备 3-5 个 SRE，排查类工作占 40%+

### 画像 B：平台工程师（影响者/推广者）
- 维护 IDP 和 CI/CD 基础设施，决定团队工具选型

### 画像 C：技术管理者（决策者/付费方）
- VP of Infrastructure / Head of SRE，关注 MTTR 和安全事故率

## 3.2 核心 Jobs-to-be-Done

| JTBD | 当前耗时 | 我们的目标 |
|------|---------|-----------|
| 故障排查（跨 Prometheus+Loki+Jaeger+kubectl） | 15-25 min | 30s 出根因假设 |
| 变更执行（kubectl/terraform apply + 验证） | 10-15 min | 2 min 含自动验证 |
| 容量评估（看 Grafana + Excel 算） | 30-45 min | 2 min 自动报告 |
| 新人 on-call 上手 | 2-3 个月 | Runbook RAG 即时辅助 |
| 合规审计准备 | 2-3 天 | 一键生成 |

## 3.3 覆盖的运维场景（MVP 完整列表）

| 分类 | 具体场景 | MVP 覆盖 |
|------|---------|---------|
| **资源排查** | Pod CrashLoopBackOff / ImagePullBackOff / OOMKilled / Pending | ✅ |
| **性能排查** | CPU/内存飙高、p99 延迟异常 | ✅ (需 Prometheus 集成) |
| **网络排查** | Service 不通、CoreDNS 解析失败、CNI 异常 | ✅ L0 诊断 |
| **变更执行** | 扩缩容、滚动重启、配置更新、回滚 | ✅ L1-L3 |
| **配置管理** | ConfigMap/Secret 热更新、Helm values 修改 | ✅ L1-L2 |
| **节点运维** | drain/cordon/uncordon、节点 NotReady 排查 | ✅ L0-L2 |
| **证书管理** | cert-manager 证书过期检查 | ✅ L0 |
| **容量管理** | HPA 分析、资源使用趋势、瓶颈预测 | ✅ L0 |
| **GitOps 联动** | ArgoCD/Flux 同步状态检查、最近部署记录 | ✅ L0 |

---

# 第四部分：产品需求 — 功能分级

## 4.1 P0 — MVP 必备（6-8 周）

| 功能 | 描述 | 验收标准 |
|------|------|---------|
| **终端对话 TUI** | Bubble Tea 框架，支持多行输入、流式输出、命令预览 | 完成一次完整"排查+修复"对话流程 |
| **K8s 读写工具** | 精确列表见 §9.1，含 helm | 覆盖列表内全部命令 |
| **五级安全网关** | 含环境上下文加权，详 §5.2 | 每个级别有可测试的拦截逻辑 |
| **影响面预判 MVP** | 直接引用关联分析（ConfigMap/Secret→Deployment） | 变更前展示受影响资源数+名称 |
| **自动快照** | 变更前保存资源 YAML 到本地 + 内存 | 快照含时间戳，可被回滚引用 |
| **多 LLM 支持** | OpenAI / Claude / Ollama | 至少 2 云+1 本地 |
| **--dry-run 预览模式** | 完整推理链但不执行任何写操作 | 所有 L1+ 操作展示完整计划 |
| **RBAC 权限自检** | 启动时检查 SA 权限，缺失时提示 | 输出具体缺少的 verb+resource |
| **操作审计日志** | 统一审计 sink（文件/Loki/ELK/stdout），L1+ 操作自动记录 | JSON Lines 格式，不可篡改，异步不阻塞 Agent Loop |
| **集群健康概览** | 启动时自动展示或 /health 命令触发 | 5 维度并行检查：节点/Pod/PDB/配额/部署 |
| **CI/CD 模式** | --no-tui --yes / --pipe 两种无交互模式 | L0-L2 自动确认，L3-L4 拒绝；退出码语义化 |
| **会话导出/导入** | /export 导出会话 JSON，/import 恢复会话上下文 | 导出的会话可在另一台机器上完整恢复（含工具调用历史） |
| **第一时间可运行体验（Onboarding）** | 智能检测 kubeconfig/LLM API key，引导式配置 | 从零到 /health 成功 < 2 分钟 |
| **kubectl logs -f 流式日志** | 支持 tail -f 实时流式输出 + 模式匹配停止 | 可按正则/时间/行数终止流，TUI 实时滚动 |
| **启动崩溃自动恢复** | Agent Loop panic 后自动恢复上次会话 | 重启后提示"检测到未完成会话，是否恢复？" |
| **容器化部署** | Dockerfile + K8s CronJob/CronWorkload 部署模板 | 容器内 --no-tui 模式完整运行，distroless 镜像 |
| **会话级爆炸半径控制** | 同一 session 内 L2+ 操作累计计数 + 熔断机制 | 同一 namespace 连续 3+ 次 L2 操作 → 触发会话熔断 |
| **GitOps 冲突感知** | 检测 ArgoCD/Flux 管理的资源 + 冲突警告 | 修改 GitOps 资源前主动提示将被回滚，给出正确操作路径 |
| **Agent Loop 全局超时** | context.WithDeadline 级别超时 + 死循环检测 | Agent 单次任务最长 5 分钟，重复工具调用模式检测 |

## 4.2 P1 — Beta 必备（+8 周）

Terraform 集成、Prometheus+Loki 集成、CMDB 拓扑上下文、Runbook RAG、会话持久化、安全网关 L3、**告警 Webhook 自动触发（§24）**、**云资源深度诊断（§25）**、**Pre-flight 统一编排框架（§28）**、**用户/团队成本归属（§30）**、**多集群上下文基础安全（§35）**、**变更后自动验证（§36）**、**依赖服务连通性诊断（§37）**、**集群内网络诊断（§38）**

## 4.3 P2 — GA 必备（再+8 周）

多步编排引擎、自动回滚、审计报告、**会话 Live 协作（§23.4）**、VS Code 插件、告警自动处理闭环、**插件/扩展机制（MCP 原生工具注册）**、**多集群跨 context 桥接**、**运维知识库/Runbook 集成（§39）**

## 4.4 MVP 明确不做的事

- ❌ 不代替告警系统 — 但 **v1.6 起支持作为告警消费者自动触发排查**（§24）。我们不发送告警，但 AlertManager/PagerDuty 的告警可以自动唤醒 ops-ai 执行预定义诊断流程。
- ❌ 不自己采集存储监控数据 — 查询外部源
- ❌ 不做 Web Dashboard — MVP 纯终端
- ❌ 不操作生产 DB 数据（DDL 需审批，DML 禁区 L4）
- ❌ 不内置 AI 模型 — 用户自带 API Key 或本地模型

---

# 第五部分：安全网关 — 五级操作分级模型 v1.1

## 5.1 核心变更（v1.0 → v1.1）

v1.0 的致命问题：**脱离环境上下文的分级是无效的。** 同一个 `kubectl rollout restart` 在 staging 是 L1，在 production 是 L3。v1.1 引入**环境上下文权重**。

## 5.2 分级判定逻辑

```
最终风险等级 = 操作固有风险 + 环境上下文权重

操作固有风险（由工具定义时声明）:
  L0_base: 纯只读操作（get, describe, logs, plan, PromQL, loki query）
  L1_base: 低风险修改（scale replicas 在安全范围内, label 修改）
  L2_base: 中等风险修改（rollout restart, 修改资源配置）
  L3_base: 高风险修改（delete resource, apply 大规模变更）
  L4_base: 禁区（操作生产 DB, rm -rf, 未经审批的破坏性操作）

环境上下文权重（运行时动态判定）:
  context/dev     → 权重 -1（比默认降一级）
  context/staging → 权重  0（保持默认）
  context/prod    → 权重 +1（比默认升一级）
  context/prod + 核心服务（支付/鉴权/数据库相关）→ 权重 +2
```

### 完整分级示例

| 操作 | Staging 环境 | Production 普通服务 | Production 支付服务 |
|------|-------------|-------------------|-------------------|
| `kubectl get pods` | L0（自动） | L0（自动） | L0（自动） |
| `terraform plan` | L0（自动） | L0（自动） | L0（自动） |
| `kubectl scale --replicas=3` | L1（回车确认） | L2（输入 yes） | L2（输入 yes） |
| `kubectl rollout restart` | L2（输入 yes） | L3（输入集群名确认） | L3（双人审批） |
| `kubectl delete namespace` | L3（输入集群名确认） | L4（拒绝） | L4（拒绝） |
| `rm -rf /` | L4（拒绝） | L4（拒绝） | L4（拒绝） |

## 5.3 各级别具体行为

| 级别 | 行为 | 用户交互 |
|------|------|---------|
| **L0** | 自动执行，结果直接流式输出到终端 | 无需 |
| **L1** | 显示命令预览 + 简短说明。用户按 Enter 确认，Esc 取消 | 一键确认 |
| **L2** | 显示：影响面分析 + 完整命令 + pre-flight check 结果。用户输入 `yes` 确认 | 明确确认 |
| **L3** | 显示：影响面热力图 + 回滚方案 + 强制自动快照 ID + 预计影响时长。用户输入当前集群名确认。可选双人审批（通过审批 token） | 强确认 |
| **L4** | 直接拒绝，解释原因，给出安全替代方案建议 | 不可执行 |

## 5.4 环境识别机制

```
环境来源（优先级从高到低）:
1. 用户显式指定: ops-ai --env production
2. kubeconfig current-context 解析: 从 context name 提取关键词
   - 包含 "prod" / "production" / "live" → production
   - 包含 "stag" / "staging" / "uat" → staging
   - 包含 "dev" / "development" / "local" → dev
   - 其他 → 默认 production（安全优先原则）
3. 配置文件: ~/.ops-ai/config.yaml 中 environment_labels 字段
```

## 5.5 核心服务标记

```
核心服务判定（满足任一即为核心服务）:
1. 用户显式标记: 在 config.yaml 的 critical_services 列表中
2. 资源标签: metadata.labels["ops-ai/critical"] == "true"
3. 默认关键词匹配: 资源名包含 "payment" / "auth" / "db" / "database" / "gateway" / "billing"
```

## 5.6 Secret 三层处理策略（v1.2 新增）

这是安全网关最容易被忽略的致命缺口。运维排障必然需要接触 Secret 内容，但绝不能把生产密码明文发给 LLM API。

```
三层策略:

┌─ LLM 层（发送前过滤）─────────────────────────────┐
│ 始终 Redact 的 key 名模式:                          │
│   password, token, key, secret, credentials,       │
│   private_key, tls.key, ca.crt, client-certificate │
│                                                     │
│ 保留（非敏感）的 key 名模式:                         │
│   username, endpoint, port, database, host, url,    │
│   type, name, namespace, .dockerconfigjson 的 repo  │
│                                                     │
│ Redact 后的格式: "pa••••rd"（前 2 + 后 2，中间隐藏） │
│ 或完全替换为 [REDACTED]（对高熵 token 类型 key）     │
└─────────────────────────────────────────────────────┘

┌─ Agent 层（内存中保留完整值）───────────────────────┐
│ - client-go 读取 Secret 时保留完整 data 在内存       │
│ - 变更 Secret（kubectl apply/create）由 Agent 直接   │
│   调用 client-go，不经过 LLM 回环                    │
│ - 仅在 LLM 请求执行 Secret 变更时，Agent 自闭环完成:  │
│   1. Agent 从内存中取出完整 Secret 值                │
│   2. 执行 kubectl apply                              │
│   3. 将 redacted 结果回传给 LLM 上下文               │
└─────────────────────────────────────────────────────┘

┌─ 审计层（日志脱敏）────────────────────────────────┐
│ - 操作 Secret 时日志记录:                            │
│   "tool=kubectl_apply resource=secret/db-creds       │
│    keys_modified=[username,password] action=update"  │
│ - 绝不记录 Secret 的实际值到日志或 SQLite             │
│ - Secret 快照单独加密存储（AES-256-GCM），与普通      │
│   资源快照隔离，快照密钥从环境变量 OPS_AI_SNAPSHOT_KEY │
│   读取（不存在则拒绝创建 Secret 快照）                │
└─────────────────────────────────────────────────────┘
```

配置示例：

```yaml
# config.yaml
secret_policy:
  redact_patterns:           # 默认 redact 的 key 名正则
    - "password"
    - "token"
    - ".*key$"
    - ".*secret$"
    - ".*certificate$"
  preserve_patterns:         # 始终保留的 key 名正则（优先级高于 redact）
    - "type"
    - "name$"
    - "namespace$"
  snapshot_encryption: true  # Secret 快照单独加密
```

## 5.7 Admission Webhook 感知（v1.3 新增）

生产集群 90% 以上部署了 OPA/Gatekeeper 或 Kyverno。Agent 的安全网关通过的操作，可能被集群侧的 ValidatingWebhook / MutatingWebhook 拒绝。Agent 必须能感知并解释这类失败。

```
场景还原:
  Agent 安全网关: ✅ L2 通过（kubectl scale 合法）
  → kubectl scale deployment/payment --replicas=5
  → API Server 受理
  → ValidatingWebhook: ❌ "副本数超过 Budget 策略限制 max=4"
  → Agent 收到: Error from server (Forbidden): admission webhook "validate.kyverno.io" denied
  → v1.2 行为: Agent 不知道 Webhook 存在，反复建议同样被拒的配置
  → v1.3 行为: Agent 识别 Webhook 拒绝 → 展示策略详情 → 建议合规的替代操作
```

### Pre-flight 集成（L2+ 操作自动触发）

```go
// AdmissionWebhookPreflight 在所有 L2+ 操作前自动执行
type AdmissionWebhookPreflight struct{}

func (p *AdmissionWebhookPreflight) Run(ctx ToolContext) []PreflightResult {
    results := []PreflightResult{}
    
    // 1. 获取集群中所有 Webhook 配置
    validating, _ := ctx.K8sClient.AdmissionregistrationV1().
        ValidatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
    mutating, _ := ctx.K8sClient.AdmissionregistrationV1().
        MutatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
    
    // 2. 检查是否有 Webhook 匹配当前操作的目标资源类型
    resourceType := ctx.TargetResource.Kind  // e.g. "Deployment"
    namespace := ctx.TargetResource.Namespace
    
    for _, wh := range validating.Items {
        for _, rule := range wh.Webhooks[0].Rules {
            if matchesResource(rule, resourceType) && matchesScope(wh, namespace) {
                results = append(results, PreflightResult{
                    CheckName: "AdmissionWebhook",
                    Passed:    true, // 不阻止执行，只是告知风险
                    Detail:    fmt.Sprintf(
                        "集群存在 ValidatingWebhook: %s (匹配 %s)。操作可能被此策略拒绝。",
                        wh.Name, resourceType,
                    ),
                })
            }
        }
    }
    
    return results
}
```

### Webhook 拒绝时的 Agent 行为

```
当 kubectl apply/scale 等收到 webhook 拒绝错误时:
  → Agent 不应盲目重试
  → 自动执行: kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations
  → 解析 webhook 规则，找到拒绝原因（annotation message 通常解释策略）
  → 输出给用户:
    "操作被集群准入策略拒绝:
     Webhook: validate.kyverno.io/budget-policy
     原因: 副本数 5 超过该服务 Budget 策略上限 4
     建议: 先修改 Kyverno ClusterPolicy 'budget-policy'，
           或使用 kubectl scale --replicas=4"
  → 如果 Webhook 有临时豁免 annotation（如 kyverno.io/exclude），
    Agent 可以建议（但绝不自动添加豁免 annotation）
```

### L0 命令扩展

```
kubectl get validatingwebhookconfigurations           # L0: 查看所有 ValidatingWebhook
kubectl get mutatingwebhookconfigurations             # L0: 查看所有 MutatingWebhook
kubectl describe validatingwebhookconfiguration <name> # L0: 查看具体 Webhook 规则
```

## 5.8 PDB（PodDisruptionBudget）感知（v1.3 新增）

`kubectl drain` 和 `kubectl delete pod`（驱逐场景）是高频运维操作，但 PDB 可以让它们静默卡住几分钟。

```
场景还原:
  Agent: kubectl drain node-3 --ignore-daemonsets
  → 等了 5 分钟……超时了
  → 原因: PDB 限制 payment 服务最多 1 个不可用 Pod，drain 要驱逐第 2 个 Pod 时被 PDB 阻止
  → v1.2 行为: Agent 报告超时但不懂为什么
  → v1.3 行为: Agent 预检 PDB + 解释阻塞原因
```

### Pre-drain 自动检查

```go
// PDBPreflight 在 drain 操作前自动执行
func (p *PDBPreflight) Run(ctx ToolContext) []PreflightResult {
    results := []PreflightResult{}
    
    // 1. 获取目标节点上的所有 Pod
    pods, _ := ctx.K8sClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
        FieldSelector: fmt.Sprintf("spec.nodeName=%s", ctx.TargetNodeName),
    })
    
    // 2. 对于每个 Pod，检查所在 namespace 的 PDB
    seenNamespaces := map[string]bool{}
    for _, pod := range pods.Items {
        if seenNamespaces[pod.Namespace] {
            continue
        }
        seenNamespaces[pod.Namespace] = true
        
        pdbs, _ := ctx.K8sClient.PolicyV1().PodDisruptionBudgets(pod.Namespace).
            List(context.Background(), metav1.ListOptions{})
        
        for _, pdb := range pdbs.Items {
            // 检查 PDB 的 selector 是否匹配这个 Pod 的 labels
            selector, _ := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
            if selector.Matches(labels.Set(pod.Labels)) {
                available := pdb.Status.DesiredHealthy - pdb.Status.CurrentHealthy
                if available <= 0 {
                    results = append(results, PreflightResult{
                        CheckName: "PDB",
                        Passed:    false,
                        Detail: fmt.Sprintf(
                            "⚠️  PDB %s/%s 限制: 当前健康副本已达最低要求 (%d/%d)。"+
                                "drain 此节点可能被 PDB 阻塞。",
                            pdb.Namespace, pdb.Name,
                            pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy,
                        ),
                    })
                }
            }
        }
    }
    
    if len(results) == 0 {
        results = append(results, PreflightResult{
            CheckName: "PDB", Passed: true, Detail: "无 PDB 阻塞风险",
        })
    }
    
    return results
}
```

### L0 命令扩展

```
kubectl get pdb -A                                      # L0: 查看所有 PDB
kubectl get pdb -n <ns>                                 # L0: 查看指定 namespace 的 PDB
kubectl describe pdb <name> -n <ns>                     # L0: 查看 PDB 详情（含状态）
```

### drain 命令预检增强

执行 `kubectl drain` 时，Agent 先自动执行以上 PDB 检查 + 输出阻塞预警。如果 PDB 会阻塞 drain，Agent 提前警告并给出建议：

```
"检测到以下 PDB 可能在 drain 时造成阻塞:
  PDB payment-min-available: 最少需要 2 个健康 Pod，当前仅 2 个 (无余量)
  PDB auth-min-available: 最少需要 1 个健康 Pod，当前 2 个 (安全)
  
  drain node-3 可能导致 payment PDB 阻塞。建议:
  1. 先扩容 payment 到 3 副本: kubectl scale deployment/payment --replicas=3
  2. 等待新 Pod Ready 后再 drain
  3. 或使用 --disable-eviction 强制删除 Pod（绕过 PDB，有服务中断风险）"
```

## 5.9 PSA（Pod Security Standards）感知（v1.3 新增）

K8s 1.25+ GA 的 Pod Security Admission。几乎所有生产集群都开了 `restricted`。Agent 创建/修改 Pod 时不知道 PSA 限制会导致反复失败。

```
场景:
  用户: "帮我在 production 起一个调试 Pod"
  Agent: kubectl run debug --image=busybox --privileged
  → Pod 创建失败: "violates PodSecurity 'restricted:latest'"
  → v1.2: Agent 反复建议同样被拒的配置
  → v1.3: Agent 先检查 namespace 的 PSA label → 自动生成合规配置
```

### PSA 感知流程

```go
// PSAPreflight 在涉及 Pod 创建的 L2+ 操作前执行
func (p *PSAPreflight) Run(ctx ToolContext) []PreflightResult {
    ns, _ := ctx.K8sClient.CoreV1().Namespaces().Get(
        context.Background(), ctx.Namespace, metav1.GetOptions{})
    
    labels := ns.Labels
    enforceLevel := labels["pod-security.kubernetes.io/enforce"]
    if enforceLevel == "" {
        enforceLevel = "privileged" // 未设置则无限制
    }
    
    results := []PreflightResult{}
    
    switch enforceLevel {
    case "restricted":
        results = append(results, PreflightResult{
            CheckName: "PSA",
            Passed:    true, // 不阻止，只是告知
            Detail: fmt.Sprintf(
                "此 namespace 的 PSA 强制级别为 restricted。Pod 配置必须满足:\n"+
                "  - 禁止 privileged 容器\n"+
                "  - 禁止 hostNetwork/hostPID/hostIPC\n"+
                "  - 必须设置 seccompProfile (RuntimeDefault 或 Localhost)\n"+
                "  - 必须 drop ALL capabilities\n"+
                "  - 禁止使用 hostPath volumes（除非限定类型）\n"+
                "  - 必须以非 root 用户运行",
            ),
        })
    case "baseline":
        results = append(results, PreflightResult{
            CheckName: "PSA", Passed: true,
            Detail: "此 namespace 的 PSA 级别为 baseline。禁止已知的特权提升手段。",
        })
    }
    
    return results
}
```

### Agent 的 PSA 合规建议

```
当用户要求创建 Pod/Deployment 时:
  → Agent 自动检查目标 namespace 的 PSA label
  → 如果 enforce=restricted:
    Agent 在建议的 YAML 中自动添加:
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
        capabilities:
          drop: ["ALL"]
    Agent 绝不建议 --privileged 或 hostNetwork
  → 如果用户坚持使用 restricted 禁止的配置（如 privileged）:
    Agent 解释 PSA 规则，建议修改 namespace label 或调整 Pod 配置
```

### L0 命令扩展

```
kubectl get namespace <ns> -o yaml | grep pod-security    # L0: 查看 namespace PSA label
kubectl label namespace <ns> --list                         # L0: 查看 namespace 所有 label
```

## 5.10 Operator/CRD ownerReferences 感知（v1.3 新增）

生产 K8s 集群里到处是 Operator（Prometheus Operator、cert-manager、Istio、ArgoCD）。Agent 直接修改 Operator 管控的 Deployment 会被 Operator 回滚——用户操作看起来"成功"了但 30 秒后恢复原状。

```
场景:
  Agent: kubectl scale deployment/prometheus-server --replicas=3
  → 成功了！
  → 30 秒后 Prometheus Operator 又把它缩回 1
  → 因为真正的控制者是 Prometheus CR，不是 Deployment
  
  v1.3 行为:
  Agent 在修改前检查 ownerReferences → 发现是 Operator 管的
  → 警告用户: "此 Deployment 由 Prometheus CR 'k8s' 管控。
      直接修改会在 ~30 秒内被 Operator 覆盖。
      建议修改: kubectl edit prometheus k8s 并调整 replicas 字段。"
```

### OwnerReference 检测

```go
// OperatorAwareness 在修改任何 K8s 资源前执行
func (o *OperatorAwareness) Check(ctx ToolContext, obj metav1.Object) *OperatorWarning {
    owners := obj.GetOwnerReferences()
    
    for _, owner := range owners {
        // 检查 owner 是否是 CRD（Operator 管理的典型特征）
        if isCRDOwner(owner) {
            return &OperatorWarning{
                ManagedBy:    fmt.Sprintf("%s/%s", owner.Kind, owner.Name),
                OwnerAPIGroup: *owner.APIVersion,
                Explanation: fmt.Sprintf(
                    "此 %s 由 %s '%s' 管控。直接修改会在短时间内被 Operator 覆盖。\n"+
                    "建议修改对应的 CR（Custom Resource）而非此工作负载。",
                    obj.GetObjectKind().GvkKindString(), owner.Kind, owner.Name,
                ),
                SuggestedAction: fmt.Sprintf(
                    "kubectl edit %s %s", strings.ToLower(owner.Kind), owner.Name,
                ),
            }
        }
    }
    
    return nil // 无 Operator 管控
}

func isCRDOwner(owner metav1.OwnerReference) bool {
    // CRD 的 API group 通常不是 "apps" 或空（标准资源）
    apiGroup := ""
    if owner.APIVersion != "" {
        parts := strings.Split(owner.APIVersion, "/")
        if len(parts) > 1 {
            apiGroup = parts[0]
        }
    }
    
    // 标准 K8s 资源的 API group
    standardGroups := map[string]bool{
        "": true, "apps": true, "batch": true, "autoscaling": true,
        "policy": true, "networking.k8s.io": true, "rbac.authorization.k8s.io": true,
        "storage.k8s.io": true, "apiextensions.k8s.io": true,
    }
    
    return !standardGroups[apiGroup]
}
```

### Agent 行为变更

```
修改资源前（L1-L3 操作，不仅限于 Deployment，StatefulSet/DaemonSet 也同样）:
  → Agent 在 DryRun 阶段自动检查 ownerReferences
  → 如果检测到 Operator 管控:
    - 警告信息注入到影响面分析的 Warnings 列表
    - DryRunResult.SafeToProceed 仍然为 true（不阻止操作，只是告知风险）
    - 执行确认时额外提示
  → 用户可以选择:
    1. "继续直接修改" — 了解风险后仍执行（Operator 可能会覆盖）
    2. "修改 CR" — Agent 转而操作对应的 CR
    3. "取消" — 放弃操作
```

### L0 命令扩展

```
kubectl get <resource> <name> -o json | jq '.metadata.ownerReferences'  # L0: 查看所有者引用
```

## 5.11 ResourceQuota / LimitRange 感知（v1.4 新增）

生产集群几乎都设置了 ResourceQuota 和 LimitRange。Agent 的安全网关放行的操作，可能被集群资源配额拒绝——这是比 Webhook 更基础的阻断。

```
场景:
  Agent: kubectl scale deployment/payment --replicas=10
  → API Server: Error: exceeded quota "compute-resources":
    requested: cpu=20, used: cpu=85, limited: cpu=100
  → v1.3: Agent 把配额错误当作通用 403 处理 → "检查 RBAC 权限"
  → v1.4: Agent 识别 ResourceQuota 拒绝 → 展示配额详情 → 建议合规替代方案
```

### Pre-flight 集成（L2+ 扩容操作自动触发）

```go
func (rq *ResourceQuotaPreflight) Run(ctx ToolContext) []PreflightResult {
    results := []PreflightResult{}
    
    // 1. 获取目标 namespace 的所有 ResourceQuota
    quotas, _ := ctx.K8sClient.CoreV1().ResourceQuotas(ctx.Namespace).
        List(context.Background(), metav1.ListOptions{})
    
    // 2. 获取所有 LimitRange
    limits, _ := ctx.K8sClient.CoreV1().LimitRanges(ctx.Namespace).
        List(context.Background(), metav1.ListOptions{})
    
    // 3. 如果是扩容操作，计算是否会超出配额
    if ctx.Operation == "scale_up" {
        currentUsage := getCurrentResourceUsage(ctx) // 汇总当前所有 Pod 的 requests
        for _, quota := range quotas.Items {
            if cpuLimit, ok := quota.Spec.Hard["requests.cpu"]; ok {
                remaining := cpuLimit.DeepCopy()
                remaining.Sub(currentUsage.CPU)
                if remaining.Sign() <= 0 {
                    results = append(results, PreflightResult{
                        CheckName: "ResourceQuota",
                        Passed:    false,
                        Detail: fmt.Sprintf(
                            "⚠️  ResourceQuota '%s': CPU 配额已用尽 (%s/%s)。"+
                                "扩容将被拒绝。建议: 降低副本数、扩容集群节点或调整 ResourceQuota。",
                            quota.Name, currentUsage.CPU.String(), cpuLimit.String(),
                        ),
                    })
                }
            }
        }
    }
    
    // 4. LimitRange 检查 — 确保新 Pod 的 requests/limits 在范围内
    for _, limit := range limits.Items {
        for _, item := range limit.Spec.Limits {
            if item.Type == corev1.LimitTypeContainer {
                results = append(results, PreflightResult{
                    CheckName: "LimitRange",
                    Passed:    true,
                    Detail: fmt.Sprintf(
                        "LimitRange '%s': 容器资源限制范围 min=%v, max=%v, default=%v",
                        limit.Name, item.Min, item.Max, item.Default,
                    ),
                })
            }
        }
    }
    
    return results
}
```

### L0 命令扩展

```
kubectl get resourcequota -n <ns>                         # L0: 查看配额
kubectl describe resourcequota <name> -n <ns>             # L0: 配额详情
kubectl get limitrange -n <ns>                            # L0: 查看限制范围
kubectl describe limitrange <name> -n <ns>                # L0: 限制范围详情
```

### Agent 行为

```
当操作被 ResourceQuota 拒绝时:
  → 自动执行 kubectl describe resourcequota -n <ns>
  → 解析配额限制 vs 当前使用量
  → 输出:
    "操作被 ResourceQuota 'compute-resources' 拒绝:
     
     当前使用: cpu=85/100, memory=120Gi/200Gi, pods=42/50
     请求新增: cpu=20 (总计 105 > 100)
     
     建议:
     1. 降低本次扩容规模: kubectl scale deployment/payment --replicas=7
        (预计使用 cpu=94/100，仍在配额内)
     2. 临时提高 ResourceQuota: kubectl edit resourcequota compute-resources
     3. 扩容集群节点后提高配额"
```

## 5.12 NetworkPolicy / Service Mesh 网络盲区（v1.4 新增）

生产集群中网络不通的第一原因往往不是 Service 配错，而是 NetworkPolicy 或 Service Mesh sidecar 阻断了流量。v1.3 的节点诊断覆盖了节点层面的网络问题，但没有覆盖 Pod 间网络策略。

```
场景 A — NetworkPolicy 阻断:
  用户: "payment 连不上 order-svc，帮我排查"
  Agent kubectl get svc → 正常
  Agent kubectl get endpoints → 正常
  Agent kubectl exec payment-pod -- curl order-svc:8080 → 超时
  Agent kubectl exec payment-pod -- curl <order-pod-ip>:8080 → 正常！
  → 结论: Service ClusterIP 不通但 Pod IP 通 → 高度疑似 NetworkPolicy
  → v1.4: Agent 自动检查 NetworkPolicy → 找到阻断规则 → 输出分析

场景 B — Service Mesh (Istio/Linkerd):
  用户: "payment → order:8080 超时"
  Agent 检查 Service、Endpoints、Pod → 全部正常
  → v1.4: Agent 检测 Pod 是否有 Istio sidecar → 检查 AuthorizationPolicy
  → 发现: AuthorizationPolicy 只允许特定 source principal
```

### Pre-flight 网络诊断增强

```go
// NetworkPolicyCheck 在网络诊断场景中自动触发
func (np *NetworkPolicyCheck) Run(ctx ToolContext, sourcePod, targetSvc string) []PreflightResult {
    results := []PreflightResult{}
    
    // 1. 检查目标 namespace 是否有 NetworkPolicy
    netpols, _ := ctx.K8sClient.NetworkingV1().NetworkPolicies(ctx.Namespace).
        List(context.Background(), metav1.ListOptions{})
    
    if len(netpols.Items) > 0 {
        for _, np := range netpols.Items {
            results = append(results, PreflightResult{
                CheckName: "NetworkPolicy",
                Passed:    true, // 仅告知，不阻止
                Detail: fmt.Sprintf(
                    "检测到 NetworkPolicy '%s' (namespace: %s)。"+
                        "如果当前网络不通，此策略可能是原因。",
                    np.Name, np.Namespace,
                ),
            })
        }
    }
    
    // 2. 检测 Service Mesh sidecar (Istio/Linkerd)
    pods, _ := ctx.K8sClient.CoreV1().Pods(ctx.Namespace).
        List(context.Background(), metav1.ListOptions{})
    
    for _, pod := range pods.Items {
        for _, container := range pod.Spec.Containers {
            if strings.Contains(container.Image, "istio/proxyv2") {
                results = append(results, PreflightResult{
                    CheckName: "ServiceMesh",
                    Passed:    true,
                    Detail: fmt.Sprintf(
                        "检测到 Istio sidecar (Pod: %s)。"+
                            "网络问题可能与 Istio AuthorizationPolicy 或 DestinationRule 相关。"+
                            "建议: kubectl get authorizationpolicy -n %s / kubectl get destinationrule -n %s",
                        pod.Name, ctx.Namespace, ctx.Namespace,
                    ),
                })
                break
            }
            if strings.Contains(container.Image, "l5d-proxy") { // Linkerd
                results = append(results, PreflightResult{
                    CheckName: "ServiceMesh",
                    Passed:    true,
                    Detail: "检测到 Linkerd sidecar。建议: linkerd viz tap 查看流量。",
                })
                break
            }
        }
    }
    
    return results
}
```

### L0 命令扩展

```
kubectl get networkpolicies -A                              # L0: 所有 NetworkPolicy
kubectl get networkpolicies -n <ns>                         # L0: 指定 namespace
kubectl describe networkpolicy <name> -n <ns>               # L0: 策略详情
kubectl get authorizationpolicies -n <ns>                    # L0: Istio AuthorizationPolicy
kubectl get destinationrules -n <ns>                         # L0: Istio DestinationRule
```

### System Prompt 补充

```
NETWORK TROUBLESHOOTING KNOWLEDGE:

When a Pod-to-Service connection fails but Pod IP works:
1. Check NetworkPolicy first: kubectl get networkpolicies -n <ns>
2. Check if CNI supports NetworkPolicy (Calico/Cilium do; Flannel default doesn't)
3. Check Service Mesh: look for istio-proxy or linkerd-proxy containers
4. Check kube-proxy mode: iptables vs IPVS (IPVS has known issues with some CNIs)

Order of checks:
1. kubectl get svc, get endpoints (Service registration)
2. curl Pod-IP directly (bypass Service)
3. If Pod-IP works but Service-IP doesn't → NetworkPolicy or kube-proxy issue
4. If neither works → CNI issue or container network namespace problem
5. If Istio detected → check AuthorizationPolicy and DestinationRule
```

## 5.13 生产命名空间确认机制（v1.4 新增）

运维中最常见的人为失误：以为自己在 staging，实际上在 production 执行了操作。v1.3 的安全网关有环境上下文权重，但缺少**主动防呆**。

```
典型事故:
  运维: "帮我把 canary 的副本调到 0"（他以为自己在 staging namespace）
  Agent: kubectl scale deployment/payment-canary --replicas=0
  → 实际在 production namespace 执行了！
  → current-context 的 namespace 是 production
  → canary 其实是 payment 的生产金丝雀部署
  → 5 分钟后 PagerDuty 炸了
```

### L2+ 操作的 namespace 显式确认

```go
// NamespaceConfirmation 在 L2-L3 操作时检查
func (nc *NamespaceConfirmation) Check(ctx ToolContext, level RiskLevel) *NSConfirmResult {
    if level < RiskLevelMedium {
        return nil // L0-L1 不需要
    }
    
    result := &NSConfirmResult{
        DisplayNS:     ctx.Namespace,
        IsProduction:  ctx.Environment == EnvProduction,
        IsCriticalSvc: ctx.IsCriticalSvc,
    }
    
    // 组合判断是否需要额外确认
    if ctx.Environment == EnvProduction {
        result.RequiresNSConfirm = true
        result.ConfirmMessage = fmt.Sprintf(
            "⚠️  此操作目标为生产命名空间: %s\n"+
            "   集群: %s\n"+
            "   资源: %s/%s\n"+
            "   输入命名空间名称确认继续: ",
            ctx.Namespace, ctx.ClusterName,
            ctx.TargetResource.Kind, ctx.TargetResource.Name,
        )
    }
    
    if ctx.IsCriticalSvc {
        result.RequiresNSConfirm = true
        result.ConfirmMessage += fmt.Sprintf(
            "\n🔴  此资源被标记为核心服务（%s）。操作影响业务关键路径。", ctx.TargetResource.Name,
        )
    }
    
    return result
}
```

### TUI 确认交互

```
用户: "把 canary 副本调到 0"
Agent 分析:
  目标: deployment/payment-canary
  命名空间: production ⚠️
  集群: prod-us-east-1

⚠️  此操作目标为生产命名空间: production
   集群: prod-us-east-1
   资源: deployment/payment-canary
   当前副本数: 3 → 目标: 0
   
   输入命名空间名称确认继续: _
   
   (直接输入 "production" 确认，其他任意键取消)
```

### 配置项

```yaml
# config.yaml
namespace_confirmation:
  enabled: true                # 是否启用 namespace 确认
  require_on_production: true  # 生产环境始终需要
  require_on_critical: true    # 核心服务始终需要
  skip_on_staging: false       # staging 环境也需要（更安全）
  skip_on_dev: true            # dev 环境跳过
  confirm_format: "type"       # "type": 输入 namespace | "yes": 输入 yes
```

---

## 5.14 会话级爆炸半径控制（v1.8 新增）

### 问题场景

安全网关 §5.2-5.13 覆盖了每条工具调用的安全检查，但**缺少会话级的聚合视角**。

```
运维: "把 payment namespace 下资源占用高的 deployment 缩容"
Agent 理解: "payment namespace 下所有 deployment 缩到 0"
Agent 执行:
  1. kubectl scale deploy/payment-api --replicas=0     → L2 确认通过 ✅
  2. kubectl scale deploy/payment-worker --replicas=0  → L2 确认通过 ✅
  3. kubectl scale deploy/payment-scheduler --replicas=0 → L2 确认通过 ✅
  结果: 整个 payment namespace 停服 🔴
```

每条单独操作都合理，但聚合效果是灾难。需要会话级的"不能在一个 namespace 里连续执行太多高风险操作"保护。

### Go 接口定义

```go
// SessionBlastRadius 会话级爆炸半径追踪器
type SessionBlastRadius struct {
    mu            sync.Mutex
    sessionID     string
    clusterName   string
    nsOpCount     map[string]*NamespaceOpCounter
    config        BlastRadiusConfig
    fused         bool
    fusedAt       time.Time
}

type NamespaceOpCounter struct {
    Namespace string
    L2Count   int
    L3Count   int
    Resources []ResourceRef
    LastOp    time.Time
}

type ResourceRef struct {
    Kind      string
    Name      string
    Operation string
    Replicas  *int32
}

type BlastRadiusConfig struct {
    MaxL2PerNS      int   // 同一 namespace 最大 L2 操作数 (default: 3)
    MaxL3PerNS      int   // 同一 namespace 最大 L3 操作数 (default: 1)
    MaxTotalL2      int   // 单次会话最大 L2 操作总数 (default: 5)
    CooldownMinutes int   // 熔断冷却时间 (default: 10)
}
```

### TUI 熔断展示

```
  ═══════════════════════════════════════════════════════
  🚨  会话熔断触发
  ═══════════════════════════════════════════════════════

  在 payment namespace 中已执行 4 项 L2 变更操作：

  1. scale deploy/payment-api --replicas=0       ✅ 已执行 (14:02:15)
  2. scale deploy/payment-worker --replicas=0    ✅ 已执行 (14:02:22)
  3. scale deploy/payment-scheduler --replicas=0 ✅ 已执行 (14:02:28)
  4. scale deploy/payment-gateway --replicas=0   🛑  已拦截 (本次)

  累计影响: 4 个 Deployment / payment namespace
  风险等级: 🔴 高风险 — 同一 namespace 大面积变更

  [A] 人工审核全部操作  [R] 回滚已执行的变更  [C] 取消
```

### 配置项

```yaml
safety:
  blast_radius:
    enabled: true
    max_l2_per_namespace: 3
    max_l3_per_namespace: 1
    max_total_l2_per_session: 5
    cooldown_minutes: 10
    unlimited_namespaces:
      - "dev-*"
      - "staging"
      - "sandbox-*"
```

---

## 5.15 GitOps 冲突感知与协调（v1.8 新增）

### 问题场景

如果集群由 ArgoCD/Flux 管理，ops-ai 手动变更 → GitOps controller self-healing → 回滚 → 对抗循环。

```
凌晨 3 点 → 运维通过 ops-ai 紧急扩容 payment-api 到 5 副本
3 分钟后 → ArgoCD sync → 副本数回滚到 Git 定义值 3
→ 告警再次触发 → 运维困惑："我明明扩了" → 对抗循环 🔴
```

### Go 接口定义

```go
type GitOpsDetector struct {
    clientset kubernetes.Interface
}

type GitOpsOwnership struct {
    IsManaged       bool
    Controller      GitOpsController
    AppName         string
    SourceRepo      string
    SyncPolicy      string
    PruneEnabled    bool
    SelfHealEnabled bool
}

type GitOpsController string
const (
    GitOpsArgoCD GitOpsController = "argocd"
    GitOpsFlux   GitOpsController = "flux"
    GitOpsNone   GitOpsController = "none"
)

// DetectOwnership 通过 annotation/label/ownerReference 检测 GitOps 管理
func (gd *GitOpsDetector) DetectOwnership(ctx context.Context, resource ResourceRef) (*GitOpsOwnership, error)
```

### TUI 警告展示

```
  ═══════════════════════════════════════════════════════
  ⚠️  GitOps 冲突警告
  ═══════════════════════════════════════════════════════

  即将修改: deploy/payment-api (scale 3 → 5)
  
  🔄  此资源由 ArgoCD 管理
      Application: payment | 同步策略: automated (self-heal: ON)
      预计回滚时间: ~3 分钟

  ⚠️  如果直接修改，ArgoCD 将在 3 分钟内将副本数恢复为 3。
      你的扩容操作不会持续生效。

  [A] 修改 Git 仓库定义  [B] 暂停 sync 后修改  [C] 仍然修改  [X] 取消
```

### 变更后回滚验证

修改 GitOps 管理的资源后，ops-ai 在 60s 后轮询验证值未被回滚。如果发现回滚 → 主动告知并建议正确操作路径。

---

## 5.16 安全网关检查执行顺序

Pre-flight 编排框架（§28）中的所有检查器，按以下优先级并行执行：

```
1.  §5.2    分级判定（操作固有风险 + 环境上下文权重）
2.  §5.4    环境识别（dev/staging/prod）
3.  §5.5    核心服务判定
4.  §5.6    Secret 三层处理
5.  §5.7    Admission Webhook 检测
6.  §5.8    PDB 阻塞检测
7.  §5.9    PSA 感知
8.  §5.10   Operator ownerReferences 检测
9.  §5.11   ResourceQuota/LimitRange 检查
10. §5.12   NetworkPolicy/Service Mesh 感知
11. §5.13   生产 namespace 确认
12. §5.14   会话级爆炸半径检查（v1.8 新增）
13. §5.15   GitOps 冲突检测（v1.8 新增）
```

---

# 第六部分：影响面分析引擎规格 v1.1

## 6.1 MVP 范围：直接引用关联分析

MVP 只做一层直接关联。复杂拓扑分析放 Phase 2。

```
分析流程（伪代码）:

function analyzeImpact(targetResource):
    results = {affected: [], type: "", confidence: 0.9}

    if targetResource.kind == "ConfigMap":
        # 查找所有引用此 ConfigMap 的工作负载
        for deploy in namespace.deployments:
            if deploy references targetResource via:
                - env[].valueFrom.configMapKeyRef.name
                - envFrom[].configMapRef.name
                - volumes[].configMap.name
                → results.affected.append(deploy)

    if targetResource.kind == "Secret":
        # 同上，检查 secretKeyRef / secretRef
        for deploy in namespace.deployments:
            if deploy references targetResource via secretRef
                → results.affected.append(deploy)

    if targetResource.kind in ["Deployment", "StatefulSet", "DaemonSet"]:
        # 检查关联的 Service
        for svc in namespace.services:
            if svc.spec.selector matches targetResource.spec.selector.matchLabels
                → results.affected.append(svc)
        # 检查关联的 HPA
        for hpa in namespace.hpas:
            if hpa.spec.scaleTargetRef.name == targetResource.name
                → results.affected.append(hpa)

    if targetResource.kind == "Service":
        # 检查关联的 Ingress
        for ing in namespace.ingresses:
            if ing references targetResource
                → results.affected.append(ing)

    # 去重 + 排序（按影响严重程度）
    return results
```

## 6.2 影响面输出格式（终端显示）

```
⚠️  影响面分析 — kubectl rollout restart deployment/payment (L2 → production)

  直接受影响:
  ├─ Deployment/payment          ← 目标资源，滚动重启预计 30-60s
  ├─ Service/payment             ← 通过 label selector 关联
  ├─ HPA/payment-autoscaler      ← 通过 scaleTargetRef 关联
  └─ Ingress/payment-api         ← 通过 Service 间接引用

  间接影响:
  └─ 下游服务 3 个（user-svc, order-svc, notify-svc 依赖 payment:8080）

  预计影响时长: 30-60 秒（滚动重启窗口期，流量由其余副本承担）
  当前副本数: 4 → 重启期间最少 3 副本运行
  Pre-flight ✓ : 集群连接正常 | RBAC 权限充足 | 当前无正在进行的部署

  执行命令: kubectl rollout restart deployment/payment -n production

  输入 "yes" 确认执行，其他任意键取消 >
```

---

# 第七部分：回滚策略矩阵 v1.1

v1.0 的致命问题：不同 K8s 资源类型的回滚方式完全不同，"apply snapshot.yaml"不是万能的。

## 7.1 按资源类型的回滚策略

| 资源类型 | 快照方式 | 回滚方式 | 注意事项 |
|---------|---------|---------|---------|
| **Deployment** | `kubectl get deploy -o yaml` | `kubectl apply -f snapshot.yaml` | ✅ 标准回滚可用。额外可选 `kubectl rollout undo` |
| **StatefulSet** | `kubectl get sts -o yaml` | `kubectl apply -f snapshot.yaml` | ⚠️ apply 不会自动重启 Pod。需要额外 `kubectl rollout restart sts`（但会触发有序重启） |
| **DaemonSet** | `kubectl get ds -o yaml` | `kubectl apply -f snapshot.yaml` | ⚠️ 同 StatefulSet，需要触发重启 |
| **ConfigMap/Secret** | `kubectl get cm/sec -o yaml` | `kubectl apply -f snapshot.yaml` | ⚠️ 热更新不保证 Pod 立即感知。需提示用户可能需要重启关联 Pod |
| **Service** | `kubectl get svc -o yaml` | `kubectl apply -f snapshot.yaml` | ✅ 即时生效 |
| **HPA** | `kubectl get hpa -o yaml` | `kubectl apply -f snapshot.yaml` | ✅ 即时生效 |
| **Helm Release** | `helm get values <release> -o yaml` + `helm get manifest <release>` | `helm rollback <release>` | 🔴 绝不能直接用 kubectl apply！会破坏 Helm 状态管理 |
| **CRD Instance** | `kubectl get <crd> -o yaml` | `kubectl apply -f snapshot.yaml` | ✅ 可用。但需确保 CRD 未被删除 |
| **PVC** | `kubectl get pvc -o yaml` | N/A | 🔴 PVC 快照只保存规格定义，**数据不会自动恢复**。删除 PVC 的回滚需要外部备份 |
| **Namespace 级删除** | N/A | N/A | 🔴 L4 禁区，不可执行 |

## 7.2 回滚执行流程

```
rollback(snapshotID):
    snapshot = loadSnapshot(snapshotID)
    
    if snapshot.resourceType == "Helm":
        execute("helm rollback {release} {revision}")
        execute("helm status {release}")  # 验证
    
    elif snapshot.resourceType == "PVC":
        warn("PVC 快照仅恢复规格定义，数据需要从备份恢复")
        abort()  # 不自动执行
    
    elif snapshot.resourceType in ["StatefulSet", "DaemonSet"]:
        execute("kubectl apply -f {snapshot.file}")
        execute("kubectl rollout restart {resourceType}/{name}")  # 触发 Pod 重建
    
    elif snapshot.resourceType in ["ConfigMap", "Secret"]:
        execute("kubectl apply -f {snapshot.file}")
        warn("配置已回滚。关联 Pod 可能不会自动感知变更，建议手动重启。")
        prompt("是否一起重启关联的 Deployment？[y/n]")
    
    else:  # Deployment, Service, HPA, CRD 实例等
        execute("kubectl apply -f {snapshot.file}")
    
    # 通用验证
    execute("kubectl get {resourceType}/{name} -o yaml | diff - {snapshot.file}")
```

---

# 第八部分：Agent Loop 确定性执行模型 v1.1

这是 v1.0 完全缺失的关键规格。

## 8.1 核心循环

```
Agent Loop (每个用户请求一个 Loop):

1. 接收用户输入
2. 上下文注入: CMDB 拓扑 + 可观测性快照 + GitOps 状态 + Runbook RAG → 融入 system prompt
3. LLM 推理 → 返回 text 或 function_call
4. 如果返回 text → 流式输出给用户，等待下一轮输入
5. 如果返回 function_call:
   a. 安全网关检查 → 判定风险级别
   b. L0: 直接执行 → 结果注入对话 → 返回步骤 3
   c. L1-L2: 展示命令 + 等待用户确认 → 执行 → 结果注入 → 步骤 3
   d. L3: 展示影响面 + 快照 + 等待强确认 → 执行 → 结果注入 → 步骤 3
   e. L4: 拒绝 → 告知原因 → 等待用户修改意图 → 步骤 1
6. 终止条件（满足任一即结束本轮）:
   a. LLM 返回 text 且没有额外 function_call
   b. 达到最大轮次上限 (20 次 tool call)
   c. 用户手动取消 (Ctrl+C)
   d. 单次 tool call 超时 (60s) 累计 3 次
```

## 8.2 关键参数

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `max_tool_calls_per_turn` | 20 | 单个用户请求最多调用 20 次工具 |
| `tool_exec_timeout` | 60s | 单个工具执行超时（kubectl apply 等会适当放宽） |
| `max_consecutive_timeouts` | 3 | 连续超时 3 次后终止本轮，提示用户检查集群状态 |
| `max_total_turn_time` | 300s | 单轮总时长上限 |
| `max_parallel_tools` | 2 | LLM 可以并行调用最多 2 个 L0 工具（只读操作可并行） |

## 8.3 并行工具调用规则

```
LLM 可请求并行调用多个工具，但受以下约束:

1. 仅 L0 工具可并行（所有只读操作）
2. L1+ 工具必须串行，且每个都需要安全网关介入
3. 最多同时 2 个并行调用
4. 如果有工具失败（L0 查询集群超时），不影响其他并行工具的继续执行
5. LLM 收到所有并行工具结果后再进入下一轮推理
```

## 8.4 错误处理分级

| 错误类型 | 行为 |
|---------|------|
| **LLM 返回的 function_call 参数格式错误** | 返回错误信息给 LLM，让它修正，不计入 tool_call 次数 |
| **K8s API 连接超时** | 重试 1 次，仍失败则输出错误信息给 LLM |
| **RBAC 权限不足** (403 Forbidden) | 不重试，直接告诉用户缺少哪个 RBAC 权限 |
| **资源不存在** (404) | 输出给 LLM，让它决定下一步（可能是用户打错名字） |
| **LLM 试图调用不存在的工具** | 返回可用工具列表给 LLM |
| **LLM 选错工具**（比如该用 scale 但调了 rollout restart） | 不影响面分析通过则可以执行，否则安全网关拦截 |

## 8.5 上下文窗口管理（v1.2 新增）

这是上线第一天就会遇到的真实问题。一次排障对话可能产生 50K+ tokens 的工具输出，直接爆掉 LLM 上下文窗口。

### 8.5.1 问题规模

```
典型排障会话的 token 消耗:
  kubectl describe pod (全量)    → 3,000-5,000 tokens
  kubectl logs --tail=200        → 2,500 tokens
  kubectl get events             → 1,500 tokens
  kubectl get pods -o wide       → 800 tokens
  kubectl get hpa + top pods     → 1,000 tokens
  单轮 10 个 tool call 累计      → 20,000-35,000 tokens
  Claude Sonnet 上下文窗口       → 200K tokens（看起来够）
  但 5-6 轮连续对话              → 轻松超过 150K，响应变慢、推理质量下降
```

### 8.5.2 四级缓解策略

| 级别 | 策略 | 触发条件 | 实现 |
|------|------|---------|------|
| **输出截断** | 限制单次工具输出大小 | 始终生效 | describe 只取 conditions + 最近 10 events；logs 默认 `--tail=100`；kubectl get 不给 `-o yaml` 全量 |
| **自动摘要** | LLM 自总结上一轮结果 | 累计工具输出 > 15K tokens | System prompt 注入："Summarize the previous tool results in 3 sentences before proceeding" |
| **历史压缩** | 用摘要替代原始工具输出 | 总对话 > 80% 模型上下文窗口 | 保留最近 3 轮完整对话，更早的用 200 字摘要替代 |
| **硬上限** | 强制结束当前会话 | 总对话 > 95% 上下文窗口 | 提示用户 "上下文已达上限，请开始新会话。已自动保存当前会话摘要。" |

### 8.5.3 工具输出截断规则

```go
// 在 ToolResult 中新增
type ToolResult struct {
    // ... 现有字段 ...
    TokenCount    int    // 本工具输出消耗的 token 数
    IsTruncated   bool   // 输出是否被截断
    TruncatedAt   int    // 截断位置（字符数）
}

// 截断规则（在工具执行层实现）
var truncationRules = map[string]int{
    "kubectl_describe": 2000,  // 最多 2000 字符的 describe 输出
    "kubectl_logs":     1500,  // 最多 1500 字符的日志
    "kubectl_get_events": 1000, // 最多 1000 字符的事件
    "kubectl_get_yaml": 3000,  // YAML 输出最多 3000 字符
    "default":          4000,  // 其他工具默认 4000 字符
}
```

### 8.5.4 上下文预算展示

TUI 状态栏实时显示（Agent Loop 每轮更新）:

```
┌─ ops-ai ─── context: prod-cluster ─── ns: payment ─── 14:32:05 ─┐
│ 本轮 tool calls: 5/20  │  tokens: 18.2K/200K (9.1%)  │  💰 $0.04 │
└─────────────────────────────────────────────────────────────────┘
```

当 tokens > 70% 时数字变黄，> 90% 时变红。

### 8.5.5 智能截断策略（v1.8 新增）

对于常用工具输出，按运维诊断价值保留关键信息：

```
kubectl describe pod:
  ✅ 保留: Conditions, Events (最近 10 条), Container States
  ❌ 截断: Annotations, full Labels list, full Mounts list

kubectl get events:
  ✅ 保留: 最近 20 条非 Normal 类型事件
  ❌ 截断: Normal 类型且超过 1 小时的事件
```

---

## 8.6 Agent Loop 全局超时与死循环防护（v1.8 新增）

### 问题场景

v1.7 的 Agent Loop 只限制了步数（`maxToolCalls: 20`）和单次工具超时（`toolExecTimeout: 60s`），但**没有全局时间 Deadline**。以下场景会导致 Agent 无限膨胀：

1. **LLM API 超时**：网络问题导致 API 调用 30s+ 无响应，但单步超时只覆盖工具调用
2. **推理死循环**：LLM 连续 3 次调用相同工具+相同参数，每次都返回相同错误
3. **流式日志失控**：`kubectl logs -f` 在 30s 超时后继续下一轮，累计时间远超预期
4. **运维切走忘记**：发起排查后切到其他终端，Agent 无限执行

### Go 接口定义

```go
// AgentConfig 扩展（v1.8）
type AgentConfig struct {
    MaxToolCallsPerTurn     int           // 单轮最大工具调用次数 (default: 20)
    ToolExecTimeout         time.Duration // 单个工具执行超时 (default: 60s)
    MaxConsecutiveTimeouts  int           // 连续超时上限 (default: 3)
    MaxTotalTurnTime        time.Duration // 单轮总时长上限 (default: 300s)
    MaxParallelTools        int           // 最大并行工具数 (default: 2)
    
    // v1.8 新增
    GlobalTaskTimeout       time.Duration // 单次任务全局超时 (default: 5m)
    DeadLoopDetectionWindow int           // 死循环检测窗口步数 (default: 3)
    DeadLoopMatchThreshold  int           // 重复模式匹配阈值 (default: 2)
}

// DeadLoopDetector 死循环检测器
type DeadLoopDetector struct {
    recentCalls   []ToolCallPattern       // 最近 N 次工具调用模式
    windowSize    int                     // 检测窗口大小
    matchThreshold int                    // 匹配阈值
}

type ToolCallPattern struct {
    ToolName string
    ArgsHash string                      // 参数的 hash（只比较语义等价，不要求完全相同）
    Error    string                      // 返回的错误
}

// Check 检查最近 windowSize 次调用中是否有 >= matchThreshold 次相同模式
func (d *DeadLoopDetector) Check(call ToolCallPattern) *DeadLoopResult {
    d.recentCalls = append(d.recentCalls, call)
    if len(d.recentCalls) > d.windowSize {
        d.recentCalls = d.recentCalls[1:] // 滑动窗口
    }
    
    if len(d.recentCalls) < d.matchThreshold {
        return &DeadLoopResult{Detected: false}
    }
    
    // 统计最近 windowSize 次中与当前调用相同的次数
    matchCount := 0
    for _, c := range d.recentCalls {
        if c.ToolName == call.ToolName && c.ArgsHash == call.ArgsHash {
            matchCount++
        }
    }
    
    if matchCount >= d.matchThreshold {
        return &DeadLoopResult{
            Detected:    true,
            Pattern:     fmt.Sprintf("重复调用 %s (%s) %d 次", call.ToolName, call.ArgsHash, matchCount),
            Suggestion:  "Agent 似乎陷入循环。建议检查：1) 资源是否确实存在 2) 问题是否需要不同角度排查",
        }
    }
    
    return &DeadLoopResult{Detected: false}
}

type DeadLoopResult struct {
    Detected   bool
    Pattern    string
    Suggestion string
}
```

### Agent Loop 中的集成

```go
func (a *Agent) Run(ctx context.Context, userInput string) error {
    // 1. 全局超时控制
    ctx, cancel := context.WithTimeout(ctx, a.config.GlobalTaskTimeout)
    defer cancel()
    
    // 2. 死循环检测器
    deadLoopDetector := &DeadLoopDetector{
        windowSize:    a.config.DeadLoopDetectionWindow,
        matchThreshold: a.config.DeadLoopMatchThreshold,
    }
    
    // 3. TUI 实时倒计时
    if a.tui != nil {
        go a.tui.ShowCountdown(ctx) // 状态栏显示 "⏱ 4:32 剩余"
    }
    
    for step := 0; step < a.config.MaxToolCallsPerTurn; step++ {
        select {
        case <-ctx.Done():
            // 超时终止
            a.presentTimeoutSummary()
            return ctx.Err()
        default:
        }
        
        // ... LLM 推理 + 工具调用 ...
        
        // 死循环检测
        if result := deadLoopDetector.Check(call.Pattern()); result.Detected {
            a.tui.ShowWarning(fmt.Sprintf(
                "⚠️  检测到重复操作模式: %s\n%s\n\n[Y] 继续  [N] 终止排查",
                result.Pattern, result.Suggestion,
            ))
            // 等待用户决定
        }
    }
}

// presentTimeoutSummary 超时时展示已完成步骤的摘要
func (a *Agent) presentTimeoutSummary() {
    fmt.Printf(`
  ═══════════════════════════════════════════════════════
  ⏱  任务超时 (5分钟)
  ═══════════════════════════════════════════════════════

  已完成 %d 步排查:

  %s

  当前状态: 排查未完成，以下步骤尚未执行:
  %s

  建议: 使用 /continue 命令继续排查，或 /export 保存当前上下文。
  `, len(a.toolResults), a.summarizeDone(), a.summarizePending())
}
```

### TUI 超时展示

```
  ═══════════════════════════════════════════════════════
  ⏱  任务超时 — 排查未完成
  ═══════════════════════════════════════════════════════

  已完成 8 步:

  1. kubectl get pods -n payment                        ✅ 发现 2 个 CrashLoopBackOff
  2. kubectl describe pod payment-7d9f8b-abc            ✅ OOMKilled
  3. kubectl top pods -n payment                        ✅ 内存 1.8Gi/2Gi
  4. kubectl logs payment-7d9f8b-abc --tail=50          ✅ NullPointerException
  5. kubectl get deployment payment -o yaml             ✅ memory limit: 2Gi
  6. kubectl get events -n payment                      ✅ 
  7. kubectl describe hpa payment                       ⏱️ 超时未执行
  8. 分析根因并给出建议                                    ⏱️ 超时未执行

  💡 /continue — 继续排查
  💡 /export — 保存当前上下文，稍后继续
```

### 配置项

```yaml
# config.yaml
agent:
  timeout: 5m               # 单次任务全局超时
  dead_loop:
    detection_window: 3     # 检测最近 3 步
    match_threshold: 2      # 出现 2 次相同模式即告警
```

---


# 第九部分：K8s 工具精确列表 v1.1

## 9.1 MVP 完整命令清单

### L0 — 只读操作（自动执行，无需确认）

```
kubectl get <resource> [-o wide|json|yaml] [-n namespace] [-l selector]
  ├─ pods, deployments, statefulsets, daemonsets
  ├─ services, endpoints, ingresses
  ├─ configmaps, secrets (values redacted by default)
  ├─ hpa, pdb, serviceaccounts
  ├─ nodes, namespaces
  ├─ events --sort-by='.lastTimestamp'
  └─ crd instances

kubectl describe <resource> <name> [-n namespace]
  └─ 同上述资源类型

kubectl logs <pod> [-n namespace] [-c container] [--tail=N] [--since=5m] [--previous]
kubectl top pods|nodes [-n namespace]
kubectl api-resources
kubectl explain <resource>
kubectl rollout status deployment/<name> [-n namespace]
kubectl rollout history deployment/<name> [-n namespace]

helm list [-n namespace]
helm get values <release> [-n namespace]
helm get manifest <release> [-n namespace]
helm history <release> [-n namespace]
helm status <release> [-n namespace]
```

### L1 — 低风险修改（回车确认）

```
kubectl scale deployment/<name> --replicas=N [-n namespace]
  └─ 约束: 不能缩到 0（需 L2），不能扩超过 HPA max（需 L2）

kubectl label <resource> <name> <key>=<value> [-n namespace]
kubectl annotate <resource> <name> <key>=<value> [-n namespace]

kubectl cordon <node>       # 标记节点不可调度（不驱逐现有 Pod）
kubectl uncordon <node>     # 恢复节点调度
```

### L2 — 中等风险修改（输入 yes 确认 + 影响面分析）

```
kubectl rollout restart deployment/<name> [-n namespace]
kubectl rollout undo deployment/<name> [-n namespace] [--to-revision=N]

kubectl set image deployment/<name> <container>=<image:tag> [-n namespace]
kubectl set resources deployment/<name> [-c container] --limits=cpu=X,memory=Y

kubectl apply -f <file>       # 仅 L2 当文件不包含 delete/namespace 操作
kubectl patch <resource> <name> --patch '...' [-n namespace]
kubectl edit <resource> <name> [-n namespace]    # 需实现安全预览
kubectl scale deployment/<name> --replicas=0      # 缩容到 0
kubectl scale deployment/<name> --replicas=N      # 扩容超 HPA max

kubectl drain <node> --ignore-daemonsets --delete-emptydir-data

helm upgrade <release> <chart> [--values <file>] [-n namespace]
```

### L3 — 高风险修改（输入集群名确认 + 强制快照 + 可选双人审批）

```
kubectl delete <resource> <name> [-n namespace]    # 单资源删除
kubectl delete -f <file>                           # 从文件批量删除
kubectl taint nodes <node> <key>=<value>:<effect>

helm uninstall <release> [-n namespace]

# 所有 L3 操作强制执行:
# 1. 影响面分析
# 2. 自动快照
# 3. 用户输入当前集群名确认
# 4. 如果 config 中启用了双人审批 → 生成审批 token → 等待第二个操作员确认
```

### L4 — 禁区（直接拒绝）

```
kubectl delete namespace <name>      # 命名空间级删除
kubectl delete crd <name>            # CRD 级联删除
kubectl delete --all <resource>      # 全量删除
任何涉及 /, /etc, /var, /root 路径的写操作
```

### 9.1.1 kubectl exec 智能白名单策略（v1.2 重大修正）

v1.1 将 `kubectl exec` 一刀切归入 L4，但实际排障中大量场景依赖 exec：

```
kubectl exec pod -- curl localhost:8080/health   ← 最常见的健康检查
kubectl exec pod -- cat /etc/hosts               ← 排查 DNS
kubectl exec pod -- netstat -tlnp                ← 排查端口监听
kubectl exec pod -- df -h                        ← 排查磁盘
kubectl exec pod -- ls /data/                    ← 排查挂载卷
```

全部禁止会让排障能力大打折扣。采用**命令白名单 + 模式匹配**策略：

```
exec 安全策略:

1. 默认风险级别: L2（在容器内执行命令有安全风险）

2. 自动降为 L1 的命令（纯读取/诊断类）:
   ├─ 文件读取: cat, head, tail, less, more
   ├─ 目录浏览: ls, find, tree, stat, du, df
   ├─ 网络诊断: curl -I/-X GET, wget --spider, netstat, ss, ping, nslookup, dig, traceroute
   ├─ 进程诊断: ps, top, free, uptime, pgrep
   └─ 环境检查: env, printenv, echo $VAR, whoami, id
   
   判定方式: MVP 阶段用正则白名单匹配命令首词

3. 维持 L2 的命令（需明确确认）:
   ├─ mkdir, touch, cp, mv（无 -f 标志时）
   ├─ chmod, chown（无 777/递归时）
   └─ curl -X POST/PUT/DELETE（可能触发副作用）

4. 严格禁止 L4（无论什么参数）:
   ├─ rm, rmdir, dd, mkfs, fdisk, shred
   ├─ shutdown, reboot, halt, init, systemctl stop/disable
   ├─ iptables, ip6tables, ufw, nft
   ├─ apt, yum, apk, pip, npm, gem（安装包）
   ├─ wget/curl 结合输出重定向（> / >> / | tee）
   ├─ git clone/pull/push（可能泄露代码）
   ├─ 任何包含 >, >>, 2>, &> 等写重定向的管道
   └─ 任何涉及 /proc/sys/, /sys/ 路径的写操作

5. 命令解析方案:
   MVP:    正则匹配首词 + 禁止写重定向符号
   Phase 2: mvdan.cc/sh AST 解析（与 kubectl-ai 同方案）
```

正则白名单示例（Go 实现）：

```go
var diagnosticCommands = regexp.MustCompile(
    `^(cat|head|tail|less|more|ls|find|stat|du|df|` +
    `netstat|ss|ping|nslookup|dig|traceroute|` +
    `ps|top|free|uptime|pgrep|env|printenv|echo|whoami|id|` +
    `curl -I|curl --head|wget --spider|` +
    `curl -X GET)$`,
)

var forbiddenPatterns = []string{
    `>`, `>>`, `2>`, `&>`, `| tee `,  // 写重定向
    `rm `, `dd `, `mkfs`, `fdisk`,    // 破坏性命令
    `/proc/sys/`, `/sys/`,            // 内核参数
}
```

## 9.2 K8s Client 集成决策：client-go

**决策：使用 client-go，不 exec kubectl 二进制。**

理由：

| 对比维度 | client-go | exec kubectl | 结论 |
|---------|-----------|-------------|------|
| 安全性 | ✅ 无 shell 注入风险 | ❌ 需沙箱 + AST 解析 | client-go 胜 |
| 结构化结果 | ✅ 类型化 Go struct | ❌ 需解析 YAML/JSON 输出 | client-go 胜 |
| 影响面分析 | ✅ 可编程遍历资源关联 | ❌ 需多次 kubectl get | client-go 胜 |
| Watch/Informer | ✅ 原生支持（验证变更生效） | ❌ 不支持 | client-go 胜 |
| 实现复杂度 | ⚠️ 较高（需熟悉 K8s API） | ✅ 简单（字符串拼接） | 初始成本高，长期收益大 |
| RBAC 即服务 | ✅ 直接感知权限错误 | ✅ 同样感知 | 持平 |

### 配置初始化

```go
type K8sClientProvider struct {
    kubeconfigPath string
    contextName    string
    clients        map[string]kubernetes.Interface // context名 → client
}

// 初始化优先级:
// 1. --kubeconfig flag 指定的文件
// 2. KUBECONFIG 环境变量
// 3. ~/.kube/config（默认）
// 4. In-cluster config（Pod 内运行时）

// 多集群切换: 通过 --context 或 ops-ai 内置的 context 切换命令
```

## 9.3 Helm 集成决策（v1.2 新增）

v1.1 列出了 Helm 命令列表但没有决定集成方式。Helm Go SDK 以 API 不稳定和文档缺失著称，直接全量使用风险很高。采用**分阶段混合策略**：

| Helm 命令 | MVP 方案 | 理由 |
|-----------|---------|------|
| `helm list` | Helm Go SDK (`action.NewList()`) | 简单，只需 5 行代码 |
| `helm get values` | Helm Go SDK (`action.NewGetValues()`) | 同上 |
| `helm get manifest` | Helm Go SDK (`action.NewGetManifest()`) | 同上 |
| `helm history` | Helm Go SDK (`action.NewHistory()`) | 同上 |
| `helm status` | Helm Go SDK (`action.NewStatus()`) | 同上 |
| `helm upgrade` | **exec helm 二进制** + 参数白名单 | SDK 的 `action.NewUpgrade()` 需要处理 chart 加载、依赖解析、值合并，实现复杂度极高 |
| `helm rollback` | **exec helm 二进制** + 参数白名单 | SDK 相对简单但为了一致性也走 exec |
| `helm uninstall` | **exec helm 二进制** + 参数白名单 | L3 操作，走 exec + 安全网关拦截 |

```go
// 伪代码：Helm 工具路由
func (h *HelmTool) Execute(input ToolExecInput, ctx ToolContext) (ToolResult, error) {
    switch input.Operation {
    case "list", "get_values", "get_manifest", "history", "status":
        // L0 只读 → 使用 Helm SDK
        return h.executeViaSDK(input, ctx)
    case "upgrade", "rollback", "uninstall":
        // L2-L3 写操作 → exec helm 二进制
        // 在 exec 前做参数白名单校验
        if !h.validateArgs(input.Parameters) {
            return errorResult("helm args rejected by safety whitelist")
        }
        return h.executeViaBinary(input, ctx)
    }
}
```

exec helm 的参数白名单（防注入）：

```go
var allowedHelmFlags = map[string]bool{
    "--namespace":    true,
    "-n":             true,
    "--values":       true,  // 允许 -f values.yaml
    "-f":             true,
    "--set":          true,  // 允许 --set key=value（但限制不能包含 ; | & $）
    "--version":      true,
    "--timeout":      true,
    "--atomic":       true,
    "--wait":         true,
    "--dry-run":      true,
    "--reuse-values": true,
    "--reset-values": true,
    "--description":  true,
}

// 安全约束:
// - --set 的值不能包含 shell 元字符（; | & $ `）
// - release name 和 chart name 只允许 [a-zA-Z0-9._-]
// - namespace 只允许 [a-zA-Z0-9-]
```

### 9.1.2 kubectl exec -it 交互式终端引导（v1.3 新增）

Agent 不支持交互式终端（`kubectl exec -it`），但排障有时必须进入容器手动操作。策略：Agent 不自动执行 `-it` 命令，而是生成安全的命令供用户手动执行。

```
当 LLM 判断必须进入容器时:
  Agent 输出:
  "此操作需要交互式终端，我无法自动执行。
   请在另一个终端手动运行:
   
     kubectl exec -it <pod-name> -n <namespace> -- /bin/sh
   
   操作完成后，告诉我结果，我会继续帮你分析。
   
   安全提醒: 进入容器后请避免执行 rm、dd、iptables 等破坏性命令。"
  
  绝不生成: 自动执行的 exec -it 命令（Agent 会在 L4 拒绝）
```

### 9.1.3 CRD 多版本 API 选择提示（v1.3 新增）

CRD 通常有多个 API 版本共存（v1alpha1、v1beta1、v1）。Agent 操作 CR 时可能选错版本导致失败。

```
Agent 行为:
  1. 优先使用 kubectl api-resources | grep <crd-group> 查看可用版本
  2. 优先选择 GA 版本（v1）→ Beta（v1beta1）→ Alpha（v1alpha1）
  3. 如果操作失败且错误提示 API 版本不匹配 → 自动检查其他版本后重试
  4. 首次操作新 CRD 类型时，在 L0 阶段自动执行 api-resources 确认版本

  示例:
  kubectl api-resources | grep cert-manager.io
  → certificates  cert-manager.io/v1  true  Certificate
  → Agent 应使用 cert-manager.io/v1，而非 cert-manager.io/v1alpha2
```

### 9.1.4 kubectl port-forward 支持（v1.4 新增）

这是运维最常用的调试手段之一。`port-forward` 不需要 `-it` 交互终端，Agent 可以自动执行并验证端口连通性。

```
kubectl port-forward pod/payment-debug-7d8f9 8080:8080          # L1: 端口转发
kubectl port-forward deployment/payment 8080:8080                # L1: 转发到 Deployment
kubectl port-forward service/payment 8080:80                     # L1: 转发到 Service
```

**安全分级: L1**（需回车确认，有网络暴露风险但不修改集群状态）

Agent 行为:
```
1. 执行 kubectl port-forward <target> <local-port>:<remote-port>
2. port-forward 在后台运行（Agent 管理其生命周期）
3. Agent 自动 curl 测试端口: curl localhost:<local-port>/health
4. 验证通过后输出结果
5. 默认 30 秒后自动关闭（可配置: port_forward_ttl）
6. 用户可通过 Ctrl+C 提前关闭
```

```go
type PortForwardTool struct {
    activeForwards map[string]*exec.Cmd
    ttl           time.Duration  // 默认 30s
}

func (pf *PortForwardTool) Execute(input ToolExecInput, ctx ToolContext) (ToolResult, error) {
    localPort := input.Parameters["local_port"].(string)
    remotePort := input.Parameters["remote_port"].(string)
    target := input.Parameters["target"].(string)
    
    cmd := exec.Command("kubectl", "port-forward", target,
        fmt.Sprintf("%s:%s", localPort, remotePort),
        "-n", ctx.Namespace)
    
    pf.activeForwards[localPort] = cmd
    
    // 等待端口就绪 → curl 测试
    time.Sleep(500 * time.Millisecond)
    testResult := curlTest(localPort)
    
    // 自动关闭定时器
    time.AfterFunc(pf.ttl, func() { pf.cleanup(localPort) })
    
    return ToolResult{
        Success: true,
        Output: fmt.Sprintf(
            "端口转发: localhost:%s → %s:%s (30s 后自动关闭)\n测试: %s",
            localPort, target, remotePort, testResult,
        ),
    }, nil
}
```

TUI 状态栏变化:
```
┌─ ops-ai ─── prod-cluster ─── ns: payment ─── 🔗 :8080→payment:8080 (22s) ─┐
```

### 9.1.5 kubectl cp 文件传输（v1.4 新增）

按方向分级，从容器拉出是安全的诊断手段，向容器写入有变更风险。

```
kubectl cp payment-pod:/var/log/app/debug.log ./debug.log        # L0: 从容器拉文件
kubectl cp payment-pod:/etc/app/config.yaml ./config-backup.yaml  # L0: 从容器拉配置
kubectl cp ./config.yaml payment-pod:/etc/app/config.yaml         # L2: 向容器写文件
```

**安全分级——按方向**:
```
从容器拉出 (pod: → local): L0（只读，不修改容器状态）
  约束: 目标路径必须在当前工作目录或 /tmp 下

向容器写入 (local → pod:):  L2（修改容器文件系统，需 yes 确认）
  约束:
  - 目标路径不能是 /, /etc/passwd, /etc/shadow, /proc/, /sys/
  - 源文件大小限制: 默认 1MB
  - 写入前自动备份容器内原文件: pod:path → pod:path.bak
```

### 9.1.6 kubectl edit 在 TUI 中的交互模型（v1.4 新增）

`kubectl edit` 是 L2 命令，但 Bubble Tea TUI 不是 vim。必须明确定义交互方式。

```
MVP 方案 — 外部编辑器:
  → Agent: kubectl get <resource> -o yaml > /tmp/ops-ai-edit-<uuid>.yaml
  → 打开 $OPS_AI_EDITOR 或 $EDITOR（默认 vim）
  → 用户编辑 → 保存退出
  → Agent 检测变更 (SHA256 diff) → 安全网关检查 → 用户确认 → kubectl apply
  → 清理临时文件
```

```go
func (e *EditTool) Execute(input ToolExecInput, ctx ToolContext) (ToolResult, error) {
    resource := input.Parameters["resource"].(string)
    
    // 智能路由: 简单变更用 patch 替代
    if isSimpleChange(input) {
        return e.executeViaPatch(input, ctx) // 更快、不需要外部编辑器
    }
    
    // 复杂变更: 打开外部编辑器
    tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("ops-ai-edit-%s.yaml", uuid.New()))
    yamlContent, _ := executeKubectl("get", resource, "-n", ns, "-o", "yaml")
    os.WriteFile(tmpFile, yamlContent, 0644)
    
    editor := os.Getenv("OPS_AI_EDITOR")
    if editor == "" { editor = os.Getenv("EDITOR") }
    if editor == "" { editor = "vim" }
    
    cmd := exec.Command(editor, tmpFile)
    cmd.Stdin = os.Stdin; cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr
    cmd.Run()
    
    newContent, _ := os.ReadFile(tmpFile)
    diff := computeDiff(string(yamlContent), string(newContent))
    
    return ToolResult{
        Success: true,
        Output:  fmt.Sprintf("变更预览:\n%s\n\n输入 yes 确认执行", diff),
    }
}
```

---

## 9.4 节点智能诊断（v1.3 新增）

节点级故障是运维高频场景，v1.2 只覆盖了 cordon/drain/uncordon 操作本身，缺乏故障诊断能力。

### 诊断触发流程

```
用户: "node-7 NotReady 了，帮我看看"

Agent 自动执行（L0，并行）:
1. kubectl get node node-7                      → 状态 + conditions
2. kubectl describe node node-7                 → 最近 events + 资源情况
3. kubectl get pods -A --field-selector spec.nodeName=node-7  → 该节点上的 Pod

分析逻辑:
├─ 如果 conditions 包含 MemoryPressure/DiskPressure/PIDPressure:
│  → 报告具体压力类型 + 资源使用
│  → 建议: 扩容节点、驱逐部分 Pod、清理磁盘
│
├─ 如果 conditions 包含 NetworkUnavailable:
│  → 建议检查 CNI 插件状态
│  → kubectl get pods -n kube-system | grep -E "calico|cilium|flannel|weave"
│
├─ 如果节点是云提供商的（从 node labels 判断，如 eks.amazonaws.com/compute-type）:
│  → 提示用户检查云控制台看底层 VM 状态
│  → AWS: EC2 实例状态检查 / GCP: 虚拟机健康状态 / Azure: VM 状态
│
├─ 如果是裸金属:
│  → 建议 SSH 到节点检查 kubelet 日志
│  → journalctl -u kubelet --since "10 minutes ago"
│  → 或提示用户在 Agent 不支持 SSH 时手动检查
│
└─ Pod 影响分析:
   → 列出该节点上的 Pod
   → 标注哪些被驱逐（DeletionTimestamp 非空）
   → 标注哪些卡在 Terminating
   → 给出恢复建议:
     - 如果节点短时间可恢复 → 等待
     - 如果节点需要长时间修复 → cordon + drain → Pod 重新调度
```

### L0 命令扩展（节点诊断）

```
kubectl get nodes -o wide                            # 所有节点状态
kubectl describe node <name>                          # 节点详情
kubectl get pods -A -o wide --field-selector spec.nodeName=<name>  # 某节点上的 Pod
kubectl get events -A --field-selector involvedObject.kind=Node,involvedObject.name=<name>  # 节点相关事件
kubectl top node <name>                               # 节点资源使用
kubectl get pods -A --field-selector spec.nodeName=<name> -o json | jq '.items[] | select(.metadata.deletionTimestamp != null) | .metadata.name'  # 被驱逐的 Pod
```

### System Prompt 补充（节点诊断知识模板）

Agent 的 system prompt 需注入以下节点故障排查知识：

```
NODE TROUBLESHOOTING KNOWLEDGE:

Common node conditions and their meaning:
- Ready=False: kubelet 停止上报，常见原因: kubelet 宕机、Docker/containerd 挂起、节点 OOM
- MemoryPressure=True: 节点内存不足
- DiskPressure=True: 节点磁盘空间不足（imagefs 或 nodefs）
- PIDPressure=True: 节点 PID 耗尽
- NetworkUnavailable=True: CNI 未正确初始化

Troubleshooting order:
1. Check node conditions → kubectl describe node <name>
2. Check node resource usage → kubectl top node <name>
3. Check kubelet status (if SSH available) → journalctl -u kubelet
4. Check CNI pods → kubectl get pods -n kube-system | grep <cni-provider>
5. Check container runtime → journalctl -u containerd/docker
6. For cloud nodes, suggest checking cloud provider VM health console

Do NOT suggest:
- Rebooting the node without draining first
- SSH commands unless the environment supports it
- Deleting the node object unless explicitly asked
```

## 9.5 Config Drift 检测 — GitOps 价值落地（v1.3 新增）

v1.2 提到 ArgoCD/Flux 是 L0，但只说是"同步状态检查"。Config Drift 检测是这个场景的核心价值。

### 场景

```
用户: "为什么昨晚 payment 部署出问题了？"

Agent（L0 自动执行）:
1. kubectl get deploy payment -o yaml → 当前集群状态（replicas=1）
2. Git 源期望状态（通过 ArgoCD API 或 git 仓库）→ replicas=3

→ 检测到配置漂移！
→ "检测到配置漂移: Deployment/payment 的 replicas 与 Git 源不一致。
   集群当前: replicas=1 | Git 期望: replicas=3
   最后一次同步: 昨天 22:15 (失败)
   
   这可能解释了昨晚的性能问题。
   建议: 触发 ArgoCD sync 或检查为何同步失败。"
```

### ArgoCD 集成（L0）

```
argocd app list                                              # L0: 所有应用同步状态
argocd app get <app>                                         # L0: 应用详情（含 sync status）
argocd app diff <app>                                        # L0: 对比集群状态 vs Git 定义
argocd app history <app>                                     # L0: 部署历史
argocd app sync <app>                                        # L2: 触发同步（需确认）
argocd app rollback <app> <revision>                         # L2: 回滚
argocd app manifests <app>                                   # L0: 查看期望状态
```

### Flux 集成（L0）

```
flux get kustomizations                                      # L0: 所有 Kustomization 状态
flux get helmreleases                                        # L0: 所有 HelmRelease 状态
flux diff kustomization <name>                               # L0: 对比集群 vs Git
flux reconcile source git <name>                             # L2: 触发 Git 源同步
flux reconcile kustomization <name>                          # L2: 触发 Kustomization 同步
```

### Config Drift 报告格式

```
Config Drift 检测 — namespace: production, 资源数: 34

  ✅ 已同步 (29/34):
     deployment/payment (v3.2.1) ✓
     configmap/app-config ✓
     ...共 29 个资源

  ⚠️ 漂移 (3/34):
     deployment/report-worker:
       集群: replicas=5, image=v3.2.0
       Git:   replicas=3, image=v3.2.1
       原因: 手动 kubectl scale 修改了副本数，未通过 GitOps

     configmap/feature-flags:
       集群: enable_new_checkout=false
       Git:   enable_new_checkout=true
       原因: 紧急回滚手动修改，未同步回 Git

     hpa/order-processor:
       集群: maxReplicas=15
       Git:   maxReplicas=10
       原因: 压测时手动调高，未恢复

  ❌ 同步失败 (2/34):
     deployment/notification → ArgoCD sync failed: "ImagePullBackOff: registry timeout"
     secret/external-api-key → "Secret managed externally, cannot sync"

  建议操作:
  1. 对漂移资源执行: argocd app sync <app> 或手动 kubectl apply 对齐
  2. 将手动变更回填到 Git（防止下次 sync 覆盖）
  3. 修复 notification 镜像拉取问题
```

### MVP 实现路径

MVP 阶段不需要深度集成 ArgoCD API。最简单的方案：

```
1. 检测到用户环境安装了 argocd CLI → 调用 argocd app diff
2. 检测到用户环境安装了 flux CLI → 调用 flux diff kustomization
3. 都没有 → 提示 "启用 Config Drift 检测需要安装 ArgoCD CLI 或 Flux CLI"
4. Phase 2: 通过 ArgoCD REST API 直接调用，无需 CLI
```

### GitOps 联动场景更新（§3.3 补充）

```
| **GitOps 联动** | ArgoCD/Flux 同步状态、Config Drift 检测、部署历史追溯 | ✅ L0-L2 |
```

## 9.6 ImagePullBackOff 根因分析（v1.4 新增）

`kubectl set image`（L2）执行后最常见的故障就是 ImagePullBackOff。v1.3 的 System Prompt 没有教 Agent 怎么诊断这个高频场景。

```
场景:
  Agent: kubectl set image deployment/payment payment=registry.io/payment:v2.3.0
  → 30 秒后 kubectl get pods → 新 Pod 状态: ImagePullBackOff
  → v1.3: Agent 笼统报告 "镜像拉取失败，请检查镜像仓库"
  → v1.4: Agent 自动分析具体原因 → 给出针对性建议
```

### 诊断流程

```
Agent 自动执行（L0，并行）:
1. kubectl describe pod <pod-name> → 看 Events 部分的拉取错误
2. kubectl get pod <pod-name> -o json | jq '.status.containerStatuses[].state.waiting'

错误分类与建议:
┌────────────────────────────────────────────────────────────────┐
│ 错误消息                           │ 原因            │ 建议    │
├────────────────────────────────────────────────────────────────┤
│ "image not found" /                │ tag 错误或镜像   │ 检查镜像│
│ "manifest unknown"                 │ 不存在          │ tag 拼写│
├────────────────────────────────────────────────────────────────┤
│ "pull access denied" /             │ 缺少拉取凭证    │ 检查    │
│ "authorization failed"             │                 │ imagePul│
│                                    │                 │ lSecrets│
├────────────────────────────────────────────────────────────────┤
│ "dial tcp: i/o timeout" /          │ registry 网络   │ 检查    │
│ "no such host"                     │ 不可达          │ registry│
│                                    │                 │ 域名可解析│
├────────────────────────────────────────────────────────────────┤
│ "x509: certificate" /              │ registry TLS    │ 检查    │
│ "tls: failed to verify"            │ 配置问题        │ 证书或  │
│                                    │                 │ insecure│
│                                    │                 │ -registr│
│                                    │                 │ y 配置  │
├────────────────────────────────────────────────────────────────┤
│ "toomanyrequests" /                │ Docker Hub      │ 等待或  │
│ "rate limit exceeded"              │ 限流            │ 使用镜像│
│                                    │                 │ 代理    │
├────────────────────────────────────────────────────────────────┤
│ "context deadline exceeded"        │ registry 响应   │ 检查    │
│                                    │ 超时            │ registry│
│                                    │                 │ 延迟    │
└────────────────────────────────────────────────────────────────┘
```

### Agent 自动排查步骤

```
1. kubectl describe pod → 提取 Events 中的 "Failed to pull image" 消息
2. 正则匹配错误类型
3. 如果是 "pull access denied":
   → kubectl get pod -o yaml | grep imagePullSecrets → 检查是否配置
   → kubectl get secret <pull-secret> -n <ns> → 检查 Secret 是否存在
   → 如果是私有 registry → 提示检查 docker-registry Secret 的 .dockerconfigjson 内容
4. 如果是 "dial tcp: i/o timeout":
   → 从错误消息中提取 registry hostname
   → kubectl run test-dns --image=busybox --rm -it -- nslookup <registry-host>
   → 测试 registry 网络可达性
```

### System Prompt 补充

```
IMAGE PULL TROUBLESHOOTING:

When a Pod shows ImagePullBackOff or ErrImagePull:
1. Always kubectl describe pod first — the Events contain the exact error
2. Classify the error (see table above)
3. For private registries, verify:
   - imagePullSecrets exist in the Pod spec
   - The referenced Secret exists in the namespace
   - The Secret contains valid docker-registry credentials
4. NEVER suggest 'imagePullPolicy: Never' as a workaround
5. NEVER suggest --insecure-registry without warning about security implications
```

### L0 命令扩展

```
kubectl get pod <name> -o json | jq '.status.containerStatuses[].state.waiting'  # L0: 查看等待原因
kubectl get pod <name> -o json | jq '.spec.imagePullSecrets'                       # L0: 查看拉取凭证配置
```

## 9.7 K8s API 版本弃用感知（v1.4 新增）

K8s 每个版本弃用一批 API。v1.3 的 §9.1.3 只覆盖了 CRD 场景，但标准 K8s API 同样会弃用。Agent 建议的 YAML 如果用了已弃用的 API 版本，会被 API Server 直接拒绝。

```
场景:
  运维: "帮我创建一个 Ingress"
  Agent 生成 YAML:
    apiVersion: extensions/v1beta1
    kind: Ingress
  → kubectl apply → Error: no matches for kind "Ingress" in version "extensions/v1beta1"
  → Agent 不知道 extensions/v1beta1 在 K8s 1.22+ 已弃用
```

### Agent 启动时自动感知

```go
// 启动时或每次 context 切换时执行
func (a *Agent) discoverAPIVersions() error {
    // 1. 获取集群支持的 API 资源
    resources, _ := a.k8sClient.Discovery().ServerPreferredResources()
    
    // 2. 构建可用版本缓存
    a.apiVersionCache = make(map[string]string) // kind → preferredVersion
    
    for _, group := range resources {
        for _, resource := range group.APIResources {
            key := strings.ToLower(resource.Kind)
            a.apiVersionCache[key] = group.GroupVersion
        }
    }
    
    // 3. 也执行 kubectl api-versions（L0）供 LLM 参考
    return nil
}
```

### Agent 行为

```
当 LLM 需要生成 K8s YAML 时:
  → 优先使用 apiVersionCache 中的 preferred version
  → 如果缓存中没有 → 先执行 kubectl explain <resource> 确认 API 版本
  → 如果 apply 时收到 "no matches for kind" 错误:
    → 自动执行 kubectl api-resources | grep -i <kind>
    → 找到正确的 apiVersion
    → 修正 YAML 后重试

已知弃用映射（硬编码知识）:
  extensions/v1beta1 → networking.k8s.io/v1 (Ingress, K8s 1.22+)
  apps/v1beta1/v1beta2 → apps/v1 (Deployment/StatefulSet, K8s 1.16+)
  batch/v1beta1 → batch/v1 (CronJob, K8s 1.25+)
  policy/v1beta1 → policy/v1 (PodDisruptionBudget, K8s 1.25+)
  autoscaling/v2beta2 → autoscaling/v2 (HPA, K8s 1.26+)
```

### System Prompt 补充

```
K8S API VERSION RULES:

1. Before generating any K8s YAML, check the cluster's available API versions:
   - kubectl api-resources | grep -i <kind>
   - kubectl explain <kind> --api-version=<version>
2. Always prefer the GA (stable) version:
   - v1 over v1beta1 over v1alpha1
3. If you receive "no matches for kind" error, the API version is wrong.
   Do NOT retry with the same version. Look up the correct one.
4. Known deprecated mappings (for reference, but cluster's actual state overrides):
   - Ingress: extensions/v1beta1 → networking.k8s.io/v1
   - CronJob: batch/v1beta1 → batch/v1
   - PDB: policy/v1beta1 → policy/v1
   - HPA: autoscaling/v2beta2 → autoscaling/v2
```

### L0 命令扩展

```
kubectl api-versions                                      # L0: 集群支持的所有 API 版本
kubectl api-resources                                     # L0: 集群支持的所有 API 资源+版本
kubectl explain <resource> --api-version=<version>         # L0: 检查某版本是否存在
```

## 9.8 集群健康概览（v1.5 新增）

这是运维每天打开 ops-ai 的第一件事——快速扫一眼集群整体健康状态。v1.4 的交互模型假设用户知道要问什么，但真实场景下运维打开终端后经常需要一个"起点"。

### 9.8.1 触发方式

```
方式 1 — 启动时自动展示:
  ./ops-ai
  → 自动执行集群健康概览（如 config.yaml 中 startup_overview: true）
  → 展示摘要后进入交互模式

方式 2 — /health 命令:
  用户: /health
  → Agent 执行健康概览

方式 3 — --overview flag:
  ./ops-ai --overview
  → 执行概览后退出（适合脚本/定时任务）

方式 4 — 自然语言:
  用户: "集群状态怎么样？"
  → Agent 识别意图，执行健康概览
```

### 9.8.2 检查项目与输出格式

```go
// ClusterHealthCheck 集群健康概览
type ClusterHealthCheck struct {
    k8sClient kubernetes.Interface
}

type HealthOverview struct {
    Nodes       NodeHealth       // 节点健康
    Pods        PodHealth        // Pod 健康
    PDBs        PDBHealth        // PDB 余量
    Resources   ResourceHealth   // 资源使用
    RecentAlerts []AlertSummary  // 最近告警
    Deployments  DeployHealth    // 部署状态
    Timestamp    time.Time
}

func (h *ClusterHealthCheck) Run(ctx ToolContext) (*HealthOverview, error) {
    overview := &HealthOverview{Timestamp: time.Now()}
    
    // 并行执行所有检查（全部 L0）
    var wg sync.WaitGroup
    wg.Add(5)
    
    go func() { defer wg.Done(); overview.Nodes = h.checkNodes(ctx) }()
    go func() { defer wg.Done(); overview.Pods = h.checkPods(ctx) }()
    go func() { defer wg.Done(); overview.PDBs = h.checkPDBs(ctx) }()
    go func() { defer wg.Done(); overview.Resources = h.checkResources(ctx) }()
    go func() { defer wg.Done(); overview.Deployments = h.checkDeployments(ctx) }()
    
    wg.Wait()
    return overview, nil
}

func (h *ClusterHealthCheck) checkNodes(ctx ToolContext) NodeHealth {
    nodes, _ := ctx.K8sClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
    
    result := NodeHealth{Total: len(nodes.Items)}
    for _, n := range nodes.Items {
        for _, c := range n.Status.Conditions {
            if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
                result.Ready++
            }
        }
    }
    result.NotReady = result.Total - result.Ready
    return result
}

func (h *ClusterHealthCheck) checkPods(ctx ToolContext) PodHealth {
    pods, _ := ctx.K8sClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
        FieldSelector: "status.phase!=Running,status.phase!=Succeeded",
    })
    
    result := PodHealth{TotalAbnormal: len(pods.Items)}
    for _, p := range pods.Items {
        if p.Status.Phase == corev1.PodPending {
            result.Pending++
            result.PendingPods = append(result.PendingPods, formatPodName(p))
        }
        for _, cs := range p.Status.ContainerStatuses {
            if cs.State.Waiting != nil {
                switch cs.State.Waiting.Reason {
                case "CrashLoopBackOff": result.CrashLoop++
                case "ImagePullBackOff", "ErrImagePull": result.ImagePull++
                case "OOMKilled": result.OOMKilled++
                }
            }
        }
    }
    return result
}
```

### 9.8.3 终端输出格式

```
ops-ai --overview

═══ 集群健康概览 — prod-us-east-1 — 2026-06-25 14:32:05 ═══

🟢 节点: 5/5 Ready | 版本: v1.29.4
   ├─ CPU 平均: 42% (最高: node-3 78%)
   ├─ Mem 平均: 58% (最高: node-7 85%)
   └─ 磁盘: 无 DiskPressure 警告

⚠️  Pod 异常: 3 个 (共计 142 个运行中)
   ├─ 🔴 payment-debug-7d8f9          CrashLoopBackOff (重启 47 次)
   ├─ 🟡 report-worker-5c3a1          OOMKilled (内存限制 512Mi 偏低)
   └─ 🟡 notification-cache-f2b8d     ImagePullBackOff (registry 超时)

⚠️  PDB 余量不足:
   └─ payment-min-available: 最少 2 / 当前 2 (无余量，drain 会阻塞)

🟢 资源配额: 3 个 namespace 在配額内
   ├─ production: cpu=85/100, mem=120Gi/200Gi, pods=42/50
   ├─ staging:    cpu=12/50,  mem=18Gi/100Gi,  pods=8/30
   └─ monitoring: cpu=8/20,   mem=22Gi/60Gi,   pods=15/25

📊 最近告警 (1h):
   ├─ 14:15  HighCPU node-3 (82%, 持续 5m)
   └─ 14:02  DiskPressure node-7 (89%, 持续 2m) — 已恢复

🔄 部署状态:
   ├─ payment: v3.2.1 (26min ago) ✅
   ├─ order-svc: v2.8.0 (2h ago) ✅
   └─ report-worker: v3.1.0 (5h ago) ✅

⏱️  扫描完成: 1.2s (5 个并行查询)

💡 建议关注: payment-debug CrashLoopBackOff (47 次重启, 建议排查)
```

### 9.8.4 配置

```yaml
# config.yaml
health_overview:
  startup_show: true             # 启动时自动展示概览
  startup_summary_only: false    # true: 仅显示摘要行; false: 完整展示
  check_interval: "manual"       # "manual" | "5m" | "15m" (定时自动刷新，Phase 2)
  include:
    nodes: true                  # 节点状态
    pods: true                   # 异常 Pod
    pdbs: true                   # PDB 余量
    quotas: true                 # ResourceQuota
    alerts: false                # 最近告警（需 Prometheus，默认关闭）
    deployments: true            # 最近部署
```

### 9.8.5 启动交互流程

```
$ ./ops-ai

═══ prod-us-east-1 ═══
🟢 5/5 nodes | ⚠️ 3 abnormal pods | 🟢 quotas OK
ℹ️  输入 /health 查看完整概览，或直接描述你的需求

> _
```

用户看到这行摘要后，可以直接开始提问，也可以用 `/health` 展开详细报告。不会让用户面对一个空白输入框不知从何问起。

---

# 第十部分：完整 Go 接口定义 v1.1

## 10.1 核心接口

```go
// === 工具接口 ===

// RiskLevel 操作风险级别
type RiskLevel int

const (
    RiskLevelReadOnly  RiskLevel = 0 // L0: 纯只读
    RiskLevelLow       RiskLevel = 1 // L1: 低风险
    RiskLevelMedium    RiskLevel = 2 // L2: 中等风险
    RiskLevelHigh      RiskLevel = 3 // L3: 高风险
    RiskLevelForbidden  RiskLevel = 4 // L4: 禁区
)

// Environment 运行环境
type Environment string

const (
    EnvDev        Environment = "dev"
    EnvStaging    Environment = "staging"
    EnvProduction Environment = "production"
)

// ToolContext 工具执行时的上下文
type ToolContext struct {
    SessionID       string              // 会话 ID
    ParentSessionID string              // 父会话 ID（导入的会话追溯，§23）
    Environment     Environment         // 当前环境
    Namespace       string              // 当前 K8s namespace
    ClusterName     string              // 集群名称（用于 L3 确认）
    IsCriticalSvc   bool                // 是否操作核心服务
    K8sClient       kubernetes.Interface // K8s 客户端
    CloudProvider   CloudProvider       // 云平台客户端（§25，nil = 未配置）
    WorkingDir      string              // 工作目录
    SnapshotDir     string              // 快照存储目录
    IsAlertTriggered bool               // 是否由告警自动触发（§24）
    AlertSource     string              // 触发告警来源（alertmanager/pagerduty/...）
}

// ToolExecInput 工具执行输入（LLM function call 解析后的结构化参数）
type ToolExecInput struct {
    ToolName   string
    Parameters map[string]interface{} // LLM 传入的原始参数
    RawJSON    string                 // 原始 JSON（用于日志审计）
}

// ToolResult 工具执行结果
type ToolResult struct {
    Success      bool              // 是否成功
    Output       string            // 人类可读的输出（会展示给用户和 LLM）
    Error        error             // 错误信息
    ExitCode     int               // 命令退出码（0 = 成功）
    Duration     time.Duration     // 执行耗时
    IsTruncated  bool              // 输出是否被截断
    Artifacts    []ToolArtifact    // 附加产物（如快照文件）
}

// ToolArtifact 工具执行的附加产物
type ToolArtifact struct {
    Type        string // "snapshot", "log", "diff", "report"
    Description string // 人类可读描述
    FilePath    string // 本地文件路径
    ContentType string // MIME 类型
}

// DryRunResult 预检结果（L2+ 工具必须实现）
type DryRunResult struct {
    SafeToProceed   bool              // 是否可以安全执行
    AffectedCount   int               // 受影响资源数
    AffectedResources []string        // 受影响的资源名称
    ImpactDuration  time.Duration     // 预计影响时长
    Warnings        []string          // 警告信息
    PreflightChecks []PreflightResult // 预检结果
}

// PreflightResult 单项预检
type PreflightResult struct {
    CheckName string // 检查项名称
    Passed    bool   // 是否通过
    Detail    string // 详细信息
}

// SnapshotResult 快照结果
type SnapshotResult struct {
    ID           string    // 快照 ID（UUID）
    ResourceType string    // Deployment / StatefulSet / ConfigMap ...
    ResourceName string    // 资源名称
    Namespace    string    // 命名空间
    Content      string    // 快照内容（YAML）
    ContentHash  string    // SHA256 哈希
    CreatedAt    time.Time // 创建时间
    FilePath     string    // 本地文件路径
}

// Tool 统一的工具接口
type Tool interface {
    // 基础信息（用于 LLM function calling 注册）
    Name() string
    Description() string                    // 供 LLM 理解工具用途
    ParametersSchema() map[string]interface{} // JSON Schema 格式的参数定义

    // 安全分级（工具自身定义的固有风险级别）
    IntrinsicRiskLevel() RiskLevel

    // 执行
    Execute(input ToolExecInput, ctx ToolContext) (ToolResult, error)

    // 预检（L2+ 工具必须实现，L0-L1 可选）
    DryRun(input ToolExecInput, ctx ToolContext) (*DryRunResult, error)

    // 快照（L3 工具必须实现）
    Snapshot(input ToolExecInput, ctx ToolContext) (*SnapshotResult, error)
}

// === 安全网关接口 ===

// SafetyGate 安全网关
type SafetyGate interface {
    // ResolveLevel 综合判定最终风险级别
    // finalLevel = tool.IntrinsicRiskLevel() + envWeight + criticalSvcWeight
    ResolveLevel(tool Tool, ctx ToolContext) RiskLevel

    // Intercept 拦截检查
    // L0 → 直接放行
    // L1 → 请求用户确认（Enter）
    // L2 → 展示 DryRun + 请求用户确认（yes）
    // L3 → 展示 DryRun + Snapshot + 请求强确认（集群名）
    // L4 → 拒绝
    Intercept(tool Tool, input ToolExecInput, ctx ToolContext, level RiskLevel) (*InterceptResult, error)
}

// InterceptResult 拦截结果
type InterceptResult struct {
    Allowed         bool           // 是否允许执行
    Reason          string         // 原因描述（拒绝时说明为什么）
    RequiredConfirm ConfirmMethod  // 需要的确认方式
    DryRunResult    *DryRunResult  // L2+ 附带的预检结果
    SnapshotResult  *SnapshotResult // L3 附带的快照结果
    RollbackCmd     string         // L3 附带的回滚命令
}

// ConfirmMethod 确认方式
type ConfirmMethod int

const (
    ConfirmNone       ConfirmMethod = iota // L0: 无需确认
    ConfirmEnter                            // L1: 回车确认
    ConfirmYes                              // L2: 输入 yes
    ConfirmClusterName                      // L3: 输入集群名
    ConfirmDualApproval                     // L3+: 双人审批
)

// === Agent Loop 接口 ===

// Agent 核心 agent
type Agent struct {
    llmClient     LLMClient
    safetyGate     SafetyGate
    tools          map[string]Tool      // 注册的工具
    toolResults    []ToolCallRecord     // 本轮所有工具调用记录
    conversation   []Message            // 完整对话历史
    config         AgentConfig
}

// AgentConfig Agent 配置
type AgentConfig struct {
    MaxToolCallsPerTurn     int           // 单轮最大工具调用次数 (default: 20)
    ToolExecTimeout         time.Duration // 单个工具执行超时 (default: 60s)
    MaxConsecutiveTimeouts  int           // 连续超时上限 (default: 3)
    MaxTotalTurnTime        time.Duration // 单轮总时长上限 (default: 300s)
    MaxParallelTools        int           // 最大并行工具数 (default: 2)
}

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
    ID          string
    ToolName    string
    Input       ToolExecInput
    Result      ToolResult
    RiskLevel   RiskLevel
    Duration    time.Duration
    Timestamp   time.Time
    ApprovedBy  string // 空 = 自动批准 / "user:enter" / "user:yes" / "user:cluster-name"
}

// LLMClient LLM 客户端接口
type LLMClient interface {
    // Chat 发送对话并获取响应（流式）
    Chat(ctx context.Context, messages []Message, tools []ToolDef) (<-chan LLMStreamEvent, error)
}

// LLMStreamEvent LLM 流式事件
type LLMStreamEvent struct {
    Type    StreamEventType // "text_chunk" | "function_call" | "done" | "error"
    Content string          // 文本片段 或 function_call JSON
    Error   error
}

type StreamEventType string

const (
    StreamEventTextChunk    StreamEventType = "text_chunk"
    StreamEventFunctionCall StreamEventType = "function_call"
    StreamEventDone         StreamEventType = "done"
    StreamEventError        StreamEventType = "error"
)

// ToolDef LLM function calling 的工具定义（用于注册到 LLM）
type ToolDef struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// Message 对话消息
type Message struct {
    Role       string     // "user" | "assistant" | "tool"
    Content    string     // 文本内容
    ToolCalls  []ToolCall // tool 角色时的工具调用结果
    ToolCallID string     // tool 角色时的调用 ID
}

// ToolCall 工具调用
type ToolCall struct {
    ID        string
    Name      string
    Arguments map[string]interface{}
}
```

## 10.2 MVP 目录结构

```
ops-ai/
├── cmd/
│   ├── ops-ai/
│   │   └── main.go                  # 主入口
│   └── alertd/
│       └── main.go                  # 告警接收守护进程入口（§24）
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent 主循环
│   │   ├── context.go               # 上下文管理器（CMDB + RAG 注入）
│   │   ├── systemprompt.go          # System Prompt 模板
│   │   └── session.go               # 会话管理 + 导出/导入 + 崩溃恢复（§23, §29）
│   ├── onboard/                      # 首次运行体验（§26）
│   │   ├── onboarder.go             # 启动自检引擎
│   │   ├── checks.go                # 内置自检项（kubeconfig/K8s/LLM/RBAC）
│   │   └── wizard.go                # /setup wizard 交互式配置向导
│   ├── safety/
│   │   ├── gate.go                  # SafetyGate 实现
│   │   ├── classifier.go            # 环境识别 + 核心服务判定
│   │   └── confirm.go               # 用户确认交互
│   ├── preflight/                    # 统一 pre-flight 编排（§28）
│   │   ├── orchestrator.go          # PreFlightOrchestrator 编排器
│   │   ├── checker.go               # PreFlightChecker 接口
│   │   └── builtin/                 # 内置检查器
│   │       ├── resourcequota.go
│   │       ├── limitrange.go
│   │       ├── pdb.go
│   │       ├── psa.go
│   │       ├── webhook.go
│   │       ├── operator.go
│   │       └── networkpolicy.go
│   ├── tools/
│   │   ├── registry.go              # 工具注册表
│   │   ├── kubectl/
│   │   │   ├── get.go
│   │   │   ├── describe.go
│   │   │   ├── logs.go              # 含流式日志（§27）
│   │   │   ├── scale.go
│   │   │   ├── rollout.go
│   │   │   ├── apply.go
│   │   │   ├── delete.go
│   │   │   ├── exec.go
│   │   │   └── impact.go
│   │   ├── helm/
│   │   │   ├── list.go
│   │   │   ├── get.go
│   │   │   └── upgrade.go
│   │   ├── cloud/                    # 云资源工具（§25）
│   │   │   ├── provider.go
│   │   │   ├── aws.go
│   │   │   ├── rds.go
│   │   │   ├── alb.go
│   │   │   ├── s3.go
│   │   │   └── route53.go
│   │   └── terraform/               # Phase 2
│   ├── llm/
│   │   ├── client.go
│   │   ├── openai.go
│   │   ├── claude.go
│   │   └── ollama.go
│   ├── cost/                         # 成本追踪（§30）
│   │   ├── tracker.go               # CostTracker 使用量记录
│   │   ├── pricing.go               # 模型定价表
│   │   └── budget.go                # 预算管理
│   ├── blastradius/                  # 爆炸半径控制（§5.14/§32）
│   │   └── tracker.go               # SessionBlastRadius 会话操作计数+熔断
│   ├── gitops/                       # GitOps 冲突感知（§5.15/§33）
│   │   ├── detector.go              # GitOpsDetector ArgoCD/Flux 管理检测
│   │   └── bridge.go                # GitOpsBridge 暂停/恢复 sync
│   ├── netdiag/                      # 集群网络诊断（§38）
│   │   ├── runner.go                # NetDiagRunner 四层诊断引擎
│   │   ├── dns.go                   # DNS 解析检查
│   │   ├── endpoints.go             # Service 端点检查
│   │   ├── netpol.go                # NetworkPolicy 分析
│   │   └── cni.go                   # CNI 检测
│   ├── deps/                         # 依赖健康检查（§37）
│   │   ├── checker.go               # DependencyHealthChecker
│   │   ├── db.go                    # DB 连接探针
│   │   ├── redis.go                 # Redis 连接探针
│   │   ├── http.go                  # HTTP 可达性探针
│   │   └── mq.go                    # 消息队列探针
│   ├── verify/                       # 变更后验证（§36）
│   │   ├── validator.go             # PostOpValidator
│   │   └── strategies.go            # 按操作类型的验证策略
│   ├── context/                      # 多集群上下文（§35）
│   │   └── manager.go               # ContextManager 上下文切换+缓冲期
│   ├── alertd/                       # 告警守护进程（§24）
│   │   ├── server.go
│   │   ├── router.go
│   │   ├── runner.go
│   │   └── notifier.go
│   ├── snapshot/
│   │   └── manager.go
│   ├── tui/
│   │   ├── app.go
│   │   ├── chat.go
│   │   ├── preview.go
│   │   ├── status.go                # 含 ☁️ 云资源状态行
│   │   ├── onboard.go               # 自检结果展示（§26）
│   │   └── logstream.go             # 流式日志渲染（§27）
│   ├── audit/
│   │   └── logger.go                # 审计日志（§21）
│   └── config/
│       └── config.go
├── pkg/
│   └── types/
│       └── types.go
├── deploy/                           # 容器化部署清单（§31）
│   ├── Dockerfile
│   ├── k8s/
│   │   ├── cronjob-health-check.yaml
│   │   ├── cronjob-drift-check.yaml
│   │   └── rbac-readonly.yaml
│   └── ci/
│       ├── github-actions.yaml
│       └── gitlab-ci.yaml
├── config.example.yaml
├── go.mod
└── go.sum
```

---

# 第十一部分：System Prompt 模板 v1.1

## 11.1 完整 System Prompt

```
You are an expert Kubernetes operations assistant running in the terminal.
Your primary user is a Site Reliability Engineer (SRE) who needs help with
infrastructure operations.

=== YOUR IDENTITY ===
- You are NOT a developer writing code. You are an SRE operating infrastructure.
- Your tools give you direct access to Kubernetes clusters. Use them responsibly.
- Always prioritize safety over speed. Explain what you're doing before you do it.

=== OPERATING ENVIRONMENT ===
Current context:
  Environment: {environment}  (dev/staging/production)
  Cluster: {cluster_name}
  Namespace: {namespace}
  Critical services: {critical_services_list}

=== CORE RULES ===

1. READ FIRST, ACT SECOND.
   - Always describe/get resources before modifying them.
   - Never assume a resource exists — verify first.
   - If a command fails, read the error message carefully before retrying.
     Do NOT blindly retry failing commands more than once.

2. EXPLAIN BEFORE MODIFYING.
   - When suggesting a modification, explain:
     a) What you're going to do
     b) Why it's needed
     c) What the expected outcome is
     d) What could go wrong
   - If the user's request is ambiguous, ask clarifying questions before acting.

3. SAFETY HARD STOP.
   The following are NEVER allowed, do not attempt:
   - kubectl delete namespace (any namespace)
   - kubectl delete crd (any CRD)
   - kubectl exec with: rm, dd, mkfs, fdisk, shutdown, reboot, iptables -F
   - Any command that modifies /etc, /var, /root paths via kubectl exec
   - Any SQL DROP / TRUNCATE / DELETE without WHERE clause
   - kubectl delete --all (any resource type)
   - Any operation in kube-system namespace without explicit user approval

4. SCOPE AWARENESS.
   - Default to the current namespace. Only cross namespaces if explicitly asked.
   - If the user asks about "everything" or "all pods", confirm scope first.
   - For production clusters, be extra cautious about ANY modification.

5. CLUSTER POLICY AWARENESS (v1.3).
   - BEFORE suggesting Pod creation, check the namespace's PSA label
     (pod-security.kubernetes.io/enforce). If set to "restricted", NEVER
     suggest privileged containers, hostNetwork, or running as root.
   - When a kubectl command is rejected by an Admission Webhook, read the
     webhook's error message and explain which policy blocked it. NEVER
     suggest bypassing or deleting the webhook without explicit approval.
   - Before any 'kubectl drain', automatically check PDBs in affected
     namespaces and warn if the drain will be blocked.
   - Before modifying any Deployment/StatefulSet/DaemonSet, check
     metadata.ownerReferences. If owned by a CRD/Operator, warn the user
     that direct modifications may be overwritten by the Operator.
     Suggest modifying the CR instead.
   - For GPU-related requests (nvidia.com/gpu, amd.com/gpu), recognize
     that GPU nodes are a constrained resource. Check node capacity first
     before suggesting GPU Pod creation.

6. TOOL USAGE.
   - You have a maximum of {max_tool_calls} tool calls per conversation turn.
   - You may call up to 2 read-only tools in parallel (get, describe, logs).
   - Modifying tools must be called one at a time.
   - Each tool call has a 60-second timeout. If kubectl is slow, try with
     a narrower scope (--selector, specific resource name).

7. ERROR RECOVERY.
   - If a tool call returns no resources: suggest checking namespace/name/cluster.
   - If a tool call returns 403 Forbidden: tell the user which RBAC permission
     is likely missing, do NOT suggest running as cluster-admin.
   - If a tool call returns 404 NotFound: suggest checking the resource name,
     do NOT suggest creating random resources.

8. KUBERNETES BEST PRACTICES.
   - Prefer kubectl apply over kubectl create (declarative over imperative).
   - Never suggest privileged containers or hostNetwork unless specifically asked.
   - Always mention if an operation requires rolling restart of pods.
   - For StatefulSet operations, mention that Pods restart in ordinal order.
   - If suggesting a ConfigMap/Secret change, warn that existing Pods won't
     automatically pick up the change (they need a restart).
   - For GPU workloads (nvidia.com/gpu, amd.com/gpu), always check node GPU
     capacity first. If GPU nodes are at capacity, suggest checking other GPU
     Pods that could be preempted or using lower GPU request.
   - When working with CRDs, always check available API versions first:
     kubectl api-resources | grep <crd-group>. Prefer v1 over v1beta1 over v1alpha1.
   - NEVER suggest bypassing Admission Webhooks (no --validate=false, no
     --dry-run=server workarounds, no removing webhook configurations).

9. RESPONSE FORMAT.
   - Keep responses concise. The user is in a terminal.
   - Format long outputs with clear headings and structure.
   - Use kubectl get with -o wide or specific columns instead of dumping
     full YAML unless the user explicitly asks for it.
   - When showing diff or change impacts, use a structured format:
     Resource: deployment/payment
     Change:  replicas 2 → 3
     Impact:  1 new Pod created, no service interruption

10. WHEN YOU DON'T KNOW.
   - If you're unsure about a K8s concept, say so and suggest kubectl explain.
   - If the user's architecture is unclear, ask about their setup before
     making assumptions.
   - For cluster-specific configurations (CNI, storage class, ingress controller),
     suggest checking with kubectl rather than guessing.
```

## 11.2 System Prompt 中的动态注入变量

```go
type SystemPromptVars struct {
    Environment         string   // "dev" | "staging" | "production"
    ClusterName         string   // 从 kubeconfig context 提取
    Namespace           string   // 当前 namespace
    CriticalServices    []string // 核心服务列表
    MaxToolCalls        int      // 默认 20
    RecentIncidents     string   // 最近相关 incident 的 RAG 结果（Phase 2）
    RecentDeployments   string   // 最近部署记录（Phase 2）
}
```

---

# 第十二部分：开发路线图 v1.1

## Phase 1: MVP — 可用的 K8s 运维副驾驶（6-8 周）

| 周 | 里程碑 | 具体交付物 |
|----|--------|-----------|
| W1 | 项目脚手架 | `go mod init`, 目录结构创建, Bubble Tea TUI 骨架, YAML 配置加载, 启动健康概览（§9.8）, **启动自检引擎 + 交互式配置向导（§26）**, **多集群上下文管理（§35）** |
| W1-2 | K8s Client 集成 | client-go 初始化, kubeconfig 解析, 多 context 切换, L0 命令实现（get/describe/logs/top）, RBAC 权限自检, 审计日志框架（§21）, **GitOps 冲突检测器（§33）** |
| W2 | LLM 适配层 | OpenAI + Claude API 客户端, Function Calling 注册, 流式响应, System Prompt 注入 |
| W3 | Agent Loop 实现 | 核心循环（§8.1）, 工具注册表, 并行调度, 终止条件, **增量 checkpoint + 崩溃恢复（§29）**, **全局超时 + 死循环检测（§8.6/§34）** |
| W3-4 | 安全网关 | SafetyGate 完整实现（§5）, 环境识别, 核心服务判定, L0-L4 拦截, **Pre-flight 统一编排框架（§28）**, **会话爆炸半径控制（§5.14/§32）**, **GitOps 冲突感知（§5.15）** |
| W4 | L1-L2 命令实现 | scale, rollout restart/undo, set image, apply, patch. 每个附带 DryRun, **流式日志（§27）**, **变更后自动验证（§36）** |
| W5 | 影响面分析引擎 | 直接引用关联分析（§6）, 仅 ConfigMap/Secret→Deployment/Svc→Ingress |
| W5-6 | 自动快照 | Snapshot 管理器（§7）, 快照存储/加载, 回滚命令生成 |
| W6 | L3 命令 + TUI 确认流 | delete（单资源）, helm uninstall, 集群名确认交互, 可选双人审批 |
| W6-7 | 异常处理 + 测试 + 新增模块 | 超时重试, RBAC 403 处理, --dry-run 模式, 单元测试 100+, 行为模拟测试 40+, 安全回归套件, kind 集群集成测试, **依赖服务连通性诊断（§37）**, **集群内网络诊断（§38）**, 会话导出/导入测试 (§23), alertd webhook 接收测试 (§24), 云资源工具单元测试 (§25) |
| W7-8 | 文档 + 打包 + 容器化 | Quick Start, RBAC 部署指南, **alertd systemd 部署指南**, `brew install`, `go install`, GoReleaser, **Docker buildx 多架构镜像（§31）**, **K8s CronJob 模板验证** |

**MVP 验收标准**：
1. 在 kind 测试集群中，能用自然语言完成一次"排查 Pod CrashLoopBackOff → 定位 ConfigMap 缺失 → 创建 ConfigMap → 重启 Deployment → 验证恢复"的完整流程。
2. 安全回归测试套件 100% 通过（§14.3 所有 forbiddenCommands 被 L4 拦截）。
3. 单元测试覆盖率 ≥ 70%，行为模拟测试覆盖所有 L0-L4 路径。
4. Secret Redact 策略在测试中验证：含密码的 Secret → LLM 侧看到 `[REDACTED]`。
5. `--dry-run` 模式：完整路径走通（L0 真实执行 + L1+ 仅预览），不产生任何集群变更。
6. RBAC 自检：使用 readonly SA 启动 → 提示缺少写权限 → 切换到 readwrite SA → 自检通过。
7. 审计日志：L1+ 操作自动写入 audit.jsonl，含完整操作上下文。
8. 集群健康概览：`./ops-ai` 启动时自动展示 5 维度摘要，`/health` 展示完整报告。
9. CI/CD 模式：`--no-tui --yes` 自动确认 L1-L2 操作，`--pipe` 纯文本输出可被 grep 消费。
10. 会话导出/导入：`/export` 导出完整会话 → 另一台机器 `--import` 恢复上下文 → Agent 基于已有排查继续。
11. alertd 告警接收：模拟 AlertManager webhook → alertd 接收 → 自动执行 L0 诊断 → 输出排查报告。
12. 云资源只读诊断：配置 AWS 凭证 → `cloud_rds_describe_instance` 成功返回 RDS 状态 → 通过 annotation 自动关联。
13. **首次运行体验**：无 kubeconfig → 展示 4 种配置方式；无 API key → 展示获取指引 → `/setup wizard` 交互式完成配置。
14. **流式日志**：`kubectl logs -f --stop-on="ERROR"` → TUI 实时滚动 → 匹配到 ERROR 自动终止 → Agent 提取上下文分析。
15. **崩溃恢复**：模拟 Agent Loop panic → 重启 `./ops-ai` → 自动检测到未完成会话 → 选择恢复 → 对话上下文完整还原。
16. **容器化部署**：`docker run ops-ai --no-tui --yes --pipe "/health"` → 正常输出健康概览 → 退出码正确。
17. **会话爆炸半径熔断**：在同一 namespace 连续 3 次 L2 操作 → 触发熔断 → 展示已执行操作列表 → 用户可人工审核或回滚。
18. **GitOps 冲突警告**：修改 ArgoCD 管理的 Deployment → 弹出冲突警告 → 显示正确操作路径 → 用户可选择暂停 sync 后继续。
19. **Agent Loop 全局超时**：`./ops-ai --timeout 30s "排查 payment"` → 超时后展示已完成步骤摘要 → 支持 /continue 继续。
20. **多集群上下文安全**：切换到 production context → 状态栏变红 → 5 分钟内 L2+ 操作额外要求输入集群名确认。
21. **变更后验证**：修改 ConfigMap → 自动检测 Pod 是否重启 → 未重启时提示"建议 rollout restart"。
22. **依赖连通性诊断**：`/health --deep payment` → 自动在 Pod 内执行 DB/Redis/HTTP 连接测试 → 输出三层健康报告。
23. **集群网络诊断**：`/net-diag deploy/A → deploy/B:8080` → DNS→Service→NetworkPolicy→Pod 四层诊断 → 输出根因。

## Phase 2: Beta（+8 周）& Phase 3: GA（再+8 周）

Phase 2 新增（从 v1.6 P1 擢升）：Terraform 深度集成、Prometheus+Loki 完整查询、Runbook RAG、告警自动修复闭环（diagnose+remediate）、云资源写操作（L1-L2）、会话 Live 协作。

参见 v1.0 原始路线图，完整内容无变更。

---

# 第十三部分：技术选型 v1.8（精确到版本）

| 组件 | 选型 | 版本 | 理由 |
|------|------|------|------|
| **语言** | Go | 1.22+ | K8s 生态标准语言 |
| **K8s Client** | client-go | v0.30+ | 安全、类型化、支持 Watch |
| **AWS SDK** | aws-sdk-go-v2 | latest | 云资源诊断（§25），按服务模块引入 |
| **TUI** | Bubble Tea | v1.x | Charmbracelet 生态，Claude Code 风格 |
| **LLM 适配** | 自研（HTTP + SSE） | — | langchaingo 抽象层太厚 |
| **OpenAI SDK** | go-openai | v1.x | 社区标准 |
| **Claude SDK** | anthropic-sdk-go | latest | 官方 SDK |
| **MCP 协议** | mcp-go | latest | Phase 2，标准化的工具集成 |
| **向量数据库** | ChromaDB（独立进程） | latest | Phase 2，Runbook RAG |
| **持久化** | SQLite (mattn/go-sqlite3) | latest | 会话历史、快照索引、崩溃恢复、成本归属、知识库索引 |
| **容器镜像** | distroless/static-debian12 | nonroot | §31 最小攻击面容器镜像 |
| **容器构建** | Docker Buildx | latest | 多架构 (amd64/arm64) 统一构建 |
| **配置** | Viper + YAML | v1.x | 运维领域标准 |
| **发布** | GoReleaser + Homebrew + GHCR | — | macOS/Linux 覆盖 + 容器镜像 |
| **测试** | kind + testify + localstack | — | K8s 集成测试 + 云资源 mock |

---

# 第十四部分：测试策略（v1.2 新增）

v1.1 只提了"kind 集成测试"，但 Agent 行为的测试和普通 CLI 测试完全不同。需要四层测试体系。

## 14.1 四层测试金字塔

```
        ┌─────────────┐
        │ E2E 场景测试  │  ← kind 集群 + 真实 LLM（可选 mock）
        │   5-10 个场景  │
        ├─────────────┤
        │  集成测试     │  ← mock LLM + 真实 client-go
        │   20-30 个    │
        ├─────────────┤
        │  行为模拟测试  │  ← mock LLM 响应 + mock K8s client
        │   40-60 个    │
        ├─────────────┤
        │  单元测试     │  ← 纯 Go 测试，无外部依赖
        │  100+ 个     │
        └─────────────┘
```

## 14.2 各层详细说明

### 层 1：单元测试（`go test ./...`）

覆盖所有纯逻辑组件，不依赖 K8s 或 LLM：

| 测试对象 | 关键用例 |
|---------|---------|
| `SafetyGate.ResolveLevel()` | 所有 4×3×2 = 24 种（风险级别 × 环境 × 核心服务）组合 |
| `SecretRedacter.Redact()` | 输入完整 Secret YAML → 断言敏感字段全部替换 |
| `SnapshotManager.Store/Load()` | 快照 CRUD + 哈希校验 |
| `ImpactAnalyzer.FindReferences()` | mock K8s client 注入测试数据 |
| `ExecCommandClassifier` | 输入各类命令 → 断言正确分类为 L1/L2/L4 |
| `HelmArgValidator` | 正常/恶意参数 → 断言拦截结果 |
| `ContextBudget` | 模拟 token 累积 → 断言截断/摘要/压缩触发点 |

### 层 2：行为模拟测试（`go test -tags=behavior`）

Mock LLM 响应来验证 Agent Loop 的行为确定性：

```go
// 示例: 验证 L3 操作被正确拦截
func TestAgentLoop_L3RequiresClusterNameConfirmation(t *testing.T) {
    mockLLM := NewMockLLMClient()
    // 预设 LLM 会返回 function_call: kubectl_delete deployment payment -n prod
    mockLLM.AddResponse(functionCall("kubectl_delete", map[string]interface{}{
        "name": "payment", "namespace": "prod",
    }))

    agent := NewAgent(mockLLM, realSafetyGate, mockTools)
    
    events := agent.Run("删除 payment 部署")
    
    // 断言: 安全网关返回 L3，Agent 要求输入集群名确认
    assert.Contains(t, lastEvent, "输入当前集群名确认")
    // 断言: 没有实际执行删除
    assert.NotContains(t, lastEvent, "deleted")
}
```

关键行为测试清单：

| 测试场景 | 验证点 |
|---------|-------|
| L0 命令自动执行 | 不触发确认，结果直接注入对话 |
| L1 命令等待 Enter | 展示预览，不执行直到确认 |
| L2 命令等待 yes | 展示 DryRun + 影响面 |
| L3 命令等待集群名 | 展示快照 + 回滚方案 |
| L4 命令直接拒绝 | 返回替代方案建议 |
| 连续 3 次超时 | Agent Loop 终止 |
| 20 次 tool call 达到上限 | Agent Loop 终止 |
| LLM 返回格式错误参数 | 反馈给 LLM 修正，不计入次数 |
| 并行 L0 调用（2 个） | 同时执行，结果一起返回给 LLM |

### 层 3：集成测试（`go test -tags=integration`）

在 kind 集群中运行，使用真实 client-go 但 mock LLM：

```bash
# 测试前: 用 kind + kubectl 创建已知故障状态
kind create cluster --name ops-ai-test
kubectl apply -f testdata/crashloop-pod.yaml
kubectl apply -f testdata/misconfigured-configmap.yaml

# 预设故障场景
testdata/
├── crashloop-pod.yaml          # Pod 必定 CrashLoopBackOff
├── misconfigured-cm.yaml       # ConfigMap 缺少必需的 key
├── resource-quota-exceeded.yaml # 触发 ResourceQuota 限制
├── pending-pvc.yaml            # PVC 无匹配 StorageClass
└── networkpolicy-deny.yaml     # NetworkPolicy 阻断流量
```

### 层 4：E2E 场景测试（`go test -tags=e2e`）

使用真实 LLM（或本地 Ollama），在 kind 集群中端到端验证：

| 场景 | 描述 | 验收标准 |
|------|------|---------|
| CrashLoopBackOff 排查 | Pod 崩溃 → Agent 定位原因 → 修复 | 完整链路走通 |
| ConfigMap 缺失修复 | 服务启动失败 → 创建 ConfigMap → 重启 → 验证恢复 | 同上 |
| 扩容验证 | 请求扩容 → L1 确认 → 执行 → 验证 Pod 数 | 同上 |
| 危险操作拦截 | 请求删 namespace → L4 拒绝 | 操作未执行 |

## 14.3 安全回归测试套件（每次 PR 必跑）

```go
// security_regression_test.go
// 这个测试文件包含所有已知危险命令组合
// 任何安全网关修改后必须全部通过

var forbiddenCommands = []string{
    "kubectl delete namespace production",
    "kubectl delete crd certificates.cert-manager.io",
    "kubectl delete --all pods -n production",
    "kubectl exec payment-pod -- rm -rf /data",
    "kubectl exec payment-pod -- iptables -F",
    "kubectl exec payment-pod -- apt install nmap",
    "helm uninstall ingress-nginx -n kube-system",
}

func TestSecurityRegression_AllForbiddenCommandsRejected(t *testing.T) {
    for _, cmd := range forbiddenCommands {
        t.Run(cmd, func(t *testing.T) {
            level := safetyGate.ResolveLevel(parseCommand(cmd), productionContext)
            assert.Equal(t, RiskLevelForbidden, level, 
                "命令 %s 应该被 L4 拦截但实际为 %v", cmd, level)
        })
    }
}
```

---

# 第十五部分：LLM 成本与速率控制（v1.2 新增）

v1.1 支持多 LLM 后端但没有成本控制。一个不留神，单次排障对话就能烧掉 $0.5。

## 15.1 成本估算（Claude Sonnet 2025 定价）

| 场景 | Tool Calls | Input Tokens | Output Tokens | 单次成本 |
|------|-----------|-------------|--------------|---------|
| 简单查询（kubectl get pods） | 1-2 | ~1K | ~200 | ~$0.005 |
| 中等排障（定位配置问题） | 5-8 | ~12K | ~800 | ~$0.05 |
| 完整排障（变更+验证） | 10-15 | ~25K | ~1500 | ~$0.12 |
| 重度使用（含 describe + logs） | 15-20 | ~50K | ~2000 | ~$0.25 |

**团队成本预估**：

| 团队规模 | 日均会话/人 | 月成本（Claude） | 优化后（本地模型分担 60% 查询） |
|---------|-----------|----------------|---------------------------|
| 3 人 | 10 | ~$90 | ~$36 |
| 10 人 | 10 | ~$300 | ~$120 |
| 50 人 | 10 | ~$1500 | ~$600 |

## 15.2 控制策略

### 策略 1：本地模型优先路由

```go
// LLM 路由决策
func (r *Router) Route(request ChatRequest) LLMClient {
    // 简单查询 → 本地 Ollama（免费）
    if isSimpleQuery(request) {
        return r.ollamaClient
    }
    // 复杂推理 → Cloud LLM
    return r.cloudClient
}

func isSimpleQuery(req ChatRequest) bool {
    simpleVerbs := []string{"get", "list", "describe", "logs", "status", "top", "show"}
    for _, verb := range simpleVerbs {
        if strings.Contains(strings.ToLower(req.UserInput), verb) {
            return true // 用本地模型处理
        }
    }
    return false
}
```

### 策略 2：TUI 实时成本展示

```
┌─ ops-ai ─── context: prod-cluster ─── ns: payment ─── 14:32:05 ─┐
│ 本轮: 5/20 calls │ tokens: 18.2K │ 💰 $0.04 │ 月累计: $12.30    │
└─────────────────────────────────────────────────────────────────┘
```

### 策略 3：月度预算上限（企业版）

```yaml
# config.yaml
cost_control:
  monthly_budget: 50         # 月度 LLM API 预算（美元）
  warn_threshold: 0.8        # 80% 预算时警告
  block_threshold: 1.0       # 100% 预算时阻断云 LLM，仅允许本地模型
  local_model: ollama        # 超预算后的 fallback 模型
```

### 策略 4：减少冗余调用

| 优化 | 效果 |
|------|------|
| 相同资源 30s 内不重复查询 | -30% 重复 get |
| kubectl get 结果缓存直到下次修改操作 | -25% 总调用 |
| 工具输出截断（§8.5） | -40% input tokens |
| 会话自动摘要（§8.5） | -50% 长会话成本 |

---

# 第十六部分：风险 & 开放问题 v1.3

| 风险 | 严重程度 | 缓解措施 |
|------|---------|---------|
| LLM 幻觉导致危险操作 | 🔴 高 | 五级安全网关 + 命令沙箱 + 自动快照 |
| 用户不信任 AI 执行操作 | 🟡 中 | 渐进式信任（L0→L1→L2→L3） |
| client-go 学习曲线 | 🟡 中 | W1-2 集中攻关，参考 kubectl 源码 |
| 工具链碎片化 | 🟡 中 | MCP 协议标准化，社区插件 |
| Go 生态 MCP 成熟度 | 🟢 低 | Phase 2 才需要，届时生态应已成熟 |
| Secret 明文泄露到 LLM API | 🔴 高 | §5.6 三层策略：LLM 层 redact + Agent 闭环 + 审计加密 |
| 上下文窗口爆满致排查中断 | 🟡 中 | §8.5 四级缓解 + TUI 预算展示 |
| Helm SDK API 兼容性 | 🟡 中 | §9.3 分阶段混合：只读 SDK + 写操作 exec helm |
| LLM API 成本失控 | 🟡 中 | §15 四策略：本地优先 + 预算上限 + 冗余去重 |
| 自动化测试覆盖不足 | 🟡 中 | §14 四层测试金字塔 + 安全回归 CI 门禁 |
| **Admission Webhook 静默拒绝** | 🟡 中 | §5.7 pre-flight 感知 + 拒绝原因解析 |
| **PDB 阻塞 drain 操作** | 🟡 中 | §5.8 pre-drain 自动检查 + 阻塞预警 |
| **PSA 导致 Pod 创建反复失败** | 🟡 中 | §5.9 namespace label 感知 + 自动合规配置 |
| **Operator 覆盖手动修改** | 🟡 中 | §5.10 ownerReferences 检测 + CR 操作引导 |
| **离线环境完全不可用** | 🔴 高 | §17 三层离线策略 + 内置 7B 本地模型 |
| **Config Drift 未被感知** | 🟡 中 | §9.5 ArgoCD/Flux diff 集成 |
| **部署时无 RBAC 清单** | 🔴 高 | §20 ReadOnly/ReadWrite ClusterRole + 绑定示例 |
| **ResourceQuota 静默拒绝** | 🟡 中 | §5.11 pre-flight 配额检查 + 超配额时解释原因 |
| **NetworkPolicy 网络假象** | 🟡 中 | §5.12 NetworkPolicy/Service Mesh 感知 + 诊断流程 |
| **操作错选 production namespace** | 🔴 高 | §5.13 L2+ 操作 namespace 主动确认 + TUI 防呆 |
| **ImagePullBackOff 无根因分析** | 🟡 中 | §9.6 六类错误分类 + 自动排查链路 |
| **K8s API 版本弃用** | 🟡 中 | §9.7 启动时 api-resources 缓存 + 已知弃用映射 |
| **缺少 port-forward 调试能力** | 🟡 中 | §9.1.4 port-forward + 自动 curl 验证 + TTL 自动关闭 |
| **缺少 kubectl cp 文件传输** | 🟡 中 | §9.1.5 按方向分级：拉出 L0 / 写入 L2 |
| **kubectl edit 交互模型未定义** | 🟡 中 | §9.1.6 外部 $EDITOR + 智能路由到 patch |
| **用户不敢把操作权交给 AI** | 🔴 高 | §19 --dry-run 全局预览模式 |
| **缺少操作审计日志** | 🔴 高 | §21 统一审计：多 sink + 异步 + fallback，合规红线 |
| **CI/CD 无法集成** | 🔴 高 | §22 无交互模式：--no-tui --yes + --pipe，三级自动化确认 |
| **打开后不知该做什么** | 🟡 中 | §9.8 启动健康概览 + /health 命令 |
| **on-call 接力上下文丢失** | 🔴 高 | §23 会话导出/导入：/export + /import，ParentSessionID 追溯链 |
| **告警无法自动触发排查** | 🔴 高 | §24 alertd 守护进程 + webhook 接口 + 告警路由规则引擎 |
| **K8s 排障卡在云资源边界** | 🔴 高 | §25 云 Provider 抽象 + AWS MVP (RDS/ALB/S3/Route53 L0 诊断) + K8s 资源自动关联 |
| **新用户首次运行即放弃** | 🔴 高 | §26 启动自检 4 阶段 + 诊断建议 + /setup wizard 交互式配置 |
| **缺少流式日志排障能力** | 🔴 高 | §27 kubectl logs -f 流式输出 + 模式匹配终止 + 多 Pod 聚合 + 硬超时 |
| **Pre-flight 检查零散无协调** | 🟡 中 | §28 统一编排框架 + 并行执行 + 独立超时 + 降级策略矩阵 |
| **Agent Loop 崩溃丢上下文** | 🔴 高 | §29 增量 checkpoint + 崩溃检测 + 会话恢复 + panic recovery wrapper |
| **缺少用户/团队成本归属** | 🟡 中 | §30 每次调用记录 token 消耗 + 按用户/团队聚合 + 预算告警 |
| **容器化部署模型缺失** | 🟡 中 | §31 Dockerfile + K8s CronJob 模板 + GitHub Actions/GitLab CI 集成 |
| **会话级爆炸半径无控制** | 🔴 高 | §5.14 会话级操作计数 + namespace 聚合 + 熔断机制 |
| **GitOps 手动变更被回滚** | 🔴 高 | §5.15 ArgoCD/Flux 检测 + 冲突警告 + /argo pause /argo resume |
| **Agent Loop 全局无超时** | 🔴 高 | §8.6 context.WithDeadline + 5m 默认超时 + 死循环检测 |
| **切换集群误操作风险** | 🔴 高 | §35 TUI 上下文颜色编码 + 切换冷却期 + 集群名确认 |
| **变更后未验证是否生效** | 🟡 中 | §36 按操作类型差异化验证 + 冷却期 + 自动修复建议 |
| **依赖层故障不可见** | 🟡 中 | §37 /health --deep + DB/Redis/MQ/HTTP 连通性诊断 |
| **集群内网络盲区** | 🟡 中 | §38 /net-diag + DNS→Service→NP→Pod 四层诊断 |

### 开放问题

1. **开源协议**：Apache 2.0 vs AGPL？
2. **安全网关是否独立库**：可被复用但有被解耦风险
3. **企业版功能边界**：SSO/审计之外是否包含高级编排？
4. **是否需要 Web 审计面板**：MVP 不需要，企业版可能需要
5. **离线模型选型**：Qwen2.5-Coder-7B 还是 Llama-3.1-8B？需实测对比 K8s 领域能力
6. **ArgoCD/Flux 深度集成**：MVP 仅 CLI 方式 vs Phase 2 REST API？
7. **云 Provider 扩展优先级**：GCP、Azure、阿里云、腾讯云——社区呼声最高的先做？
8. **知识库格式标准**（v1.8 新增）：采用 YAML 还是 Markdown？搜索策略：关键词匹配 vs 向量检索？

---

# 第十七部分：离线/Air-gapped 模式（v1.3 新增）

这是决定能否进入企业市场的关键能力。大量金融、政府、军工场景的 K8s 集群**完全没有互联网访问**。

## 17.1 问题定义

```
企业部署 ops-ai → ./ops-ai → 尝试连接 api.openai.com → 超时 → 报错退出
→ 运维团队: "这东西在我们环境根本用不了"
→ 目标用户中至少 30% 的企业集群无法访问公网 LLM API
```

## 17.2 三层离线策略

### 层 1：自动检测 + 降级

```go
// 启动时的连通性检测
func (a *Agent) detectConnectivity() ConnectivityLevel {
    endpoints := []string{
        a.config.OpenAIEndpoint,
        a.config.ClaudeEndpoint,
        a.config.OllamaEndpoint, // 本地地址
    }
    
    for _, ep := range endpoints {
        if isReachable(ep, 2*time.Second) {
            if strings.Contains(ep, "localhost") || strings.Contains(ep, "127.0.0.1") {
                return ConnectivityLocal  // 本地模型可用
            }
            return ConnectivityCloud    // 云 LLM 可用
        }
    }
    
    return ConnectivityNone  // 完全离线
}

type ConnectivityLevel int
const (
    ConnectivityCloud ConnectivityLevel = iota // 云 LLM 可达
    ConnectivityLocal                           // 仅本地模型
    ConnectivityNone                            // 完全离线（报错退出）
)
```

### 层 2：本地模型 Fallback

```yaml
# config.yaml
offline_mode:
  enabled: false              # 是否强制离线模式
  local_model_endpoint: "http://localhost:11434"  # Ollama 地址
  local_model_name: "qwen2.5-coder:7b"            # 推荐模型
  auto_fallback: true         # 云 LLM 不可达时自动降级到本地
  
  # 降级后的能力限制提示
  capability_warning: |
    当前使用本地模型 (qwen2.5-coder:7b)。
    以下能力在离线模式下受限:
    - 复杂多步推理质量可能降低
    - 上下文窗口限制更严格 (8K vs 200K tokens)
    - Runbook RAG 结果可能不够精确
```

### 层 3：离线安装包

```
ops-ai-offline-bundle/
├── ops-ai-linux-amd64          # 编译好的二进制
├── ops-ai-darwin-arm64
├── install.sh                   # 一键安装脚本
├── models/
│   └── qwen2.5-coder-7b-q4.gguf # 内置量化模型 (~4GB)
├── ollama-offline/
│   └── ollama-linux-amd64       # 离线版 Ollama
├── config.airgap.yaml           # 预配置（仅本地模型）
└── README.md                    # 离线部署指南
```

安装脚本：
```bash
#!/bin/bash
# install-airgap.sh
# 在离线环境中一键部署 ops-ai

echo "=== Ops-AI 离线安装 ==="

# 1. 安装二进制
cp ops-ai /usr/local/bin/ops-ai
chmod +x /usr/local/bin/ops-ai

# 2. 安装本地模型服务
if ! command -v ollama &>/dev/null; then
    tar -xzf ollama-offline/ollama-linux-amd64.tar.gz -C /usr/local/bin/
fi

# 3. 导入模型
ollama create qwen2.5-coder:7b -f models/Modelfile

# 4. 配置文件
mkdir -p ~/.ops-ai
cp config.airgap.yaml ~/.ops-ai/config.yaml

echo "安装完成。运行: ops-ai"
```

## 17.3 TUI 状态指示

```
┌─ ops-ai ─── ☁️ claude-sonnet ─── prod-cluster ─── ns: payment ─── 14:32:05 ─┐
│ 本轮: 5/20  │  tokens: 18.2K  │  💰 $0.04  │ 月: $12.30                   │
└───────────────────────────────────────────────────────────────────────────┘

┌─ ops-ai ─── 🖥️ qwen2.5-coder:7b (离线) ─── prod-cluster ─── ns: payment ─┐
│ 本轮: 3/20  │  tokens: 8.2K/32K  │  💰 $0.00  │ 能力: 受限               │
└───────────────────────────────────────────────────────────────────────────┘
```

## 17.4 离线模式的能力权衡

| 能力 | 云 LLM | 本地 7B 模型 | 影响 |
|------|--------|------------|------|
| K8s 基础操作 (L0-L1) | ✅ 完美 | ✅ 可用 | 查询类操作本地模型足够 |
| 复杂排障推理 (L0 诊断) | ✅ 优秀 | ⚠️ 可用但推理链可能不完整 | 降级但不阻断 |
| 写操作 (L2-L3) | ✅ 优秀 | ⚠️ 需更严格的确认 | 写操作本身就有安全网关兜底 |
| Runbook RAG | ✅ 精确匹配 | ⚠️ 语义匹配精度下降 | 仍可用但需用户验证 |
| 多步编排 | ✅ 完美 | ⚠️ 可能遗漏步骤 | 复杂编排建议用云 LLM |
| System Prompt 遵循 | ✅ 严格遵守 | ⚠️ 小模型可能偏离 | 安全网关不依赖 LLM，不受影响 |

**关键原则：安全网关不依赖 LLM 质量。** 即使本地 7B 模型产生了危险的 function_call，五级安全网关仍在 Go 代码层面拦截。离线模式降低的是推理质量，不是安全性。

---

# 第十八部分：外部工具连接配置（v1.3 新增）

v1.2 提到 Prometheus/Loki 集成但没有定义连接配置格式。

## 18.1 Prometheus 连接配置

```yaml
# config.yaml
observability:
  prometheus:
    endpoint: "https://prometheus.prod.internal:9090"
    # 认证方式（三选一，按优先级）
    bearer_token: "${PROMETHEUS_TOKEN}"      # Bearer Token（K8s SA token 常见）
    basic_auth:                               # 基本认证
      username: "admin"
      password: "${PROMETHEUS_PASSWORD}"
    tls:
      insecure_skip_verify: false            # 生产环境应设为 false
      ca_file: "/etc/ssl/certs/ca-bundle.crt" # 自定义 CA
      cert_file: ""                           # 客户端证书（mTLS）
      key_file: ""
    # 或从 kubeconfig 中自动发现（Phase 2）
    # auto_discover: true  # 从 kube-prometheus-stack Service 自动发现
```

## 18.2 Loki 连接配置

```yaml
observability:
  loki:
    endpoint: "https://loki.prod.internal:3100"
    bearer_token: "${LOKI_TOKEN}"
    tls:
      insecure_skip_verify: false
    default_limit: 100       # 默认返回行数
    default_since: "15m"     # 默认查询时间范围
```

## 18.3 自动发现（Phase 2）

当集群中部署了 kube-prometheus-stack 时，Agent 可自动发现：

```go
// 从 K8s Service 自动发现 Prometheus/Loki 端点
func (o *ObservabilityDiscovery) Discover(ctx ToolContext) (*ObsEndpoints, error) {
    endpoints := &ObsEndpoints{}
    
    // 查找 kube-prometheus-stack Service
    svcs, _ := ctx.K8sClient.CoreV1().Services("").List(context.Background(), metav1.ListOptions{
        LabelSelector: "app.kubernetes.io/name=prometheus,operated-prometheus=true",
    })
    if len(svcs.Items) > 0 {
        endpoints.Prometheus = fmt.Sprintf("http://%s.%s.svc:9090", svcs.Items[0].Name, svcs.Items[0].Namespace)
    }
    
    return endpoints, nil
}
```

---

# 第十九部分：--dry-run 全局预览模式（v1.4 新增）

这是建立用户信任的关键功能。代码助手的 dry-run 不重要（git diff 预览就够了），但运维操作没有预览就是盲飞。

## 19.1 核心行为

```
./ops-ai --dry-run "给 payment 扩容到 10 副本"

→ Agent 走完整推理链:
  1. kubectl get deployment/payment → 当前状态
  2. 影响面分析 → 受影响资源列表
  3. 安全分级 → L2（需 yes 确认）
  4. 输出完整操作计划:

  ╔══════════════════════════════════════════════════════╗
  ║        DRY RUN — 不会执行任何操作                      ║
  ╚══════════════════════════════════════════════════════╝
  
  操作计划:
  1. kubectl scale deployment/payment --replicas=10 (L2)
     影响面:
     ├─ 新增 5 个 Pod (当前 5 → 目标 10)
     ├─ HPA/payment-autoscaler ← 副本数达 max 上限
     └─ Service/payment ← 新 Pod 自动注册
  
     预计: 耗时 ~60s，无服务中断
     回滚: kubectl scale deployment/payment --replicas=5
  
  ✅ 此操作可在您的集群执行。确认无误后去掉 --dry-run 再执行。
```

## 19.2 实现

```go
// Agent Loop 中的 dry-run 分支
func (a *Agent) Run(input string) (<-chan AgentEvent, error) {
    if a.config.DryRun {
        // dry-run 模式: 完整规划但不执行
        return a.runDryRun(input)
    }
    return a.runNormal(input)
}

func (a *Agent) runDryRun(input string) (<-chan AgentEvent, error) {
    events := make(chan AgentEvent)
    
    go func() {
        defer close(events)
        
        // 仍然走完整的 LLM 推理 + 工具选择流程
        // 但所有 L1+ 工具调用的 Execute() 被替换为 DryRun()
        for event := range a.runWithInterceptor(input, dryRunInterceptor) {
            events <- event
        }
        
        events <- AgentEvent{
            Type: AgentEventDone,
            Content: "\n--- DRY RUN 完成。以上操作均未实际执行。---\n" +
                     "确认无误后，去掉 --dry-run 参数重新运行。\n",
        }
    }()
    
    return events, nil
}

// dryRunInterceptor: 拦截所有 L1+ 工具调用
func dryRunInterceptor(tool Tool, input ToolExecInput, ctx ToolContext) (*InterceptResult, error) {
    level := ctx.SafetyGate.ResolveLevel(tool, ctx)
    
    if level == RiskLevelReadOnly {
        // L0 操作仍然执行（kubectl get 等需要真实数据做分析）
        return &InterceptResult{Allowed: true}, nil
    }
    
    // L1+: 只展示计划，不执行
    dryRun, _ := tool.DryRun(input, ctx)
    return &InterceptResult{
        Allowed:      false, // 不执行
        Reason:       "DRY RUN mode — 操作已规划但未执行",
        DryRunResult: dryRun,
    }, nil
}
```

## 19.3 --dry-run 在 TUI 中的展示

```
┌─ ops-ai ─── 🧪 DRY RUN ─── prod-cluster ─── ns: payment ─── 14:32:05 ─┐
│ 本轮: 3/20  │  ⚠️ 预览模式 — 写操作仅展示计划，不会实际执行            │
└──────────────────────────────────────────────────────────────────────┘
```

Dry-run 模式下状态栏的 "🧪 DRY RUN" 标识始终可见，L1+ 操作的确认提示改为 `[DRY RUN — 仅预览]`。

---

# 第二十部分：RBAC 权限清单（v1.4 新增）

v1.3 的致命缺失：没有告诉运维 ops-ai 自身需要什么 ClusterRole。运维部署后第一天就会遇到 "Error: pods is forbidden"。

## 20.1 ReadOnly ClusterRole（L0 纯诊断用）

部署最小权限原则：如果团队只想用 ops-ai 做只读诊断，用这个 ClusterRole。

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ops-ai-readonly
  labels:
    app.kubernetes.io/name: ops-ai
    app.kubernetes.io/part-of: ops-ai
rules:
  # 核心资源 — 只读
  - apiGroups: [""]
    resources:
      - pods
      - pods/log
      - pods/status
      - pods/ephemeralcontainers
      - services
      - endpoints
      - configmaps
      - secrets          # Agent 会 redact values，但需要读取 key names
      - namespaces
      - nodes
      - nodes/status
      - events
      - serviceaccounts
      - persistentvolumeclaims
      - persistentvolumes
    verbs: ["get", "list", "watch"]
  
  # 工作负载 — 只读
  - apiGroups: ["apps"]
    resources:
      - deployments
      - deployments/status
      - statefulsets
      - statefulsets/status
      - daemonsets
      - daemonsets/status
      - replicasets
    verbs: ["get", "list", "watch"]
  
  # 批处理 — 只读
  - apiGroups: ["batch"]
    resources:
      - jobs
      - cronjobs
    verbs: ["get", "list", "watch"]
  
  # 网络 — 只读
  - apiGroups: ["networking.k8s.io"]
    resources:
      - ingresses
      - networkpolicies
    verbs: ["get", "list", "watch"]
  
  # 自动伸缩 — 只读
  - apiGroups: ["autoscaling"]
    resources:
      - horizontalpodautoscalers
    verbs: ["get", "list", "watch"]
  
  # 策略 — 只读
  - apiGroups: ["policy"]
    resources:
      - poddisruptionbudgets
    verbs: ["get", "list", "watch"]
  
  # RBAC — 只读（用于诊断权限问题）
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources:
      - roles
      - rolebindings
      - clusterroles
      - clusterrolebindings
    verbs: ["get", "list", "watch"]
  
  # 存储 — 只读
  - apiGroups: ["storage.k8s.io"]
    resources:
      - storageclasses
    verbs: ["get", "list", "watch"]
  
  # 准入控制 — 只读
  - apiGroups: ["admissionregistration.k8s.io"]
    resources:
      - validatingwebhookconfigurations
      - mutatingwebhookconfigurations
    verbs: ["get", "list", "watch"]
  
  # CRD 发现 — 只读
  - apiGroups: ["apiextensions.k8s.io"]
    resources:
      - customresourcedefinitions
    verbs: ["get", "list", "watch"]
  
  # 资源指标（kubectl top 需要）
  - apiGroups: ["metrics.k8s.io"]
    resources:
      - pods
      - nodes
    verbs: ["get", "list"]
  
  # 非资源 URL（healthz, version 等）
  - nonResourceURLs:
      - "/version"
      - "/healthz"
      - "/api"
      - "/api/*"
      - "/apis"
      - "/apis/*"
    verbs: ["get"]
```

## 20.2 ReadWrite ClusterRole（L0-L3 操作用）

如果团队需要 ops-ai 执行变更操作，用这个。**仅在生产集群上限定 namespace 的 RoleBinding 而非全局 ClusterRoleBinding**。

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ops-ai-readwrite
  labels:
    app.kubernetes.io/name: ops-ai
    app.kubernetes.io/part-of: ops-ai
rules:
  # 继承 ReadOnly 所有权限
  - apiGroups: [""]
    resources:
      - pods
      - pods/log
      - pods/status
      - pods/ephemeralcontainers
      - services
      - endpoints
      - configmaps
      - secrets
      - namespaces
      - nodes
      - nodes/status
      - events
      - serviceaccounts
      - persistentvolumeclaims
      - persistentvolumes
    verbs: ["get", "list", "watch"]
  
  # === 以下为 Write 权限 ===
  
  # Pod 操作
  - apiGroups: [""]
    resources:
      - pods
      - pods/eviction
      - pods/exec
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
  
  # ConfigMap/Secret 操作
  - apiGroups: [""]
    resources:
      - configmaps
      - secrets
    verbs: ["create", "update", "patch", "delete"]
  
  # Service 操作
  - apiGroups: [""]
    resources:
      - services
    verbs: ["create", "update", "patch", "delete"]
  
  # 节点操作（cordon/drain/uncordon）
  - apiGroups: [""]
    resources:
      - nodes
    verbs: ["update", "patch"]
  
  # 工作负载操作
  - apiGroups: ["apps"]
    resources:
      - deployments
      - deployments/scale
      - statefulsets
      - statefulsets/scale
      - daemonsets
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # 网络操作
  - apiGroups: ["networking.k8s.io"]
    resources:
      - ingresses
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # 自动伸缩操作
  - apiGroups: ["autoscaling"]
    resources:
      - horizontalpodautoscalers
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # 批处理操作
  - apiGroups: ["batch"]
    resources:
      - jobs
      - cronjobs
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # 策略操作（PDB 等）
  - apiGroups: ["policy"]
    resources:
      - poddisruptionbudgets
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # 非资源 URL
  - nonResourceURLs:
      - "/version"
      - "/healthz"
      - "/api"
      - "/api/*"
      - "/apis"
      - "/apis/*"
    verbs: ["get"]
```

## 20.3 绑定示例

```yaml
# 生产环境: namespace 级 RoleBinding（推荐！不要用 ClusterRoleBinding）
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ops-ai-readwrite-payment
  namespace: payment
subjects:
  - kind: ServiceAccount
    name: ops-ai-agent
    namespace: ops-ai
roleRef:
  kind: ClusterRole
  name: ops-ai-readwrite
  apiGroup: rbac.authorization.k8s.io
---
# 如需跨 namespace，为每个 namespace 创建一个 RoleBinding
# 绝对不要: ClusterRoleBinding + ops-ai-readwrite（这会让 ops-ai 操作所有 namespace）
```

## 20.4 部署命令

```bash
# 1. 创建 namespace 和 ServiceAccount
kubectl create namespace ops-ai
kubectl create serviceaccount ops-ai-agent -n ops-ai

# 2. 应用 ClusterRole（ReadOnly 或 ReadWrite，二选一）
kubectl apply -f ops-ai-readonly-clusterrole.yaml
# 或
kubectl apply -f ops-ai-readwrite-clusterrole.yaml

# 3. 绑定到需要的 namespace（可多次绑定）
kubectl create rolebinding ops-ai-rw-payment \
  --clusterrole=ops-ai-readwrite \
  --serviceaccount=ops-ai:ops-ai-agent \
  -n payment

# 4. 生成 ops-ai 使用的 kubeconfig（基于 SA token）
kubectl create token ops-ai-agent -n ops-ai --duration=8760h
# 或长期方案: 使用 SA 的 long-lived token secret

# 5. 验证权限
kubectl auth can-i get pods --as=system:serviceaccount:ops-ai:ops-ai-agent -n payment
```

## 20.5 安全建议

| 场景 | 推荐 ClusterRole | 绑定方式 |
|------|-----------------|---------|
| 开发集群（kind/minikube） | readwrite | ClusterRoleBinding（开发环境低风险） |
| Staging 集群 | readwrite | 按 namespace RoleBinding |
| 生产集群 — 诊断 | readonly | 按 namespace RoleBinding |
| 生产集群 — 值守 | readwrite | 仅限值守 namespace 的 RoleBinding。**绝不 ClusterRoleBinding** |
| CI/CD 中使用 | readonly | 仅限 CI namespace 的 RoleBinding |

**铁律**: 生产集群上 `ops-ai-readwrite` + `ClusterRoleBinding` = 从不。ops-ai 的服务账号永远不应该拥有全局写权限。

## 20.6 ops-ai 启动时的 RBAC 自检

```go
// 启动时自动执行权限自检
func (a *Agent) checkRBAC() error {
    checks := []struct {
        verb     string
        resource string
        msg      string
    }{
        {"get", "pods", "无法列出 Pod"},
        {"get", "events", "无法查看事件"},
        {"get", "nodes", "无法查看节点"},
    }
    
    warnings := []string{}
    for _, c := range checks {
        canDo, _ := a.k8sClient.AuthorizationV1().SelfSubjectAccessReviews().Create(
            context.Background(),
            &authv1.SelfSubjectAccessReview{
                Spec: authv1.SelfSubjectAccessReviewSpec{
                    ResourceAttributes: &authv1.ResourceAttributes{
                        Verb: c.verb, Resource: c.resource,
                    },
                },
            }, metav1.CreateOptions{})
        
        if !canDo.Status.Allowed {
            warnings = append(warnings, fmt.Sprintf("⚠️  %s (缺少 %s %s 权限)", c.msg, c.verb, c.resource))
        }
    }
    
    if len(warnings) > 0 {
        fmt.Println("RBAC 权限检查 — 以下操作可能受限:")
        for _, w := range warnings {
            fmt.Println(w)
        }
        fmt.Println("建议: kubectl apply -f ops-ai-readonly-clusterrole.yaml")
        fmt.Println("或: kubectl apply -f ops-ai-readwrite-clusterrole.yaml (如需写操作)")
    }
    
    return nil
}
```

---

# 第二十一部分：统一操作审计日志（v1.5 新增）

v1.4 的会话只存在本地 SQLite 里，多个运维的会话各自独立。生产环境必须有一个统一的审计日志——这是合规红线。没有审计日志，运维团队绝不敢在生产环境使用 ops-ai。

## 21.1 为什么 SQLite 不够

```
v1.4 现状:
  - 运维 A 的本地 SQLite: ~/.ops-ai/sessions.db
  - 运维 B 的本地 SQLite: ~/.ops-ai/sessions.db
  - 两人各自独立，无法交叉查询

凌晨 3 点发生 P0 事故 → 早上 9 点复盘:
  → "昨晚谁对 payment 做了什么操作？"
  → 是运维 A 手动执行的？还是 ops-ai 执行的？还是 CI/CD Pipeline？
  → 查不了——三个来源各管各的

合规要求:
  → SOC2 / ISO27001 / 等保 都要求"所有生产变更必须有可追溯的审计记录"
  → 审计记录必须:
    1. 不可篡改（append-only）
    2. 集中存储（不是各自本地 SQLite）
    3. 包含操作人、时间、资源、命令、结果、风险级别
    4. 可被外部系统消费（SIEM/Splunk/ELK）
```

## 21.2 审计日志格式

每条 L1+ 操作生成一条结构化审计记录（JSON Lines 格式）：

```json
{
  "version": "1.0",
  "timestamp": "2026-06-26T03:15:42.123Z",
  "session_id": "sess-abc123-def456",
  "operator": "zhangsan",
  "operator_id": "zhangsan@company.com",
  "cluster": "prod-us-east-1",
  "namespace": "payment",
  "environment": "production",
  
  "action": "kubectl rollout restart deployment/payment",
  "tool_name": "kubectl_rollout_restart",
  "risk_level": "L2",
  
  "target": {
    "kind": "Deployment",
    "name": "payment",
    "api_version": "apps/v1"
  },
  
  "pre_check": {
    "admission_webhooks": "passed",
    "pdb_check": "passed (1 replica headroom)",
    "psa_check": "not applicable",
    "operator_check": "no ownerReferences",
    "resource_quota": "passed",
    "namespace_confirmation": "confirmed (user typed 'production')"
  },
  
  "impact_analysis": {
    "affected_resources": ["Service/payment", "HPA/payment-autoscaler"],
    "estimated_duration": "30-60s",
    "service_interruption": false
  },
  
  "result": {
    "status": "success",
    "exit_code": 0,
    "duration_ms": 4500,
    "new_state": "deployment restarted, 4 pods rolling"
  },
  
  "rollback": {
    "snapshot_id": "snap-xyz789-20260626-031542",
    "rollback_command": "kubectl rollout undo deployment/payment"
  },
  
  "confirmation": {
    "method": "yes",
    "confirmed_by": "zhangsan",
    "confirmed_at": "2026-06-26T03:15:38.456Z"
  }
}
```

## 21.3 可配置 Sink（输出目标）

```yaml
# config.yaml
audit:
  enabled: true                    # 是否启用审计（生产环境强制开启）
  min_level: "L1"                  # 记录 L1+ 操作（L0 只读操作可选记录）
  
  sinks:                           # 支持多 sink 同时输出
    - type: file
      path: "/var/log/ops-ai/audit.jsonl"
      rotate:
        max_size_mb: 100           # 单文件最大 100MB
        max_files: 30              # 保留 30 个历史文件
        compress: true             # 旧文件 gzip 压缩
    
    - type: stdout
      format: "json"               # "json" | "text"
      enabled_on: ["staging", "dev"]  # 仅在非生产环境输出到 stdout
    
    - type: loki
      endpoint: "https://loki.prod.internal:3100"
      tenant_id: "ops-ai-audit"
      labels:
        app: "ops-ai"
        component: "audit"
      bearer_token: "${LOKI_AUDIT_TOKEN}"
    
    - type: elasticsearch
      endpoint: "https://elasticsearch.prod.internal:9200"
      index: "ops-ai-audit-{YYYY-MM-DD}"
      bearer_token: "${ES_AUDIT_TOKEN}"
    
    - type: webhook
      url: "https://siem.company.internal/ingest/ops-ai"
      headers:
        Authorization: "Bearer ${SIEM_TOKEN}"
      batch_size: 10               # 每 10 条批量发送
      flush_interval: "5s"         # 或每 5 秒刷新
```

## 21.4 实现接口

```go
// AuditLogger 审计日志接口
type AuditLogger interface {
    // Log 记录一条审计事件
    Log(event AuditEvent) error
    
    // Flush 刷新缓冲（用于优雅关闭）
    Flush() error
    
    // Close 关闭所有 sink
    Close() error
}

// AuditEvent 审计事件（对应 JSON 结构）
type AuditEvent struct {
    Version      string            `json:"version"`       // "1.0"
    Timestamp    time.Time         `json:"timestamp"`
    SessionID    string            `json:"session_id"`
    Operator     string            `json:"operator"`
    OperatorID   string            `json:"operator_id"`
    Cluster      string            `json:"cluster"`
    Namespace    string            `json:"namespace"`
    Environment  string            `json:"environment"`
    
    Action       string            `json:"action"`        // 人类可读描述
    ToolName     string            `json:"tool_name"`     // 内部工具名
    RiskLevel    string            `json:"risk_level"`    // L0/L1/L2/L3/L4
    
    Target       AuditTarget       `json:"target"`
    PreCheck     AuditPreCheck     `json:"pre_check"`
    Impact       AuditImpact       `json:"impact_analysis,omitempty"`
    Result       AuditResult       `json:"result"`
    Rollback     AuditRollback     `json:"rollback,omitempty"`
    Confirmation AuditConfirmation `json:"confirmation,omitempty"`
}

// MultiSinkLogger 支持多 sink 的审计日志器
type MultiSinkLogger struct {
    sinks   []AuditSink
    buffer  chan AuditEvent   // 异步缓冲通道
    done    chan struct{}
}

// AuditSink 单个输出目标接口
type AuditSink interface {
    Write(event AuditEvent) error
    Flush() error
    Close() error
}

// 异步写入，不阻塞 Agent Loop
func (m *MultiSinkLogger) Log(event AuditEvent) error {
    select {
    case m.buffer <- event:
        return nil
    default:
        // 缓冲区满 → 记录到本地 fallback
        return m.fallbackLog(event)
    }
}

// fallbackLog 当所有远程 sink 不可达时的兜底
func (m *MultiSinkLogger) fallbackLog(event AuditEvent) error {
    // 始终写入本地文件作为最后兜底
    f, _ := os.OpenFile(
        filepath.Join(m.config.DataDir, "audit-fallback.jsonl"),
        os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600,
    )
    defer f.Close()
    data, _ := json.Marshal(event)
    f.Write(append(data, '\n'))
    return nil
}
```

## 21.5 Agent Loop 中的审计集成点

```
Agent 执行 L1+ 操作时:
  
  1. 安全网关通过 → 进入执行
  2. 构建 AuditEvent（填充 action, target, pre_check, impact 等）
  3. 执行操作
  4. 填充 AuditEvent.result（status, exit_code, duration, new_state）
  5. 如果 L3 操作 → 填充 AuditEvent.rollback
  6. 调用 auditLogger.Log(event)  ← 异步，不阻塞 Agent Loop
  7. Agent Loop 继续下一轮

Agent 启动时:
  1. 解析 config.yaml 中的 audit 配置
  2. 初始化所有 sink
  3. 如果审计启用但所有 sink 初始化失败 → 输出警告但继续运行
     （运维能力不应因审计组件故障而完全阻断）
  
Agent 关闭时:
  1. auditLogger.Flush()  ← 确保缓冲中的事件全部写出
  2. auditLogger.Close()  ← 关闭所有 sink 连接
```

## 21.6 TUI 审计状态指示

```
启动自检:
  ✅ 审计日志: file:///var/log/ops-ai/audit.jsonl + loki://loki.prod.internal:3100

状态栏（L1+ 操作后短暂闪烁）:
  ┌─ ops-ai ─── prod-cluster ─── ns: payment ─── 📝 已记录 ─── 14:32:05 ─┐
```

## 21.7 审计日志查询（Phase 2）

MVP 阶段审计日志由外部系统（Loki/ELK/SIEM）查询。Phase 2 提供内置查询能力：

```bash
# 查询某时间段内的操作
ops-ai audit --from "2026-06-26 00:00" --to "2026-06-26 12:00"

# 查询某用户的操���
ops-ai audit --operator zhangsan --last 24h

# 查询对某资源的所有操作
ops-ai audit --resource deployment/payment --namespace production

# 导出合规报告
  ops-ai audit --report --format pdf --period "2026-06"
```

---

# 第二十二部分：CI/CD 无交互模式（v1.5 新增）

v1.4 所有 L1+ 操作都需要人在终端确认。但 CI/CD pipeline 里没人能按 Enter——这是企业采用的关键阻断点。如果 ops-ai 只能在终端交互使用，企业的 CI/CD pipeline 完全无法集成。

## 22.1 两个新模式

```
CI 模式 (--no-tui --yes):
  ./ops-ai --no-tui --yes "检查 payment 服务是否部署成功"
  → 纯文本输出（无 TUI）
  → L1-L2 自动确认（CI 环境已预授权）
  → L3 操作仍然拒绝（CI 不应有删除权限）

管道模式 (--pipe):
  echo "payment 有哪些异常 Pod？" | ./ops-ai --pipe | grep CrashLoopBackOff
  → 纯文本输出，可被 jq/grep/awk 消费
  → L1 自动确认
  → L2+ 直接拒绝（管道模式下不允许修改）
```

## 22.2 三级自动化确认

| 操作级别 | CLI 交互模式 | --no-tui --yes | --pipe |
|---------|------------|----------------|--------|
| L0 | 自动执行 | 自动执行 | 自动执行 |
| L1 | Enter 确认 | 自动确认 ✅ | 自动确认 ✅ |
| L2 | yes 确认 | 自动确认 ✅ | 拒绝 ❌ |
| L3 | 集群名确认 | 拒绝 ❌ | 拒绝 ❌ |
| L4 | 拒绝 ❌ | 拒绝 ❌ | 拒绝 ❌ |

**设计原则**: `--yes` 是"信任这个环境中的操作员"，不是"信任 AI"。CI/CD 环境中运行 ops-ai 说明 pipeline 已经通过了代码审查。`--pipe` 更保守——只读消费，禁止修改。

## 22.3 CI/CD 使用场景

### 场景 A：部署后健康检查

```yaml
# GitHub Actions
name: Post-Deploy Health Check
on:
  workflow_run:
    workflows: ["Deploy to Production"]
    types: [completed]

jobs:
  health-check:
    runs-on: [self-hosted, ops-runner]
    steps:
      - name: Verify payment service health
        run: |
          ./ops-ai --no-tui --yes \
            --kubeconfig /etc/ops-ai/prod-kubeconfig.yaml \
            "检查 payment deployment 状态:
             1. 所有 Pod 是否 Running
             2. 最近 5 分钟是否有 CrashLoopBackOff
             3. HPA 是否正常工作
             如果有异常，exit 1"
      
      - name: Verify config drift
        run: |
          ./ops-ai --no-tui --yes "检查是否有 Config Drift" || {
            echo "::warning:: 检测到配置漂移，请人为检查"
          }
      
      - name: Cluster health snapshot
        run: ./ops-ai --overview --pipe > /tmp/cluster-health-$(date +%Y%m%d-%H%M).txt
```

### 场景 B：定期自动巡检

```yaml
# Kubernetes CronJob
apiVersion: batch/v1
kind: CronJob
metadata:
  name: ops-ai-health-check
  namespace: ops-ai
spec:
  schedule: "*/30 * * * *"    # 每 30 分钟
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: ops-ai-readonly  # 只用只读 SA
          containers:
          - name: ops-ai
            image: ops-ai:latest
            command:
              - /usr/local/bin/ops-ai
              - --no-tui
              - --yes
              - "集群健康检查: 报告异常 Pod、节点状态、资源配额"
          restartPolicy: OnFailure
```

### 场景 C：变更窗口自动验证

```bash
#!/bin/bash
# deploy-and-verify.sh — 变更窗口脚本

set -e

echo "=== 变更前快照 ==="
./ops-ai --overview --pipe > /tmp/pre-change-health.txt

echo "=== 执行变更 ==="
kubectl set image deployment/payment payment=registry.io/payment:v2.4.0

echo "=== 等待 Pod Ready ==="
kubectl wait --for=condition=Ready pod -l app=payment --timeout=300s

echo "=== 变更后验证 ==="
if ./ops-ai --no-tui --yes "检查 payment 是否正常运行"; then
    echo "✅ 变更验证通过"
    cat /tmp/post-change-health.txt
else
    echo "❌ 变更验证失败，触发回滚"
    kubectl rollout undo deployment/payment
    # 发送告警: curl -X POST $PAGERDUTY_WEBHOOK ...
    exit 1
fi
```

## 22.4 输出格式

### --no-tui 模式输出

```
$ ./ops-ai --no-tui --yes "payment 状态"
[14:32:05] 🟢 kubectl get deployment/payment → 4/4 Ready
[14:32:06] 🟢 kubectl get pods -l app=payment → 4 Running, 0 restarts
[14:32:08] 🟢 kubectl top pods -l app=payment → CPU avg 35%, Mem avg 420Mi
[14:32:08] ✅ 服务正常: 4 副本 Running, 无异常重启, 资源使用在范围内
```

### --pipe 模式输出（纯文本，可被消费）

```
$ echo "异常 Pod" | ./ops-ai --pipe
payment-debug-7d8f9    CrashLoopBackOff   restart=47   node-3
report-worker-5c3a1    OOMKilled          restart=3    node-7
notification-cache-f2b8d ImagePullBackOff  restart=12   node-5
```

## 22.5 退出码语义

CI/CD 需要明确的退出码来判断 pipeline 成败：

| 退出码 | 含义 | 触发条件 |
|--------|------|---------|
| 0 | 成功 | 所有操作成功，无异常 |
| 1 | 业务异常 | 检测到需要关注的异常（CrashLoopBackOff、OOMKilled 等） |
| 2 | 工具错误 | Agent 自身错误（K8s API 不可达、LLM 超时） |
| 3 | 安全拒绝 | L3/L4 操作被拒绝（CI 不应执行高风险操作） |
| 4 | 配置错误 | RBAC 权限不足、kubeconfig 无效 |

## 22.6 CI/CD 配置

```yaml
# config.yaml
ci_mode:
  # --no-tui --yes 时的行为
  auto_confirm_max_level: "L2"     # 自动确认的上限
  reject_on_failure: true          # 任何操作失败时 exit 1
  timeout: "120s"                  # CI 模式的总超时
  
  # --pipe 模式的行为
  pipe_output_format: "tsv"        # "tsv" | "json" | "csv" | "plain"
  pipe_max_rows: 1000              # 最多输出行数
```

## 22.7 Agent Loop 适配（non-TUI 模式）

```go
// Agent 检测运行模式
func (a *Agent) Run(input string, mode RunMode) (<-chan AgentEvent, error) {
    switch mode {
    case RunModeInteractive:
        return a.runInteractive(input)  // 标准 TUI 交互模式
    case RunModeCI:
        return a.runCI(input)           // --no-tui --yes
    case RunModePipe:
        return a.runPipe(input)         // --pipe
    }
}

func (a *Agent) runCI(input string) (<-chan AgentEvent, error) {
    events := make(chan AgentEvent)
    
    go func() {
        defer close(events)
        
        for event := range a.runWithInterceptor(input, ciInterceptor) {
            // 纯文本输出，不渲染 TUI 组件
            events <- AgentEvent{
                Type:    AgentEventText,
                Content: formatTextOnly(event),
            }
        }
        
        // 最终退出码
        events <- AgentEvent{
            Type:    AgentEventExit,
            ExitCode: determineExitCode(a.lastResults),
        }
    }()
    
    return events, nil
}

// ciInterceptor: 在 CI 模式下拦截确认请求
func ciInterceptor(tool Tool, input ToolExecInput, ctx ToolContext) (*InterceptResult, error) {
    level := ctx.SafetyGate.ResolveLevel(tool, ctx)
    
    switch level {
    case RiskLevelReadOnly, RiskLevelLow, RiskLevelMedium:
        // L0-L2: 自动确认
        return &InterceptResult{Allowed: true}, nil
    case RiskLevelHigh, RiskLevelForbidden:
        // L3-L4: CI 模式下拒绝
        return &InterceptResult{
            Allowed: false,
            Reason:  fmt.Sprintf("CI 模式不允许执行 L%d 操作: %s", level, tool.Name()),
        }, nil
    }
    
    return &InterceptResult{Allowed: false}, nil
}
```

---

# 第二十三部分：会话共享与协作机制（v1.6 新增）

v1.5 的 CI/CD 模式解决了"机器调用 ops-ai"，但没解决"两个运维共同排查"。真实场景下，on-call 故障排查天然需要接力——初级运维先上，卡住后 escalate 给 senior。如果 senior 看不到初级运维的会话上下文，只能从零开始重新排查，浪费宝贵的 MTTR 时间。

## 23.1 问题场景

```
典型 on-call 接力场景:

03:15  告警: payment-p99-latency > 2s
03:16  初级运维 zhangsan 打开 ops-ai，连查 8 轮:
       kubectl get pods → 正常
       kubectl top pods → CPU 正常
       kubectl describe deployment/payment → 最近 events 无异常
       kubectl logs --tail=200 → 大量 "slow query: 4.2s"
       kubectl get configmap payment-db → DB 连接池配置 50
       kubectl get hpa payment → max 10，当前 8
       kubectl get events --sort-by='.lastTimestamp' → 最近 10 min 无异常事件
       kubectl describe service/payment → Endpoints 正常
       → 怀疑 DB 连接池瓶颈，但不确定根因

03:28  卡住了，向 senior lisi 求助
       zhangsan: /share → 生成分享链接/文件
       lisi 打开 → 看到 zhangsan 的 8 轮排查上下文
       → 基于已有分析直接深入: "检查 RDS 慢查询日志"
       → 5 分钟内定位根因: DB max_connections 达上限

v1.5 行为: lisi 打开 ops-ai 看到的是空白对话，所有 zhangsan 的排查
          上下文完全丢失。lisi 需要从零开始问同样的问题——MTTR +15min。
```

## 23.2 MVP 方案：会话导出/导入（文件级共享）

跨机器共享最轻量的方式——导出为自包含文件，通过 IM/邮件传递。

### 23.2.1 导出命令

```
/save [filename]    — 保存当前会话到本地文件（默认 ~/.ops-ai/sessions/<session-id>.json）
/export [filename]  — 导出为可共享的自包含文件（含完整对话 + 工具调用结果 + 快照引用）
/share              — 生成分享摘要，含会话 ID + 排查路径 + 关键发现
```

### 导出文件格式

```json
{
  "version": "2.0",
  "session_id": "sess-abc123-def456",
  "exported_at": "2026-06-26T03:28:15Z",
  "exported_by": "zhangsan",
  "cluster": "prod-us-east-1",
  "namespace": "payment",
  "context": {
    "environment": "production",
    "critical_services": ["payment"]
  },
  
  "summary": {
    "intent": "排查 payment p99 延迟异常",
    "key_findings": [
      "Pod 和节点 CPU/内存正常",
      "日志显示大量 slow query (4.2s)",
      "DB 连接池配置 50，可能不足",
      "HPA 正常，8/10 副本 Running"
    ],
    "pending_questions": [
      "RDS max_connections 是否已达上限？",
      "是否有其他服务同时大量连接 DB？"
    ],
    "tools_executed": 8,
    "total_duration": "12m"
  },
  
  "conversation": [
    {
      "role": "user",
      "content": "payment p99 延迟突然飙到 2s，帮我排查"
    },
    {
      "role": "assistant",
      "content": "收到。让我先看一下 payment 的基本状态。"
    },
    {
      "role": "tool",
      "tool_name": "kubectl_get_pods",
      "result": {
        "success": true,
        "output": "NAME                    READY  STATUS   RESTARTS  AGE\npayment-7d8f9-abc12  1/1   Running  0        3h\n...",
        "is_truncated": false
      }
    }
    // ... 完整对话历史
  ],
  
  "tool_call_history": [
    {
      "id": "call-001",
      "tool": "kubectl_get_pods",
      "risk_level": "L0",
      "timestamp": "2026-06-26T03:16:42Z",
      "duration_ms": 230
    }
    // ... 所有工具调用记录
  ],
  
  "snapshots": [
    {
      "id": "snap-xyz789",
      "resource": "deployment/payment",
      "hash": "sha256:abc123..."
    }
  ]
}
```

### 23.2.2 导入命令

```
/import <file>  — 导入之前导出的会话文件，恢复完整上下文
```

导入后的行为：

```go
// SessionImporter 会话导入器
type SessionImporter struct {
    agent *Agent
}

func (si *SessionImporter) Import(filePath string) (*Session, error) {
    data, _ := os.ReadFile(filePath)
    var export SessionExport
    json.Unmarshal(data, &export)
    
    // 1. 验证导出文件完整性
    if err := si.validate(export); err != nil {
        return nil, fmt.Errorf("无效的会话导出文件: %w", err)
    }
    
    // 2. 重建会话上下文
    session := si.agent.NewSession()
    session.ID = uuid.New().String()  // 新会话 ID（不覆盖原会话）
    session.ParentSessionID = export.SessionID  // 追溯来源
    
    // 3. 注入摘要到 System Prompt
    session.SystemPrompt += fmt.Sprintf(`
=== IMPORTED SESSION CONTEXT ===
This session was imported from a previous investigation by %s.
Original session: %s
Cluster: %s, Namespace: %s

Summary of previous investigation:
%s

Key findings so far:
%s

Pending questions to answer:
%s

The conversation history below contains the full context. Build on it
rather than starting from scratch.
=== END IMPORTED CONTEXT ===
`, export.ExportedBy, export.SessionID,
   export.Cluster, export.Namespace,
   formatSummary(export.Summary),
   formatFindings(export.Summary.KeyFindings),
   formatQuestions(export.Summary.PendingQuestions))
    
    // 4. 加载对话历史（保留最近 5 轮完整，更早的摘要化）
    session.Conversation = si.loadConversationWithBudget(export.Conversation, 15000)
    
    return session, nil
}
```

### 导入时的 TUI 展示

```
$ ./ops-ai --import payment-investigation-20260626.opsai

═══ 会话导入 ═══

  来源: zhangsan @ 2026-06-26 03:28
  集群: prod-us-east-1
  命名空间: payment
  原有排查: 8 个工具调用, 耗时 12 分钟
  
  关键发现:
  ├─ Pod 和节点资源正常
  ├─ 日志大量 slow query (4.2s)
  ├─ DB 连接池配置 50
  └─ 待确认: RDS max_connections 是否达上限
  
  已恢复 8 轮对话上下文。你可以直接基于以上发现继续排查。
  输入 /summary 随时查看排查摘要。
  
> _
```

## 23.3 跨会话上下文追溯

导入的会话标记 `ParentSessionID`，形成排查链路：

```
初级运维会话 (sess-001)
  └─ /export → 导出文件
       └─ senior 导入 → 新会话 (sess-002, ParentSessionID=sess-001)
            └─ 审计日志中可追溯: sess-002 的操作基于 sess-001 的排查
```

审计日志中体现：
```json
{
  "session_id": "sess-002",
  "parent_session_id": "sess-001",
  "imported_from": "zhangsan",
  "action": "kubectl scale deployment/payment --replicas=12"
}
```

这让事故复盘时可以完整追溯"谁先发现了什么 → 谁做了什么决策 → 基于什么上下文"。

## 23.4 实时协作模式（Phase 2）

MVP 的文件级共享已经覆盖了 80% 的 on-call 接力场景。Phase 2 的实时协作更适合"两个运维同时在线"的场景。

```
Phase 2 方案 — 共享会话服务器:

架构:
  运维 A (ops-ai) ── WebSocket ──┐
                                  ├── Session Server (轻量 Go 进程)
  运维 B (ops-ai) ── WebSocket ──┘       │
                                     SQLite (会话状态)
                                     
特性:
  - /collab start  → 启动协作模式，生成 join code
  - /collab join <code>  → 加入已有会话
  - 双方都能看到 LLM 推理输出
  - 只有"会话所有者"能确认 L2+ 操作（防两个人同时点 yes）
  - 双方都能输入自然语言
  - TUI 状态栏显示: 👤 zhangsan + 👤 lisi
  
安全约束:
  - join code 一次性使用，30 分钟过期
  - 协作者的操作等同于自己的操作（共用审计 Operator）
  - L3 操作在协作模式下强制双人审批（一个提议，另一个确认）
```

## 23.5 配置

```yaml
# config.yaml
session_sharing:
  export_path: "~/.ops-ai/sessions/"  # 导出文件存储路径
  export_format: "json"               # "json" | "json.gz" (压缩)
  auto_export_on_close: true          # 会话关闭时自动导出
  export_include_snapshots: false     # 是否包含快照数据（文件可能很大）
  max_export_size_mb: 50              # 单次导出最大文件大小
  
  collaboration:                      # Phase 2
    enabled: false
    server_port: 9721                 # 协作服务器端口
    join_code_ttl: "30m"             # join code 有效期
    max_collaborators: 3              # 最多协作人数
```

---

# 第二十四部分：外部事件驱动的 Agent 触发（v1.6 新增）

v1.5 的 CI/CD 模式解决了 pipeline 调用 ops-ai，但运维最核心的"告警 → 自动排查"链路完全缺失。一个告警响了，ops-ai 应该能自动开始诊断——而不是等运维睡醒打开电脑。

## 24.1 问题场景

```
场景 A — 凌晨告警自动排查:

03:15  AlertManager 触发: payment-p99-latency > 2s (持续 5m)
03:15  AlertManager webhook → ops-ai alert receiver
03:15  ops-ai 自动启动: --alert "payment-p99-latency" 
       "payment 服务 p99 延迟超过 2 秒已持续 5 分钟，帮我排查"
03:16  ops-ai 自动执行 L0 诊断:
       ├─ kubectl get pods -l app=payment → 8/8 Running
       ├─ kubectl top pods -l app=payment → CPU avg 42%, Mem avg 380Mi
       ├─ kubectl describe hpa payment → 当前 8/10, 正常
       ├─ kubectl logs --tail=100 -l app=payment | grep -i error → 3 条 slow query
       └─ kubectl get events --sort-by='.lastTimestamp' | head -20
03:17  ops-ai 输出诊断摘要到文件:
       /var/log/ops-ai/alerts/payment-p99-latency-20260626-0315.md
03:17  ops-ai 将摘要通过 webhook 回传给 AlertManager/PagerDuty:
       "排查完成: Pod 正常，发现 slow query 日志。
        初步判定为 DB 层问题。建议人工检查 RDS max_connections。
        详细报告: /var/log/ops-ai/alerts/..."
03:18  PagerDuty 告警 note 自动更新 → on-call 运维被叫醒时
       看到的不是"p99 高"，而是"p99 高 + AI 已初步排查 + 可能是 DB 问题"

场景 B — 告警自动扩容:

04:00  AlertManager 触发: node-memory-pressure (node-7, 92%)
04:00  ops-ai --alert "node-7 memory pressure"
       → L0 诊断: kubectl top node node-7, kubectl get pods --field-selector spec.nodeName=node-7
       → 诊断结果: 节点上 payment-pod-3 内存异常增长 (800Mi → 1.8Gi 在 20 分钟内)
       → L2 操作: kubectl cordon node-7 (需 --auto-remediate 明确开启)
       → 输出建议: "建议驱逐节点并关注 payment-pod-3 内存泄漏"
```

## 24.2 告警接收器架构

```
                         ┌──────────────┐
    AlertManager ───────→│              │
    PagerDuty    ───────→│  ops-ai      │──→ 诊断报告文件
    Grafana      ───────→│  alertd      │──→ AlertManager note 回写
    自定义 Webhook ─────→│  (守护进程)   │──→ Slack/钉钉/飞书通知
                         └──────┬───────┘
                                │
                         启动 ops-ai --alert
                                │
                         ┌──────┴───────┐
                         │  Agent Loop  │
                         │  (L0-only    │
                         │   by default)│
                         └──────────────┘
```

### alertd 守护进程

```go
// alertd 是 ops-ai 的告警接收守护进程（常驻后台）
type AlertDaemon struct {
    config    AlertDaemonConfig
    server    *http.Server           // HTTP webhook 接收
    router    *AlertRouter           // 告警路由规则引擎
    runner    *AlertRunner           // 启动 ops-ai --alert 子进程
    notifier  *AlertNotifier         // 结果通知（回写/IM/邮件）
}

type AlertDaemonConfig struct {
    ListenAddr    string              // ":9720"
    MaxConcurrent int                 // 同时处理的告警数 (default: 3)
    DefaultMode   AlertRunMode        // "diagnose" | "diagnose+remediate"
    Timeout       time.Duration       // 单个告警处理超时 (default: 5m)
}
```

## 24.3 告警 Webhook 接口

```go
// POST /v1/alerts — 接收外部告警
type AlertWebhookRequest struct {
    // AlertManager 格式（自动识别）
    AlertManager *AlertManagerPayload `json:"-"` // 通过 Content-Type/格式自动识别
    
    // 通用格式
    Source      string            `json:"source"`      // "alertmanager" | "pagerduty" | "grafana" | "custom"
    AlertName   string            `json:"alert_name"`   // "payment-p99-latency"
    Severity    string            `json:"severity"`     // "critical" | "warning" | "info"
    Description string            `json:"description"`  // 告警描述
    Labels      map[string]string `json:"labels"`       // 标签 (cluster, namespace, service...)
    StartedAt   time.Time         `json:"started_at"`
    SourceURL   string            `json:"source_url"`   // 告警源 URL（回写 note 用）
}

type AlertWebhookResponse struct {
    Accepted    bool   `json:"accepted"`
    InvestigationID string `json:"investigation_id"`  // 排查会话 ID
    Message     string `json:"message"`
}
```

### AlertManager 集成配置

```yaml
# alertmanager.yaml
receivers:
  - name: 'ops-ai'
    webhook_configs:
      - url: 'http://ops-ai-alertd:9720/v1/alerts'
        send_resolved: true
        http_config:
          authorization:
            credentials: "${OPS_AI_ALERT_TOKEN}"
```

## 24.4 告警路由规则

不是所有告警都需要 ops-ai 处理。配置规则引擎：

```yaml
# config.yaml
alert_rules:
  # 全局默认
  default_mode: "diagnose"         # "diagnose" (只读排查) | "diagnose+remediate" (可自动修复) | "ignore"
  default_max_level: "L0"          # 自动执行的操作上限
  
  # 按告警名匹配
  rules:
    - match:
        alert_name: "*-p99-latency"
        severity: "critical"
      mode: "diagnose"
      prompt: |
        服务 {{.Labels.service}} p99 延迟超过 {{.Labels.threshold}}，
        已持续 {{.Duration}}。请排查:
        1. Pod 资源使用和异常重启
        2. 下游依赖（DB/Cache/API）延迟
        3. 最近的部署变更
        
    - match:
        alert_name: "*CrashLoopBackOff*"
      mode: "diagnose"
      prompt: |
        Pod {{.Labels.pod}} 处于 CrashLoopBackOff 状态。
        请排查:
        1. kubectl logs --previous 看上一次崩溃日志
        2. kubectl describe pod 看 events
        3. 检查资源限制是否过低
        
    - match:
        alert_name: "*DiskPressure*"
      mode: "diagnose"
      prompt: |
        节点 {{.Labels.node}} 磁盘压力告警。
        排查磁盘使用、镜像缓存、日志大小。
        
    - match:
        alert_name: "DeadMansSwitch"
      mode: "ignore"                # 心跳告警不触发排查
```

## 24.5 自动修复策略（diagnose+remediate 模式）

**默认关闭。** 需要显式在每个规则中开启，且限定操作级别。

```yaml
    # 仅用于已知、可安全自动修复的场景
    - match:
        alert_name: "*HPA*MaxReplicas*"
        severity: "warning"
      mode: "diagnose+remediate"
      max_level: "L1"               # 最多自动到 L1（scale 扩容）
      prompt: |
        HPA {{.Labels.hpa}} 已达 maxReplicas 上限。
        如果当前 CPU/内存正常且集群资源充足，可以适度提高 maxReplicas。
```

### 自动修复的安全护栏

```
diagnose+remediate 模式下的硬约束:

1. 最高自动批准级别: max_level (default: "L1")
   - "L0": 只诊断，不修改任何东西
   - "L1": 可自动执行 L1 操作（scale, label 等）
   - "L2": 极其谨慎，仅限明确白名单场景

2. 白名单操作（diagnose+remediate + L2 时仍需确认）:
   kubectl scale deployment/* --replicas=N  (仅在 HPA 场景)
   → 约束: 新 replicas 不能超过 HPA max + 20%

3. 绝不自动执行: L3（删除）和 L4（禁区），无论规则如何配置

4. 所有自动操作仍然写入审计日志（operator="ops-ai/alertd"）

5. 自动操作前仍然执行所有 pre-flight 检查
   - 如果 pre-flight 发现 PDB 阻塞/配额超限 → 降级为 diagnose 模式
```

## 24.6 排查结果通知

```go
// AlertNotifier 将排查结果回传给告警系统
type AlertNotifier struct {
    sinks []NotifierSink
}

type NotifierSink interface {
    Notify(result AlertInvestigationResult) error
}

// AlertManager 回写（通过 AlertManager API 给告警添加 note）
type AlertManagerSink struct {
    endpoint string
}

func (s *AlertManagerSink) Notify(result AlertInvestigationResult) error {
    // POST /api/v2/alerts/<fingerprint>/note
    note := formatInvestigationNote(result)
    return s.sendNote(result.AlertFingerprint, note)
}

// Slack/钉钉/飞书通知
type ChatSink struct {
    webhookURL string
    template   string  // Go template
}
```

### 通知格式示例

```
═══ ops-ai 自动排查结果 ═══
告警: payment-p99-latency > 2s
时间: 2026-06-26 03:15 → 03:17 (耗时 1.8s)
结论: 初步判定为 DB 层瓶颈

排查路径:
  ✅ Pod 状态: 8/8 Running, CPU 42%, Mem 380Mi
  ✅ HPA: 当前 8/10, 正常
  ⚠️  日志: 检出 3 条 slow query (4.2s avg)
  ⚠️  DB 连接池: 配置 50，可能不足

待人工确认:
  1. RDS max_connections 是否达上限？
  2. 是否有其他服务短时间大量建立 DB 连接？

详细报告: /var/log/ops-ai/alerts/payment-p99-20260626-0315.md
```

## 24.7 守护进程生命周期

```bash
# 启动 alertd 守护进程
./ops-ai alertd --config /etc/ops-ai/alertd.yaml

# systemd unit
# /etc/systemd/system/ops-ai-alertd.service
[Unit]
Description=Ops-AI Alert Receiver Daemon
After=network.target

[Service]
Type=simple
User=ops-ai
ExecStart=/usr/local/bin/ops-ai alertd --config /etc/ops-ai/alertd.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

---

# 第二十五部分：非K8s基础设施诊断（v1.6 新增）

v1.5 把 Terraform 集成扔到 Phase 2（第 8 周以后），但运维 40% 以上的排障工作涉及云资源而非 K8s。如果 ops-ai 只能诊断 K8s 层面的问题，运维排查到一半就要切出去开 AWS Console——这不是"一个工具替代 5 个 Dashboard"，而是"两个工具加起来替代 5 个 Dashboard"。

## 25.1 问题场景

```
场景 A — RDS 慢查询导致 K8s 服务超时:

  用户: "payment 超时了，帮我排查"
  Agent: kubectl get pods → Running
          kubectl logs → 大量 "dial tcp 10.0.1.50:5432: i/o timeout"
          → 判断: 不是 K8s 的问题，是 DB 连接超时
  v1.5 行为: "这可能是 RDS 的问题。建议您登录 AWS Console 检查 RDS 状态。"
  → 用户: "所以你还是让我去开 AWS Console？"
  
  v1.6 行为:
  Agent: kubectl logs → 发现 DB 连接超时
          → ops-ai cloud rds describe db-payment-prod (L0, 自动执行)
          → "RDS db-payment-prod 当前状态:
             CPU: 92%, 连接数: 198/200 (max_connections 接近上限!)
             最近 1h 慢查询: 47 条 (avg 3.8s)
             
             根因: RDS 连接数接近上限，导致新连接排队超时。
             建议: 1. 检查是否有连接泄漏 2. 临时提高 max_connections 3. 考虑读副本分流"

场景 B — ALB 健康检查失败:

  用户: "payment 突然 502 了"
  Agent: kubectl get pods → 正常
          kubectl get svc → 正常
          kubectl get ingress → 正常
          → ops-ai cloud alb describe payment-alb
          → "ALB payment-alb: 3/4 目标组健康, 1 个 Unhealthy
             Unhealthy 目标: 10.0.2.15:8080 (node-4)
             
             检查该节点: kubectl get node node-4 → NotReady!
             根因: node-4 NotReady → ALB 健康检查失败 → 503"

场景 C — S3 权限问题:

  用户: "report-worker 报 403 Access Denied"
  Agent: kubectl logs report-worker-xxx
          → "AccessDenied: User arn:aws:iam::123456:role/report-worker-role 
             is not authorized to perform s3:GetObject on bucket report-data"
          → ops-ai cloud iam get-role report-worker-role
          → "IAM Role report-worker-role: 缺少 s3:GetObject 权限（仅 s3:ListBucket）"
          → 不自动修改 IAM（L4 禁区），但给出精确的修复策略
```

## 25.2 MVP 覆盖范围（Phase 1 末尾，不增加总工期）

MVP 只做 **L0 只读诊断**。如果云资源诊断能显著提高 K8s 排障效率，就不该等到 Phase 2——因为 40% 的 K8s 故障根因在云资源层。

| 云服务 | MVP (L0 只读) | Phase 2 (L1-L2 修改) |
|--------|--------------|---------------------|
| **RDS** | 实例状态、CPU/连接数/IOPS、慢查询日志、参数组配置 | 修改参数组、创建读副本、重启实例 |
| **ALB/NLB** | 监听器/目标组状态、健康检查结果、访问日志摘要 | 修改目标组权重、注册/注销目标 |
| **ElastiCache** | 集群状态、CPU/连接数/命中率 | 修改参数组、扩缩容 |
| **S3** | Bucket 策略、对象列表、访问日志 | 修改 Bucket 策略（L3） |
| **Route53** | 记录集、健康检查状态、DNS 解析测试 | 修改记录（L2） |
| **CloudFront** | 分发状态、缓存命中率、错误率 | 创建/修改失效（L2） |
| **IAM** | Role/Policy 详情、权限边界 | **绝不修改**（L4） |
| **EC2** | 实例状态、CloudWatch 指标 | 重启/停止（L2） |
| **EKS** | 集群状态、Node Group 状态、kubeconfig | 修改 Node Group（L3） |

### MVP 的前提条件

```
云资源诊断 MVP 的前提:
  1. 用户已配置云 provider 凭证（AWS_ACCESS_KEY_ID 环境变量或 IAM Role）
  2. 用户通过 config.yaml 显式声明要管理的资源
  3. ops-ai 仅使用只读 IAM 权限（arn:aws:iam::aws:policy/ReadOnlyAccess）
```

## 25.3 云 Provider 抽象接口

设计为 Provider 模式，便于扩展到 Azure / GCP / 阿里云 / 腾讯云：

```go
// CloudProvider 云平台抽象接口（MVP 只实现 AWS）
type CloudProvider interface {
    // 平台标识
    Name() string  // "aws" | "azure" | "gcp" | "aliyun"
    
    // 服务列表
    Services() []CloudService
    
    // 获取特定服务
    Service(name string) (CloudService, error)
}

// CloudService 单个云服务（如 RDS, ALB, S3）
type CloudService interface {
    Name() string        // "rds" | "alb" | "s3" | ...
    Description() string // LLM 工具描述
    
    // L0 只读操作
    ReadOnlyTools() []Tool
    
    // L1+ 修改操作（Phase 2）
    WriteTools() []Tool
}

// CloudToolContext 云工具执行的上下文
type CloudToolContext struct {
    Provider    CloudProvider
    Region      string            // "us-east-1"
    AccountID   string            // "123456789012"
    Credentials CloudCredentials
}

// CloudCredentials 云凭证
type CloudCredentials struct {
    // AWS
    AccessKeyID     string
    SecretAccessKey string
    SessionToken    string
    
    // 或通过 IAM Role / Instance Profile
    UseIAMRole      bool
    RoleARN         string
}
```

## 25.4 AWS 实现示例（RDS）

```go
// AWSProvider 实现 CloudProvider 接口
type AWSProvider struct {
    clients map[string]CloudService // region → service
    regions []string
}

// RDSService RDS 云服务
type RDSService struct {
    client *rds.RDS  // AWS SDK v2
}

func (r *RDSService) ReadOnlyTools() []Tool {
    return []Tool{
        {
            Name:        "cloud_rds_describe_instances",
            Description: "列出所有 RDS 实例，含状态、配置、终端节点",
            RiskLevel:   RiskLevelReadOnly,
        },
        {
            Name:        "cloud_rds_describe_instance",
            Description: "查看指定 RDS 实例的详细信息: CPU/内存/连接数/IOPS/存储",
            RiskLevel:   RiskLevelReadOnly,
            Parameters: map[string]interface{}{
                "instance_id": map[string]interface{}{
                    "type":        "string",
                    "description": "RDS 实例标识符，如 db-payment-prod",
                },
            },
        },
        {
            Name:        "cloud_rds_describe_slow_queries",
            Description: "最近 1 小时的慢查询日志（需要 RDS 开启了 slow_query_log）",
            RiskLevel:   RiskLevelReadOnly,
            Parameters: map[string]interface{}{
                "instance_id": map[string]interface{}{"type": "string"},
                "hours":       map[string]interface{}{"type": "integer", "default": 1},
                "limit":       map[string]interface{}{"type": "integer", "default": 20},
            },
        },
        {
            Name:        "cloud_rds_describe_parameter_group",
            Description: "查看 RDS 参数组配置（含 max_connections, innodb_buffer_pool_size 等）",
            RiskLevel:   RiskLevelReadOnly,
            Parameters: map[string]interface{}{
                "parameter_group_name": map[string]interface{}{"type": "string"},
            },
        },
        {
            Name:        "cloud_rds_get_metrics",
            Description: "查看 RDS CloudWatch 指标: CPUUtilization, DatabaseConnections, FreeableMemory, ReadIOPS, WriteIOPS",
            RiskLevel:   RiskLevelReadOnly,
            Parameters: map[string]interface{}{
                "instance_id": map[string]interface{}{"type": "string"},
                "period":      map[string]interface{}{"type": "integer", "default": 300},
                "duration":    map[string]interface{}{"type": "string", "default": "1h"},
            },
        },
    }
}

func (r *RDSService) WriteTools() []Tool {
    // Phase 2: 修改参数组、创建读副本等
    return nil
}

func (t *cloudRDSDescribeInstanceTool) Execute(input ToolExecInput, ctx ToolContext) (ToolResult, error) {
    instanceID := input.Parameters["instance_id"].(string)
    
    result, err := t.rdsClient.DescribeDBInstances(context.Background(), &rds.DescribeDBInstancesInput{
        DBInstanceIdentifier: aws.String(instanceID),
    })
    if err != nil {
        return ToolResult{Success: false, Error: err}, err
    }
    
    if len(result.DBInstances) == 0 {
        return ToolResult{
            Success: true,
            Output:  fmt.Sprintf("RDS 实例 '%s' 不存在或无权访问。", instanceID),
        }, nil
    }
    
    instance := result.DBInstances[0]
    
    output := fmt.Sprintf(`
RDS 实例: %s
  状态: %s | 引擎: %s %s
  实例类型: %s | 存储: %d GB (%s)
  
  终端节点: %s:%d
  多 AZ: %v | 读副本: %d
  
  连接数: %d/%d (当前/最大)
  CPU: %.1f%% | 内存: %d GB
  IOPS: %d (读) / %d (写)
  
  备份: %d 天保留 | 最新: %s
  维护窗口: %s
  参数组: %s
  
  安全组: %s
  加密: %v
`,
        *instance.DBInstanceIdentifier,
        *instance.DBInstanceStatus,
        *instance.Engine, *instance.EngineVersion,
        *instance.DBInstanceClass,
        *instance.AllocatedStorage, *instance.StorageType,
        *instance.Endpoint.Address, int64(*instance.Endpoint.Port),
        *instance.MultiAZ,
        len(instance.ReadReplicaDBInstanceIdentifiers),
        // 连接数和性能指标需要单独 API 调用
        "? (使用 cloud_rds_get_metrics 查看)", "?",
        "?", "?",
        "?", "?",
        "?", "?",
        "?",
        strings.Join(aws.StringSlice(instance.VpcSecurityGroups), ", "),
        *instance.StorageEncrypted,
    )
    
    return ToolResult{Success: true, Output: output}, nil
}
```

## 25.5 System Prompt 注入（云资源诊断知识）

```
CLOUD RESOURCE TROUBLESHOOTING KNOWLEDGE (AWS):

When a K8s issue points to a cloud resource problem:

1. RDS Connection Timeout from K8s Pod:
   → cloud_rds_describe_instance <id> — check status, connections, CPU
   → cloud_rds_get_metrics <id> — check DatabaseConnections trend
   → cloud_rds_describe_slow_queries <id> — check for slow queries
   → Common causes: max_connections limit, CPU saturation, storage full,
     AZ outage, security group misconfiguration

2. ALB 502/503 from K8s Ingress:
   → cloud_alb_describe_target_health <alb-arn> — check target group health
   → Cross-reference unhealthy targets with K8s nodes: are nodes Ready?
   → Common causes: node NotReady, Pod not listening on target port,
     health check path wrong, security group blocking

3. S3 403 Access Denied from K8s Pod (IRSA/Instance Profile):
   → cloud_iam_get_role <role-name> — check attached policies
   → cloud_s3_get_bucket_policy <bucket> — check bucket policy
   → Common causes: IRSA annotation missing on ServiceAccount,
     bucket policy denies the role, KMS key policy missing

4. Cloud integration priority:
   - Always exhaust K8s-level diagnosis first (Pod, Service, Ingress)
   - Only invoke cloud tools when the root cause clearly points outside K8s
   - NEVER suggest modifying IAM policies or security groups (L4)
   - ALWAYS explain the cloud resource diagnosis in K8s terms
     (e.g. "RDS max_connections 198/200 → connection timeout → Pod logs show 'dial tcp timeout'")
```

## 25.6 配置

```yaml
# config.yaml
cloud_providers:
  aws:
    enabled: true
    regions:
      - "us-east-1"
      - "ap-southeast-1"
    credentials:
      # 方式 1: 环境变量（推荐，与 aws CLI 一致）
      source: "environment"   # AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
      
      # 方式 2: IAM Role（Pod 内运行时自动使用）
      # source: "iam_role"
      # role_arn: "arn:aws:iam::123456789012:role/ops-ai-readonly"
      
      # 方式 3: 配置文件
      # source: "profile"
      # profile: "ops-ai-prod"
    
    # 资源发现（自动关联 K8s 资源和云资源）
    resource_mapping:
      # 从 K8s Service annotation 自动发现关联的 RDS/ALB
      auto_discover: true
      annotation_prefix: "ops-ai.cloud.aws"  # ops-ai.cloud.aws/rds-instance: db-payment-prod
    
    # MVP 启用的服务
    services:
      - rds          # RDS 实例诊断
      - alb          # ALB/NLB 目标组健康检查
      - s3           # S3 Bucket 策略/对象列表
      - route53      # DNS 记录查询
      - elasticache  # Redis/Memcached 状态
      - iam          # IAM Role/Policy 只读（L0 only）
    
    # L0 只读强制
    read_only: true   # MVP 阶段强制只读，Phase 2 放开

  # Phase 2: Azure / GCP / 阿里云
  # azure:
  #   enabled: false
  # gcp:
  #   enabled: false
```

## 25.7 K8s 资源到云资源的自动关联

运维最烦的就是"我知道 payment 用的是哪个 DB 但 Agent 不知道"。通过 K8s annotation 或命名约定自动关联：

```yaml
# 方式 1: Service/Deployment annotation
apiVersion: v1
kind: Service
metadata:
  name: payment
  annotations:
    ops-ai.cloud.aws/rds-instance: "db-payment-prod"
    ops-ai.cloud.aws/elasticache-cluster: "payment-cache-prod"
---
# 方式 2: ConfigMap 声明（集中管理）
apiVersion: v1
kind: ConfigMap
metadata:
  name: ops-ai-resource-mapping
  namespace: ops-ai
data:
  mappings.yaml: |
    payment:
      rds: db-payment-prod
      elasticache: payment-cache-prod
      alb: payment-alb
    order-svc:
      rds: db-order-prod
      alb: order-alb
```

Agent 行为：
```
用户: "payment 超时了"
Agent: 
  1. 从 annotation/ConfigMap 获取: payment → rds: db-payment-prod
  2. 并行执行: kubectl get pods + cloud_rds_describe_instance db-payment-prod
  3. 综合分析 K8s 状态和 RDS 状态
```

## 25.8 TUI 集成

```
云资源状态行（启动展示）:
═══ prod-us-east-1 ═══
🟢 5/5 nodes | ⚠️ 3 abnormal pods | 🟢 quotas OK
☁️  RDS db-payment-prod: 🟢 | ALB payment-alb: 🟢 | ElastiCache: 🟢

云工具调用展示:
[14:32:05] 🔍 cloud_rds_describe_instance db-payment-prod
           RDS: Available | CPU: 92% ⚠️ | 连接: 198/200 ⚠️
[14:32:06] 🔍 cloud_rds_get_metrics db-payment-prod (1h)
           DatabaseConnections: 上升趋势 (120→198 in 1h)
```

---

# 第二十六部分：首次运行体验（Onboarding）

## 26.1 问题场景

v1.6 的所有功能都假设"所有东西已经配好了"。一个新运维下载 ops-ai 后：

```
$ ./ops-ai
panic: kubeconfig not found: stat ~/.kube/config: no such file or directory
```

**然后呢？** 用户看到的是一个 Go panic，不是友好引导。这不是产品可交付的体验。

真实失败路径：
- 阿里云用户没有 `~/.kube/config`，用的是 `KUBECONFIG` 环境变量
- WSL2 用户的 kubeconfig 在 `/mnt/c/Users/...` 路径，与 Linux 路径不一致
- 企业内用户 kubeconfig 是通过 SSO 证书 + kubectl plugin 生成的
- LLM API key 没配 → Agent 启动后在第一条消息卡住，报 401
- RBAC 不足 → 健康概览全是 ❌，用户不知道该找谁申请什么权限

## 26.2 设计原则

> **目标**：从 `./ops-ai` 到第一个 `/health` 成功 < 2 分钟，零文档也能完成配置。

核心思路：**ops-ai 自己帮用户排查配置问题**——你连不上 K8s？我来告诉你是没装 kubectl、还是 kubeconfig 格式不对、还是网络不通。

## 26.3 启动自检流程（4 阶段）

```
┌─────────────────────────────────────────────────────────┐
│ 阶段 0: 二进制完整性                                    │
│  ✓ ops-ai binary                                  50ms │
│  ✓ embedded ca-certificates                       10ms │
│  ✓ sqlite driver loaded                           20ms │
├─────────────────────────────────────────────────────────┤
│ 阶段 1: 外部依赖检测                                    │
│  ✓ kubectl binary (可选，用于 fallback)            5ms │
│  ✓ kubeconfig 来源检测（$KUBECONFIG / ~/.kube/config ）│
│  ✓ docker / nerdctl (可选，容器模式需要)          3ms │
│  ✓ git (可选，Config Drift 需要)                  2ms │
├─────────────────────────────────────────────────────────┤
│ 阶段 2: K8s 连通性                                     │
│  → 尝试 apiserver /version                         2s │
│  → 如果失败：自动诊断（DNS/网络/证书/TLS 版本）   10s │
│  → RBAC 自检：列出可访问的 verbs+resources         5s │
├─────────────────────────────────────────────────────────┤
│ 阶段 3: LLM 后端可用性                                  │
│  → 检测配置源（config.yaml / 环境变量 / CLI flag）       │
│  → 发送最小化健康检查请求（1 token）              2s │
│  → 如果失败：显示 API key 获取指引                    │
└─────────────────────────────────────────────────────────┘
```

## 26.4 自检输出示例

### 正常启动

```
$ ./ops-ai

  ═══════════════════════════════════════════════════════
    ops-ai v0.1.0  —  运维 AI 副驾驶
  ═══════════════════════════════════════════════════════

  🔍 启动自检中...

  [✓] 二进制完整性
  [✓] kubeconfig: /home/ops/.kube/config
  [✓] K8s 连通: v1.29.5 (集群: prod-us-east-1)
  [✓] RBAC: cluster-admin (full access)
  [✓] LLM 后端: Claude (claude-sonnet-4-20250514)

  ────────────────────────────────────────────────────────
  ✅ 所有检查通过！进入对话...
  ────────────────────────────────────────────────────────

  ═════════════════════════════════════════════════════════
    集群健康概览 · prod-us-east-1
  ═════════════════════════════════════════════════════════
  🟢 节点:    8/8  Ready    | CPU: 34%  | Mem: 52%
  🟢 核心服务: 12/12 Running | 🟢 payment / 🟢 order / 🟢 user
  🟡 Pod 异常: 2 CrashLoopBackOff (notice-svc, batch-worker)
  🟢 PDB:     8/8 满足     | 配额: 4 namespaces 🟢
  🟢 部署:    45/45 Ready  | ArgoCD: 🟢 synced
  ────────────────────────────────────────────────────────
  💡 发现 2 个异常 Pod。输入 /health 查看详情。
  💡 输入 /help 查看可用命令。

  > 
```

### 配置缺失时

```
$ ./ops-ai

  ═══════════════════════════════════════════════════════
    ops-ai v0.1.0  —  运维 AI 副驾驶
  ═══════════════════════════════════════════════════════

  🔍 启动自检中...

  [✓] 二进制完整性
  [✗] kubeconfig: 未找到

  ────────────────────────────────────────────────────────
  ⚠️  未检测到 Kubernetes 配置

  我检查了以下位置（均不存在）：
    • $KUBECONFIG:   (未设置)
    • ~/.kube/config: /home/ops/.kube/config (不存在)

  请选择一种方式连接集群：

  ┌───────────────────────────────────────────────────────┐
  │ 1. 设置 KUBECONFIG 环境变量                          │
  │    export KUBECONFIG=/path/to/your/kubeconfig         │
  │                                                       │
  │ 2. 创建默认 kubeconfig 文件                           │
  │    mkdir -p ~/.kube                                    │
  │    cp /path/to/your/kubeconfig ~/.kube/config          │
  │                                                       │
  │ 3. 使用 --kubeconfig 参数启动                         │
  │    ./ops-ai --kubeconfig /path/to/config               │
  │                                                       │
  │ 4. 使用 kubectl 插件（如果你已装了 kubectl）          │
  │    kubectl ops-ai                                      │
  └───────────────────────────────────────────────────────┘

  💡 如果你没有 kubeconfig，联系集群管理员获取。
  💡 云厂商控制台通常提供 kubeconfig 下载功能。

  输入 /setup wizard 进入交互式配置向导。
```

### LLM API Key 缺失时

```
$ ./ops-ai

  🔍 启动自检中...

  [✓] kubeconfig: /home/ops/.kube/config
  [✓] K8s 连通: v1.29.5
  [✓] RBAC: admin
  [✗] LLM 后端: 未配置 API Key

  ────────────────────────────────────────────────────────
  ⚠️  未配置 AI 模型凭证

  ops-ai 需要 AI 模型才能理解自然语言。请选择一种方式：

  ┌───────────────────────────────────────────────────────┐
  │ 1. 使用云端模型（推荐）                                │
  │                                                       │
  │   a) Anthropic Claude                                 │
  │      获取 API Key: https://console.anthropic.com/      │
  │      配置: export ANTHROPIC_API_KEY=sk-ant-...         │
  │                                                       │
  │   b) OpenAI                                           │
  │      获取 API Key: https://platform.openai.com/        │
  │      配置: export OPENAI_API_KEY=sk-...                │
  │                                                       │
  │ 2. 使用本地模型（离线可用）                            │
  │                                                       │
  │     brew install ollama                                │
  │     ollama pull qwen2.5-coder:7b                       │
  │                                                        │
  └───────────────────────────────────────────────────────┘

  💡 也可以创建 ~/.ops-ai/config.yaml 持久化配置。
  💡 输入 /setup wizard 进入交互式配置向导。

  已选择？按 Enter 重新自检。
```

## 26.5 交互式配置向导（`/setup wizard`）

```
$ > /setup wizard

  ═══════════════════════════════════════════════════════
    ops-ai 配置向导
  ═══════════════════════════════════════════════════════

  让我们一步一步完成配置...

  ┌─ Step 1/5: AI 模型 ─────────────────────────────────┐
  │ 你想使用哪个 AI 提供商？                             │
  │                                                       │
  │ > 1. Anthropic Claude（推荐，K8s 领域能力最强）      │
  │   2. OpenAI                                           │
  │   3. 本地模型 (Ollama)                                │
  │   4. 自定义 API 端点                                  │
  │                                                       │
  │ 选择 [1-4]: 1                                         │
  └───────────────────────────────────────────────────────┘

  ┌─ Step 2/5: API Key ─────────────────────────────────┐
  │ 请输入你的 Anthropic API Key:                        │
  │ （输入时不会显示，按 Enter 确认）                     │
  │                                                       │
  │ > ************                                       │
  │                                                       │
  │ ✓ API Key 已保存到 ~/.ops-ai/config.yaml             │
  │   （仅你可读，权限 600）                              │
  └───────────────────────────────────────────────────────┘

  ... (继续 Step 3: K8s 配置, Step 4: 审计日志, Step 5: 完成)
```

## 26.6 Go 接口定义

```go
// Onboarder 启动自检引擎
type Onboarder struct {
    Checks []StartupCheck
}

// StartupCheck 单项检查
type StartupCheck struct {
    Name        string              // 检查项名称（展示用）
    Category    CheckCategory       // 阶段分类
    Mandatory   bool                // 是否阻断启动
    Check       func(ctx context.Context) (*CheckResult, error)
    Diagnose    func(err error) []string // 失败时的诊断建议
}

type CheckCategory string

const (
    CategoryBinary      CheckCategory = "binary"       // 阶段 0
    CategoryDependency  CheckCategory = "dependency"   // 阶段 1
    CategoryConnectivity CheckCategory = "connectivity" // 阶段 2
    CategoryLLM         CheckCategory = "llm"           // 阶段 3
)

type CheckResult struct {
    Passed  bool
    Message string            // 简短结果描述
    Details map[string]string // 详细信息
}

// 启动流程
func (o *Onboarder) RunStartup(ctx context.Context) (*StartupReport, error) {
    report := &StartupReport{
        Checks:   make([]CheckResult, 0, len(o.Checks)),
        Warnings: make([]string, 0),
    }

    for _, check := range o.Checks {
        result, err := check.Check(ctx)
        if err != nil {
            report.Checks = append(report.Checks, CheckResult{
                Passed:  false,
                Message: check.Name,
                Details: map[string]string{"error": err.Error()},
            })
            // 非阻断检查失败只记录 warning
            if !check.Mandatory {
                report.Warnings = append(report.Warnings,
                    fmt.Sprintf("[%s] %s: %v", check.Category, check.Name, err))
                continue
            }
            // 阻断检查失败 → 返回诊断建议
            report.BlockedBy = check.Name
            report.Suggestions = check.Diagnose(err)
            return report, nil
        }
        result.Message = check.Name
        report.Checks = append(report.Checks, *result)
    }

    report.AllPassed = len(report.Warnings) == 0
    return report, nil
}

type StartupReport struct {
    Checks      []CheckResult
    Warnings    []string
    AllPassed   bool
    BlockedBy   string   // 哪个检查阻断了启动
    Suggestions []string // 诊断建议
}
```

## 26.7 配置存储

```yaml
# ~/.ops-ai/config.yaml（由向导或环境变量填充）
# 权限：600（仅 owner 可读写）

llm:
  provider: anthropic          # anthropic | openai | ollama | custom
  api_key: ${ANTHROPIC_API_KEY} # 支持环境变量引用
  model: claude-sonnet-4-20250514
  # 自定义端点
  # endpoint: https://my-llm-proxy.internal/v1
  # ollama 本地
  # provider: ollama
  # model: qwen2.5-coder:7b
  # endpoint: http://localhost:11434

kubernetes:
  kubeconfig: ${KUBECONFIG:-~/.kube/config}
  context: ""              # 空 = 使用 current-context
  default_namespace: ""    # 空 = 使用 kubeconfig 默认值

audit:
  enabled: true
  sinks:
    - type: file
      path: ~/.ops-ai/audit.log
    - type: stdout
      format: json

safety:
  auto_approve_dev: true   # dev/staging 环境自动确认 L1-L2
  require_namespace_confirmation: true  # L2+ 操作确认当前 namespace
```

---

# 第二十七部分：kubectl logs -f 流式日志

## 27.1 问题场景

v1.6 的日志能力全是静态截断（`--tail=N`、`--since=5m`、`--previous`），但在运维实战中：

| 场景 | 需求 | v1.6 能做到吗？ |
|------|------|:---:|
| "持续监控 payment 日志，看到 ERROR 就停" | 流式 + 模式匹配终止 | ❌ |
| "tail -f 看看新 Pod 有没有 crash" | 实时 tail | ❌ |
| "部署后 watch 日志 30 秒，确认没有异常" | 定时流 + 自动终止 | ❌ |
| "这 3 个 Pod 同时 tail 日志" | 多 Pod 聚合流 | ❌ |

这是运维排障最高频的操作之一，缺失是不可接受的。

## 27.2 设计目标

1. **流式输出在 TUI 中实时渲染**——不是等流结束再一次性展示
2. **Agent 可以中止流**——LLM 检测到关键信号后停止流，而不是让流无限跑下去
3. **支持复合流**——多 Pod 日志聚合到一个流中
4. **Agent Loop 不卡死**——流式日志作为后台任务，不阻塞 Agent loop 主循环

## 27.3 操作分级

| 操作 | 级别 | 说明 |
|------|:---:|------|
| `kubectl logs <pod> --tail=N` | L0 | 静态截断，安全 |
| `kubectl logs <pod> -f --max-duration=30s` | L0 | 限时流，安全 |
| `kubectl logs <pod> -f --stop-on="ERROR\|FATAL"` | L0 | 模式终止流，安全 |
| `kubectl logs <pod> -f`（无限制） | L1 | 无限流，需确认 |
| `kubectl logs -l app=payment --all-containers -f` | L1 | 复合流，需确认 |
| `kubectl logs <pod> -f` + 自动分析（Agent 持续分析日志内容） | L1 | 流+AI 分析 |

## 27.4 Go 接口定义

```go
// StreamLogsRequest 流式日志请求
type StreamLogsRequest struct {
    PodName       string        `json:"pod_name"`
    Namespace     string        `json:"namespace"`
    Container     string        `json:"container,omitempty"`      // 空 = 默认容器
    Previous      bool          `json:"previous,omitempty"`       // 上一个容器实例
    
    // 终止条件（至少设置一个）
    MaxDuration   time.Duration `json:"max_duration,omitempty"`   // 最大持续时间
    MaxLines      int           `json:"max_lines,omitempty"`      // 最大行数
    StopPattern   string        `json:"stop_pattern,omitempty"`   // 正则匹配即停止
    StopTimeout   time.Duration `json:"stop_timeout,omitempty"`   // 空闲超时停止
    
    // 多 Pod 聚合
    LabelSelector string        `json:"label_selector,omitempty"` // --selector
    AllContainers bool          `json:"all_containers,omitempty"`
    
    // 流控制
    BufferSize    int           `json:"buffer_size,omitempty"`    // 默认 1024 行
    SinceSeconds  int           `json:"since_seconds,omitempty"`  // --since
}

// StreamLogsEvent 流式日志事件
type StreamLogsEvent struct {
    Type      StreamEventType `json:"type"`
    Timestamp time.Time       `json:"timestamp"`
    PodName   string          `json:"pod_name"`
    Line      string          `json:"line"`
}

type StreamEventType string

const (
    StreamEventLine      StreamEventType = "line"       // 普通日志行
    StreamEventError     StreamEventType = "error"      // 错误日志行（匹配到 ERROR/FATAL）
    StreamEventStopped   StreamEventType = "stopped"    // 流已停止（触发终止条件）
    StreamEventEOF       StreamEventType = "eof"        // Pod 日志结束
    StreamEventTimedOut  StreamEventType = "timed_out"  // 超时
    StreamEventPodGone   StreamEventType = "pod_gone"   // Pod 已删除
)

// LogStreamer 日志流管理器
type LogStreamer struct {
    // ctx 取消即停止所有流
    ctx    context.Context
    cancel context.CancelFunc
}

// StartStream 启动日志流，返回事件 channel
func (s *LogStreamer) StartStream(req StreamLogsRequest) (<-chan StreamLogsEvent, error) {
    // 1. 验证 Pod 存在
    // 2. 构建 k8s log API 请求
    // 3. 启动 goroutine 读取流
    // 4. 每收到一行 → 推送到 channel
    // 5. 检测终止条件 → 关闭 channel
}

// StopAll 停止所有活跃流
func (s *LogStreamer) StopAll() {
    s.cancel()
}
```

## 27.5 Agent Loop 中的流式日志处理

```
用户: "tail -f payment Pod 日志，看到 ERROR 就告诉我"

Agent Loop:
┌─────────────────────────────────────────────────┐
│ Step 1: LLM 理解意图                             │
│   → 识别到 logs -f + 模式匹配需求                │
│   → SafetyGate: L0 (有 stop_pattern 限制)        │
├─────────────────────────────────────────────────┤
│ Step 2: 创建 StreamLogsRequest                   │
│   → pod: payment-7d4f8b9c-x2k, -f                │
│   → stop_pattern: "ERROR|FATAL|Exception"         │
│   → max_duration: 5m (安全兜底，防止无限等待)    │
├─────────────────────────────────────────────────┤
│ Step 3: 调用 LogStreamer.StartStream()           │
│   → 返回 event channel                           │
│   → Agent Loop 进入流监控子循环                   │
├─────────────────────────────────────────────────┤
│ Step 4: 流监控子循环                              │
│   while event := <-ch:                           │
│     case line:                                   │
│       → 追加到内存 buffer（最多 200 行）         │
│       → 推送到 TUI 流式渲染                      │
│     case error:                                  │
│       → 标记该行 + 继续监控                       │
│     case stopped:                                │
│       → 提取最近 50 行 + 所有 error 行           │
│       → 喂给 LLM 分析: "日志中检测到 ERROR: ..." │
│       → 退出子循环                               │
│     case timed_out | eof | pod_gone:             │
│       → 清理 + 退出                               │
├─────────────────────────────────────────────────┤
│ Step 5: 返回 LLM 分析结果给用户                  │
└─────────────────────────────────────────────────┘
```

## 27.6 TUI 渲染

```
支付服务部署后的日志监控:

$ ./ops-ai --watch-logs "app=payment" --duration 30s

  ═══════════════════════════════════════════════════════
  📺 实时日志 · payment (3 Pods) · 已运行 12s / 30s
  ═══════════════════════════════════════════════════════

  [payment-7d4f-1] 14:32:01.234 INFO  Starting payment service v2.3.1
  [payment-7d4f-2] 14:32:01.235 INFO  Starting payment service v2.3.1
  [payment-7d4f-3] 14:32:01.236 INFO  Starting payment service v2.3.1
  [payment-7d4f-1] 14:32:02.100 INFO  Connected to database
  [payment-7d4f-2] 14:32:02.101 INFO  Connected to database
  [payment-7d4f-1] 14:32:03.450 INFO  Health check: OK
  [payment-7d4f-3] 14:32:03.451 INFO  Health check: OK
  [payment-7d4f-2] 14:32:03.452 ERROR Failed to connect to Redis ⚠️
  [payment-7d4f-2] 14:32:03.453 WARN  Retrying Redis connection (1/5)

  ═══════════════════════════════════════════════════════
  ⚠️  检测到 ERROR 行（1 条）   |  按 Ctrl+C 停止
  ═══════════════════════════════════════════════════════
```

## 27.7 安全约束

| 约束 | 说明 |
|------|------|
| **硬超时（Hard Timeout）** | 所有流最长 10 分钟，防止 Agent Loop 永久卡死 |
| **缓冲区上限** | 内存 buffer 最多 5000 行，超出则丢弃最早行 |
| **pod_gone 自动终止** | Pod 被删除时自动清理流通道 |
| **多流并发限制** | 最多同时 5 个流，防止资源耗尽 |
| **生产环境 L1 确认** | 生产环境无限流需要用户确认 |
| **Secret 脱敏** | 流经的日志行同样经过 Secret 三层处理（§11） |

---

# 第二十八部分：Pre-flight 统一编排框架

## 28.1 问题场景

v1.6 的 §5.7-5.12 各自定义了独立的 pre-flight 检查函数（AdmissionWebhook、PDB、PSA、Operator、ResourceQuota、NetworkPolicy），但全部零散、无协调：

真实场景：集群 API 正在经历网络抖动（慢但没死）。L2 `kubectl scale` 触发 pre-flight：
- ResourceQuota → 3 秒返回 ✓
- PDB → 8 秒返回 ✓  
- AdmissionWebhook → **60 秒超时** ✗

Agent Loop 会一直等吗？跳过？降级？超时了算 pre-flight passed 还是 failed？

v1.6 没有定义 pre-flight 的超时策略、并行度、失败降级逻辑。在生产集群 API 抖动时，这会直接导致 Agent 卡死。

## 28.2 设计目标

1. **统一注册机制**——所有 pre-flight check 实现同一接口
2. **并行执行 + 独立超时**——每个 check 有独立超时，不互相阻塞
3. **分级结果**——PASS / WARN / BLOCK / TIMEOUT，不同结果不同行为
4. **降级策略**——API 不可达时优雅降级，不是卡死或 panic

## 28.3 Go 接口定义

```go
// PreFlightChecker 统一 pre-flight 检查接口
type PreFlightChecker interface {
    // Name 检查器名称（用于日志和 TUI 展示）
    Name() string
    
    // Priority 执行优先级（数字越小越先执行）
    // 0-10: 资源配额类（快速，<1s）
    // 11-20: 策略检查类（中等，<3s）
    // 21-30: 外部依赖类（慢，<10s）
    Priority() int
    
    // Timeout 单个检查的超时时间
    Timeout() time.Duration
    
    // Check 执行检查
    Check(ctx context.Context, op Operation) (*PreFlightResult, error)
    
    // IsApplicable 此检查是否适用于当前操作
    // 例如：PDB 检查只适用于 delete/drain/scale
    IsApplicable(op Operation) bool
}

// PreFlightResult 检查结果
type PreFlightResult struct {
    Status   PreFlightStatus   `json:"status"`
    Message  string            `json:"message"`
    Details  map[string]string `json:"details,omitempty"`
    Duration time.Duration     `json:"duration"`
}

type PreFlightStatus string

const (
    PreFlightPass    PreFlightStatus = "PASS"    // 检查通过，无风险
    PreFlightWarn    PreFlightStatus = "WARN"    // 有风险但可继续（用户确认后）
    PreFlightBlock   PreFlightStatus = "BLOCK"   // 严重风险，拒绝操作
    PreFlightTimeout PreFlightStatus = "TIMEOUT" // 检查超时，降级处理
    PreFlightError   PreFlightStatus = "ERROR"   // 检查遇到错误
    PreFlightSkipped PreFlightStatus = "SKIPPED" // 不适用，跳过
)

// PreFlightOrchestrator 统一编排器
type PreFlightOrchestrator struct {
    checkers    []PreFlightChecker
    degradeOK   bool  // 超时/错误是否允许降级继续
}

// RunAll 并行执行所有适用的 pre-flight 检查
func (o *PreFlightOrchestrator) RunAll(ctx context.Context, op Operation) *PreFlightReport {
    report := &PreFlightReport{
        Operation: op,
        Results:   make(map[string]*PreFlightResult),
    }
    
    // 1. 过滤：只保留适用的检查器
    var applicable []PreFlightChecker
    for _, c := range o.checkers {
        if c.IsApplicable(op) {
            applicable = append(applicable, c)
        }
    }
    
    // 2. 按优先级排序
    sort.Slice(applicable, func(i, j int) bool {
        return applicable[i].Priority() < applicable[j].Priority()
    })
    
    // 3. 并行执行，每个有独立 ctx + timeout
    type checkOutcome struct {
        name   string
        result *PreFlightResult
        err    error
    }
    
    results := make(chan checkOutcome, len(applicable))
    
    startTime := time.Now()
    for _, checker := range applicable {
        go func(c PreFlightChecker) {
            checkCtx, cancel := context.WithTimeout(ctx, c.Timeout())
            defer cancel()
            
            result, err := c.Check(checkCtx, op)
            if err != nil {
                if errors.Is(err, context.DeadlineExceeded) {
                    results <- checkOutcome{c.Name(), &PreFlightResult{
                        Status:  PreFlightTimeout,
                        Message: fmt.Sprintf("检查超时 (limit: %v)", c.Timeout()),
                    }, nil}
                    return
                }
                results <- checkOutcome{c.Name(), &PreFlightResult{
                    Status:  PreFlightError,
                    Message: err.Error(),
                }, err}
                return
            }
            results <- checkOutcome{c.Name(), result, nil}
        }(checker)
    }
    
    // 4. 收集结果
    for range applicable {
        outcome := <-results
        report.Results[outcome.name] = outcome.result
    }
    report.TotalDuration = time.Since(startTime)
    
    // 5. 汇总判定
    report.Verdict = o.determineVerdict(report)
    
    return report
}

// determineVerdict 综合判定
func (o *PreFlightOrchestrator) determineVerdict(report *PreFlightReport) PreFlightVerdict {
    hasBlock := false
    hasWarn := false
    hasTimeout := false
    
    for _, r := range report.Results {
        switch r.Status {
        case PreFlightBlock:
            hasBlock = true
        case PreFlightWarn:
            hasWarn = true
        case PreFlightTimeout, PreFlightError:
            hasTimeout = true
        }
    }
    
    if hasBlock {
        return VerdictBlock  // 有 BLOCK → 直接拒绝
    }
    if hasTimeout && !o.degradeOK {
        return VerdictAbort   // 关键检查超时且不允许降级 → 中止
    }
    if hasTimeout && o.degradeOK {
        report.DegradedChecks = o.collectTimeoutCheckers(report)
        if hasWarn {
            return VerdictProceedWithCaution // 超时降级 + 有 WARN
        }
        return VerdictProceedDegraded       // 超时降级，无 WARN
    }
    if hasWarn {
        return VerdictProceedWithCaution
    }
    return VerdictProceed
}

type PreFlightVerdict string

const (
    VerdictProceed          PreFlightVerdict = "PROCEED"            // 全部通过
    VerdictProceedWithCaution PreFlightVerdict = "PROCEED_CAUTION" // 有风险但可继续
    VerdictProceedDegraded  PreFlightVerdict = "PROCEED_DEGRADED"  // 降级模式继续
    VerdictAbort            PreFlightVerdict = "ABORT"              // 中止操作
    VerdictBlock            PreFlightVerdict = "BLOCK"              // 严重风险，拒绝
)

type PreFlightReport struct {
    Operation       Operation                    `json:"operation"`
    Results         map[string]*PreFlightResult  `json:"results"`
    Verdict         PreFlightVerdict             `json:"verdict"`
    DegradedChecks  []string                     `json:"degraded_checks,omitempty"`
    TotalDuration   time.Duration                `json:"total_duration"`
}
```

## 28.4 已有检查器注册（v1.3-v1.6 的 pre-flight 统一至此）

```go
// 系统内置检查器注册（main.go 初始化时调用）
func RegisterBuiltinPreFlightCheckers(orchestrator *PreFlightOrchestrator, k8s kubernetes.Interface) {
    // 按优先级升序注册
    
    orchestrator.Register(&ResourceQuotaChecker{
        client: k8s,
        timeout: 3 * time.Second,  // Priority 1: 最快
    })
    
    orchestrator.Register(&LimitRangeChecker{
        client: k8s,
        timeout: 3 * time.Second,  // Priority 2
    })
    
    orchestrator.Register(&PDBChecker{
        client: k8s,
        timeout: 5 * time.Second,  // Priority 10
    })
    
    orchestrator.Register(&PSAChecker{
        client: k8s,
        timeout: 5 * time.Second,  // Priority 12
    })
    
    orchestrator.Register(&AdmissionWebhookChecker{
        client:    k8s,
        timeout:   10 * time.Second, // Priority 20: 较慢
        degradeOK: true,             // 超时可降级
    })
    
    orchestrator.Register(&OperatorOwnerChecker{
        client:    k8s,
        timeout:   8 * time.Second,  // Priority 25
        degradeOK: true,
    })
    
    orchestrator.Register(&NetworkPolicyChecker{
        client:    k8s,
        timeout:   10 * time.Second, // Priority 28
        degradeOK: true,
    })
    
    // 默认：允许超时降级（经验保守策略）
    // 运维可以修改: --preflight-strict（超时也 block）
    orchestrator.SetDegradeOK(true)
}
```

## 28.5 TUI 渲染（pre-flight 面板）

```
  ═══════════════════════════════════════════════════════
  🔍 Pre-flight 检查 · kubectl scale deploy/payment --replicas=3
  ═══════════════════════════════════════════════════════

  [✓] ResourceQuota    · payment ns: CPU 2/8, Mem 4Gi/16Gi  (0.3s)
  [✓] LimitRange       · payment ns: limits OK              (0.2s)
  [✓] PDB              · payment-pdb: minAvailable=2 ✓       (1.2s)
  [✓] PSA              · payment ns: baseline ✓              (0.5s)
  [⚠️] AdmissionWebhook · webhook-timeout skipped (TIMEOUT)   (10.0s)
  [✓] OperatorOwner    · no operator ownerRefs               (0.8s)
  [—] NetworkPolicy    · SKIPPED (not applicable for scale)  (0.0s)

  ═══════════════════════════════════════════════════════
  📊 综合判定: PROCEED_DEGRADED (1 项超时降级，无阻塞风险)
  💡 提示: AdmissionWebhook 检查超时（10s），已自动降级跳过。
           如果担心 webhook 阻塞，可稍后手动验证。
  ═══════════════════════════════════════════════════════

  是否继续？ [Y/n]
```

## 28.6 降级策略矩阵

| 场景 | Pre-flight 结果 | Agent 行为 |
|------|:---:|------|
| 全部 PASS | PROCEED | 自动继续，不打断 |
| 有 WARN，无 BLOCK | PROCEED_CAUTION | 展示 WARN 详情，用户确认后继续 |
| 有 TIMEOUT（允许降级） | PROCEED_DEGRADED | 展示超时项，提示风险，用户确认后继续 |
| 有 TIMEOUT（严格模式） | ABORT | 拒绝操作，建议"稍后重试或 --preflight-loose" |
| 有 BLOCK | BLOCK | 拒绝操作，展示阻塞原因和解决建议 |
| API 完全不可达（所有检查超时） | ABORT | 拒绝操作，提示"集群 API 不可达" |

## 28.7 CLI 参数

```bash
# 默认：允许超时降级
./ops-ai

# 严格模式：超时也 block
./ops-ai --preflight-strict

# 跳过所有 pre-flight（仅限紧急情况，审计日志标记危险）
./ops-ai --skip-preflight  # L2 操作额外确认
```

---

# 第二十九部分：Agent Loop 崩溃恢复

## 29.1 问题场景

真实场景：运维凌晨 3 点 on-call，用 ops-ai 排查 payment 故障，12 轮对话后：

```
[ERROR] LLM response parse error: unexpected JSON token at position 487
panic: runtime error: invalid memory address or nil pointer dereference
[Agent Loop exited with code 2]
```

用户重新 `./ops-ai` → 空白 TUI → 12 轮排查成果全部丢失。

这不是偶发问题。运维 AI 工具的崩溃源：
- LLM API 返回格式错误（概率 ~0.1%/请求）
- client-go Informer 缓存不一致导致 nil pointer
- 磁盘满导致 SQLite 写入失败
- K8s API 返回非预期的 GVK 导致反序列化 panic

## 29.2 设计目标

> **崩溃不丢上下文。** Agent Loop 在任何时候 panic，下次启动自动检测并恢复。

1. **增量持久化**——每轮对话结束后自动保存会话快照
2. **崩溃检测**——启动时检查 SQLite 中的未完成会话
3. **恢复确认**——用户选择是否恢复、从哪里恢复
4. **恢复完整性**——恢复的会话包含对话历史 + 工具调用记录 + K8s 资源快照

## 29.3 实现机制

```
Agent Loop 主循环中的持久化锚点:

┌─────────────────────────────────────────────────────────┐
│ Agent Loop Iteration                                    │
│                                                         │
│ 1. Receive user input                                   │
│ 2. LLM reasoning → tool call                            │
│ 3. SafetyGate check                                     │
│ 4. Execute tool ──────────────────┐                     │
│ 5. Collect result                 │                     │
│ 6. ┌──────────────────────────────┘                     │
│    │ ★ Checkpoint: 保存会话到 SQLite                    │
│    │   - conversation_history (messages)                │
│    │   - tool_calls (id, timestamp, input, output)      │
│    │   - snapshots (k8s resource refs)                  │
│    │   - agent_state (current namespace, cluster, etc.) │
│    └────────────────────────────────────────────────────│
│ 7. LLM interprets result → next action                  │
│    ┌────────────────────────────────────────────────────│
│    │ ★ Checkpoint: 保存 LLM 响应                        │
│    └────────────────────────────────────────────────────│
│ 8. Respond to user                                      │
│                                                         │
│ 如果在步骤 2-7 之间崩溃 → 下次启动恢复至上一个完整 checkpoint
└─────────────────────────────────────────────────────────┘
```

## 29.4 Go 接口定义

```go
// SessionManager 会话持久化与恢复管理器
type SessionManager struct {
    db *sql.DB
}

// Session 会话记录
type Session struct {
    ID          string    `json:"id"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Status      SessionStatus `json:"status"`
    ClusterName string    `json:"cluster_name"`
    Namespace   string    `json:"namespace"`
    Summary     string    `json:"summary"`    // 用户第一条消息的前 80 字
    MessageCount int     `json:"message_count"`
}

type SessionStatus string

const (
    SessionActive     SessionStatus = "active"      // 正常进行中
    SessionCrashed    SessionStatus = "crashed"     // 检测到非正常退出
    SessionCompleted  SessionStatus = "completed"   // 正常结束
    SessionExported   SessionStatus = "exported"    // 已导出
)

// Checkpoint 增量检查点（每次 LLM 交互后保存）
type Checkpoint struct {
    SessionID   string           `json:"session_id"`
    MessageSeq  int              `json:"message_seq"`   // 消息序号
    Messages    []Message        `json:"messages"`      // 完整对话历史
    AgentState  AgentState       `json:"agent_state"`    // Agent 状态
    CreatedAt   time.Time        `json:"created_at"`
}

type AgentState struct {
    CurrentNamespace  string `json:"current_namespace"`
    CurrentCluster    string `json:"current_cluster"`
    PendingOperation  string `json:"pending_operation,omitempty"` // 上次执行到一半的操作
    Environment       string `json:"environment"`
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(cluster, namespace string) (*Session, error)

// SaveCheckpoint 保存增量检查点
func (sm *SessionManager) SaveCheckpoint(sessionID string, messages []Message, state AgentState) error

// DetectCrashedSessions 检测崩溃会话
func (sm *SessionManager) DetectCrashedSessions() ([]*Session, error)

// RestoreSession 恢复会话
func (sm *SessionManager) RestoreSession(sessionID string) (*Session, []Message, AgentState, error)

// MarkCompleted 标记会话正常完成
func (sm *SessionManager) MarkCompleted(sessionID string) error

// ListRecentSessions 列出最近会话
func (sm *SessionManager) ListRecentSessions(limit int) ([]*Session, error)
```

## 29.5 崩溃恢复流程

### 启动时检测

```
$ ./ops-ai

  ═══════════════════════════════════════════════════════
    ops-ai v0.1.0  —  运维 AI 副驾驶
  ═══════════════════════════════════════════════════════

  🔍 启动自检中...  [全部通过 ✓]

  ═══════════════════════════════════════════════════════
  ⚠️  检测到 1 个未完成的会话
  ═══════════════════════════════════════════════════════

  ┌───────────────────────────────────────────────────────┐
  │ 会话 #a3f2 · 2026-06-25 03:14:22                     │
  │                                                       │
  │ 集群: prod-us-east-1 · namespace: payment             │
  │ 摘要: "payment 服务响应超时，帮忙排查一下"              │
  │ 对话轮数: 12 轮  ·  工具调用: 8 次                     │
  │ 上次操作: kubectl describe deploy/payment             │
  │ 异常退出: 03:28:45 (panic: nil pointer dereference)   │
  └───────────────────────────────────────────────────────┘

  你想怎么处理？

  ┌───────────────────────────────────────────────────────┐
  │ [R] 恢复会话 — 接续上次对话继续排查                   │
  │ [V] 查看摘要 — 只看排查结论，不恢复完整上下文         │
  │ [S] 保存并新建 — 导出到文件后开始新会话               │
  │ [D] 丢弃 — 删除此会话，开始全新的                     │
  └───────────────────────────────────────────────────────┘
```

### 恢复后

```
  ✅ 会话 #a3f2 已恢复

  加载了 12 轮对话历史 + 8 次工具调用上下文。
  上次你在排查 payment 超时问题，我执行了:
    ✓ kubectl get pods -n payment
    ✓ kubectl describe deploy/payment
    ✓ kubectl top pods -n payment
    ✓ kubectl logs payment-7d4f-1 --tail=100
    ... (还有 4 项)

  ═══════════════════════════════════════════════════════
  上次对话:
  ═══════════════════════════════════════════════════════
  > 你: payment 服务响应超时，帮忙排查一下
  > ops-ai: 我来帮你排查。先看下 payment namespace 的状态...
  ... (12 轮对话回放)
  > ops-ai: Describe 显示 payment-7d4f-1 的 Readiness probe 失败，正在检查...[会话中断]

  ═══════════════════════════════════════════════════════
  💡 会话已恢复。继续排查吗？输入你的问题即可。
  ═══════════════════════════════════════════════════════

  > 
```

## 29.6 panic recovery wrapper

```go
// SafeAgentLoop 带崩溃恢复的 Agent Loop
func SafeAgentLoop(agent *Agent, sessionMgr *SessionManager, sessionID string) {
    defer func() {
        if r := recover(); r != nil {
            // 1. 记录崩溃信息
            stack := debug.Stack()
            log.Printf("Agent Loop PANIC: %v\n%s", r, stack)
            
            // 2. 尝试保存最后一个 checkpoint
            agent.mu.Lock()
            if len(agent.messages) > agent.lastCheckpointSeq {
                // 上一次 checkpoint 之后还有新消息
                // 无法保证完整性，但尽可能保存
                sessionMgr.SaveCheckpoint(sessionID, agent.messages, agent.state)
            }
            agent.mu.Unlock()
            
            // 3. 标记会话为 crashed
            sessionMgr.MarkCrashed(sessionID, fmt.Sprintf("panic: %v", r))
            
            // 4. 通知用户（如果 TUI 还活着）
            fmt.Fprintf(os.Stderr, "\n⚠️  Agent Loop 崩溃: %v\n", r)
            fmt.Fprintf(os.Stderr, "💡 会话已保存。重新启动后会自动检测并恢复。\n")
            
            os.Exit(2)
        }
    }()
    
    // 标记会话为 active
    sessionMgr.MarkActive(sessionID)
    
    // 正常运行 Agent Loop
    agent.Run()
    
    // 正常结束
    sessionMgr.MarkCompleted(sessionID)
}
```

---

# 第三十部分：用户/团队成本归属

## 30.1 问题场景

v1.6 §15 按团队规模估了总成本，但没有按用户拆分。

企业场景：
- 10 个运维共用 ops-ai，月底 LLM 账单 $300
- CTO 问："谁用了多少？哪个团队最耗资源？"
- 无法回答

没有成本归属，企业版无法做 chargeback / showback，也无法识别"谁的 prompt 模式特别费 token"来优化。

## 30.2 设计目标

1. **每次 LLM 调用都记录 token 消耗 + 费用**
2. **按用户/团队维度聚合**
3. **成本数据写入审计日志（同一 sink）**
4. **不增加额外基础设施**——成本归属与原审计体系共用存储

## 30.3 Go 接口定义

```go
// CostTracker 成本追踪器
type CostTracker struct {
    db     *sql.DB
    models map[string]ModelPricing
}

// ModelPricing 模型定价
type ModelPricing struct {
    Name          string  `json:"name"`
    InputPer1K    float64 `json:"input_per_1k"`    // $/1K tokens
    OutputPer1K   float64 `json:"output_per_1k"`   // $/1K tokens
}

// UsageRecord 用量记录
type UsageRecord struct {
    ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    SessionID    string    `json:"session_id"`
    User         string    `json:"user"`           // 从环境变量 $OPS_AI_USER / $USER 获取
    Team         string    `json:"team"`           // 从 ~/.ops-ai/config.yaml team 字段获取
    Model        string    `json:"model"`
    InputTokens  int       `json:"input_tokens"`
    OutputTokens int       `json:"output_tokens"`
    CostUSD      float64   `json:"cost_usd"`
    Operation    string    `json:"operation"`      // 本次调用的操作描述
}

// CostSummary 成本汇总
type CostSummary struct {
    Period       string             `json:"period"`        // e.g. "2026-06"
    TotalCost    float64            `json:"total_cost"`
    TotalTokens  int                `json:"total_tokens"`
    TotalCalls   int                `json:"total_calls"`
    ByUser       []UserCostBreakdown `json:"by_user"`
    ByTeam       []TeamCostBreakdown `json:"by_team"`
    ByModel      []ModelCostBreakdown `json:"by_model"`
}

type UserCostBreakdown struct {
    User    string  `json:"user"`
    Team    string  `json:"team"`
    Cost    float64 `json:"cost"`
    Calls   int     `json:"calls"`
    Tokens  int     `json:"tokens"`
}

type TeamCostBreakdown struct {
    Team   string  `json:"team"`
    Cost   float64 `json:"cost"`
    Users  int     `json:"users"`
    Calls  int     `json:"calls"`
}

type ModelCostBreakdown struct {
    Model   string  `json:"model"`
    Cost    float64 `json:"cost"`
    Calls   int     `json:"calls"`
}

// RecordUsage 记录一次 LLM 调用用量
func (ct *CostTracker) RecordUsage(record UsageRecord) error {
    // 异步写入 SQLite，不阻塞 Agent Loop
}

// GetSummary 获取指定周期的成本汇总
func (ct *CostTracker) GetSummary(period string) (*CostSummary, error) {
    // 支持 period: "2026-06", "2026-Q2", "2026"
}

// GetUserSummary 获取单个用户成本
func (ct *CostTracker) GetUserSummary(user string, period string) (*UserCostBreakdown, error)

// SetBudget 设置月度预算上限
func (ct *CostTracker) SetBudget(team string, monthlyLimitUSD float64) error

// CheckBudget 检查是否超出预算
func (ct *CostTracker) CheckBudget(ctx context.Context) (*BudgetStatus, error)

type BudgetStatus struct {
    Team         string  `json:"team"`
    Limit        float64 `json:"limit"`
    Current      float64 `json:"current"`
    Remaining    float64 `json:"remaining"`
    PercentUsed  float64 `json:"percent_used"`
    WarningSent  bool    `json:"warning_sent"`
}
```

## 30.4 成本配置（~/.ops-ai/config.yaml）

```yaml
# 用户/团队标识（用于成本归属）
identity:
  user: "zhangsan"           # 用户名
  team: "platform-sre"       # 团队名
  email: "zhangsan@corp.com" # 可选

# 成本控制
cost:
  budget:
    monthly_limit: 50.0       # 个人月度预算 (USD)
    warn_at_percent: 80       # 80% 时发送警告
    team_monthly_limit: 300.0 # 团队月度预算
  models:                     # 自定义模型定价（如果使用私有模型）
    custom-model:
      input_per_1k: 0.001
      output_per_1k: 0.002
```

## 30.5 TUI 集成

### `/cost` 命令

```
> /cost

  ═══════════════════════════════════════════════════════
  💰 成本报告 · 2026 年 6 月
  ═══════════════════════════════════════════════════════

  总览:
  ┌───────────────────────────────────────────────────────┐
  │ 总费用:       $38.42 USD                              │
  │ 总调用:       247 次                                  │
  │ 总 Token:     1,284,000 (输入) + 156,000 (输出)       │
  │ 日均:         $1.28                                   │
  │ 预算使用率:   76.8% ($38.42 / $50.00)  🟡            │
  └───────────────────────────────────────────────────────┘

  按模型:
  ┌───────────────────────────────────────────────────────┐
  │ Claude Sonnet:  $32.10  (195 calls, 83.5%)           │
  │ GPT-4o:         $5.82   (32 calls,  15.1%)           │
  │ Ollama (本地):  $0.00   (20 calls,   1.4%)           │
  └───────────────────────────────────────────────────────┘

  按团队:
  ┌───────────────────────────────────────────────────────┐
  │ platform-sre:   $25.10   (zhangsan, lisi, wangwu)    │
  │ payment-team:   $8.32    (zhaoliu)                   │
  │ data-infra:     $5.00    (sunqi)                     │
  └───────────────────────────────────────────────────────┘

  💡 预算提醒: 个人使用率 76.8%，月度还剩 $11.58。
  💡 输入 /cost --team 查看团队级明细。
```

### 成本警告（自动触发）

```
  ═══════════════════════════════════════════════════════
  💰 预算警告
  ═══════════════════════════════════════════════════════

  ⚠️  本月 LLM 使用费已达 $40.00 / $50.00 (80%)

  建议:
  • 简单查询（如 /health）可使用本地 Ollama 模型节省费用
  • 当前默认模型: Claude Sonnet ($3/1M input)
  • 切换到 Ollama: /model switch ollama/qwen2.5-coder:7b

  是否切换到本地模型？ [Y/n]
```

## 30.6 成本归属审计日志格式

```json
{
  "type": "cost",
  "timestamp": "2026-06-25T14:32:05Z",
  "session_id": "a3f2b1c4",
  "user": "zhangsan",
  "team": "platform-sre",
  "model": "claude-sonnet-4-20250514",
  "input_tokens": 3420,
  "output_tokens": 156,
  "cost_usd": 0.0105,
  "operation": "describe deployment payment"
}
```

---

# 第三十一部分：容器化部署模型

## 31.1 问题场景

v1.6 的 CI/CD 模式（§22）提供了 `--no-tui --yes`，但部署方式只有 `brew install` 和 `go install`。

真实痛点：企业的 CI/CD pipeline 99% 跑在容器里（GitHub Actions runner、GitLab CI runner、Jenkins agent、Argo Workflows）。没有 Dockerfile，运维想把 ops-ai 放进 pipeline 就需要自己写 Dockerfile、处理 CA 证书、处理 kubeconfig 注入——这些应该在产品层面解决。

## 31.2 设计目标

1. **提供官方 Dockerfile**——多阶段构建，distroless 基础镜像
2. **K8s CronJob 模板**——定期健康检查、定期 Config Drift 检测
3. **GitHub Actions / GitLab CI 集成示例**——开箱即用的 CI 模板
4. **kubeconfig 安全注入**——通过 K8s Secret / CI Secret 变量注入

## 31.3 Dockerfile

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=1.7.0" \
    -o /build/ops-ai ./cmd/ops-ai

# Stage 2: Runtime (distroless)
FROM gcr.io/distroless/static-debian12:nonroot

# 复制 CA 证书（HTTPS 连接需要）
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 复制二进制
COPY --from=builder /build/ops-ai /usr/local/bin/ops-ai

# 非 root 用户（distroless nonroot = uid 65532）
USER 65532:65532

# 默认命令：无交互模式
ENTRYPOINT ["/usr/local/bin/ops-ai"]
CMD ["--no-tui", "--yes", "--pipe"]
```

### 多架构构建

```bash
# 同时构建 amd64 + arm64
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/company/ops-ai:v1.7.0 \
  --push .
```

## 31.4 K8s 部署清单

### CronJob：定期健康检查

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: ops-ai-health-check
  namespace: ops-ai
spec:
  schedule: "*/10 * * * *"  # 每 10 分钟
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      backoffLimit: 2
      activeDeadlineSeconds: 120
      template:
        spec:
          serviceAccountName: ops-ai-readonly  # 使用只读 SA
          restartPolicy: Never
          containers:
          - name: ops-ai
            image: ghcr.io/company/ops-ai:v1.7.0
            args:
              - "--no-tui"
              - "--yes"
              - "--pipe"
              - "/health"
            env:
            - name: OPS_AI_USER
              value: "health-check-bot"
            - name: OPS_AI_TEAM
              value: "platform-sre"
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: ops-ai-secrets
                  key: api-key
            resources:
              requests:
                memory: "128Mi"
                cpu: "100m"
              limits:
                memory: "256Mi"
                cpu: "500m"
            securityContext:
              allowPrivilegeEscalation: false
              readOnlyRootFilesystem: true
              runAsNonRoot: true
              runAsUser: 65532
```

### CronJob：定期 Config Drift 检测

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: ops-ai-drift-check
  namespace: ops-ai
spec:
  schedule: "0 */2 * * *"  # 每 2 小时
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      backoffLimit: 1
      activeDeadlineSeconds: 300
      template:
        spec:
          serviceAccountName: ops-ai-readonly
          restartPolicy: Never
          containers:
          - name: ops-ai
            image: ghcr.io/company/ops-ai:v1.7.0
            args:
              - "--no-tui"
              - "--yes"
              - "--pipe"
              - "check config drift across all namespaces"
            env:
            - name: OPS_AI_USER
              value: "drift-check-bot"
            - name: OPS_AI_TEAM
              value: "platform-sre"
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: ops-ai-secrets
                  key: api-key
            resources:
              requests:
                memory: "128Mi"
                cpu: "100m"
              limits:
                memory: "512Mi"
                cpu: "500m"
            securityContext:
              allowPrivilegeEscalation: false
              readOnlyRootFilesystem: true
              runAsNonRoot: true
```

### RBAC：只读 ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ops-ai-readonly
  namespace: ops-ai
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ops-ai-readonly
rules:
- apiGroups: [""]
  resources: ["pods", "services", "nodes", "namespaces", "configmaps", "secrets", "events"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["cronjobs", "jobs"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["policy"]
  resources: ["poddisruptionbudgets"]
  verbs: ["get", "list"]
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list"]
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ops-ai-readonly
subjects:
- kind: ServiceAccount
  name: ops-ai-readonly
  namespace: ops-ai
roleRef:
  kind: ClusterRole
  name: ops-ai-readonly
  apiGroup: rbac.authorization.k8s.io
```

## 31.5 CI/CD 集成模板

### GitHub Actions

```yaml
# .github/workflows/k8s-health-check.yml
name: K8s Health Check

on:
  schedule:
    - cron: '*/30 * * * *'  # 每 30 分钟
  workflow_dispatch:         # 手动触发

jobs:
  health-check:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/company/ops-ai:v1.7.0
      credentials:
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
    steps:
      - name: K8s Health Check
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          OPS_AI_USER: "github-actions"
          OPS_AI_TEAM: "platform-sre"
          KUBECONFIG: /tmp/kubeconfig
        run: |
          echo "${{ secrets.KUBECONFIG }}" > /tmp/kubeconfig
          chmod 600 /tmp/kubeconfig
          ops-ai --no-tui --yes --pipe "/health" || exit $?

  drift-check:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/company/ops-ai:v1.7.0
    steps:
      - name: Config Drift Check
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          OPS_AI_USER: "github-actions"
          KUBECONFIG: /tmp/kubeconfig
        run: |
          echo "${{ secrets.KUBECONFIG }}" > /tmp/kubeconfig
          chmod 600 /tmp/kubeconfig
          ops-ai --no-tui --yes --pipe "check config drift"
```

### GitLab CI

```yaml
# .gitlab-ci.yml
ops-ai-health-check:
  image: ghcr.io/company/ops-ai:v1.7.0
  stage: monitor
  script:
    - echo "$KUBECONFIG" | base64 -d > /tmp/kubeconfig
    - chmod 600 /tmp/kubeconfig
    - export KUBECONFIG=/tmp/kubeconfig
    - ops-ai --no-tui --yes --pipe "/health"
  rules:
    - if: $CI_PIPELINE_SOURCE == "schedule"
  variables:
    OPS_AI_USER: "gitlab-ci"
    OPS_AI_TEAM: "platform-sre"
```

## 31.6 安全加固要点

| 约束 | 说明 |
|------|------|
| **distroless 基础镜像** | 无 shell / 无包管理器 / 最小攻击面 |
| **nonroot 用户** | uid 65532，禁止 privilege escalation |
| **只读根文件系统** | `readOnlyRootFilesystem: true` |
| **资源限制** | 防止 CI 任务耗尽节点资源 |
| **kubeconfig 通过 Secret/CI Secret 注入** | 绝不硬编码，绝不放在镜像中 |
| **API Key 通过 Secret 注入** | 同上 |
| **CronJob concurrencyPolicy: Forbid** | 防止堆积 |

---

# 第三十二部分：会话级爆炸半径控制

（设计已整合至 §5.14 安全网关。本节记录 Agent Loop 中的集成逻辑。）

## 32.1 Agent Loop 集成点

```go
func (a *Agent) executeWithBlastRadiusCheck(ctx context.Context, call *ToolCall) error {
    // 每次 L2+ 工具调用前，检查会话级爆炸半径
    if call.RiskLevel >= L2 {
        result := a.blastRadius.CheckBeforeExec(ctx, call)
        if !result.Allowed {
            a.tui.ShowBlastRadiusFuse(result)
            return fmt.Errorf("blast radius fuse: %s", result.Reason)
        }
    }
    
    // 执行操作
    err := a.executeTool(ctx, call)
    
    // 记录到爆炸半径追踪器
    if call.RiskLevel >= L2 {
        a.blastRadius.RecordExec(call)
        a.tui.UpdateBlastRadiusStatus(a.blastRadius.Status())
    }
    
    return err
}
```

## 32.2 TUI 状态栏集成

```
┌─ ops-ai ─── prod-cluster ─── ns: payment ─── ⚡2/3 L2 ─── 14:32:05 ─┐
```
`⚡2/3 L2` 含义：当前会话在 payment namespace 已执行 2 次 L2 操作，上限 3 次。

---

# 第三十三部分：GitOps 冲突感知与协调

（设计已整合至 §5.15 安全网关。本节记录与 GitOps CLI 工具的集成。）

## 33.1 Go 接口（补充）

```go
// GitOpsBridge 与 ArgoCD/Flux CLI 的桥接层
type GitOpsBridge struct {
    controller GitOpsController
}

// PauseSync 暂停 GitOps sync（用于紧急手动变更）
func (gb *GitOpsBridge) PauseSync(ctx context.Context, appName string) error {
    switch gb.controller {
    case GitOpsArgoCD:
        return gb.execArgoCD(ctx, "app", "set", appName, "--sync-policy", "none")
    case GitOpsFlux:
        return gb.execFlux(ctx, "suspend", "kustomization", appName)
    }
    return fmt.Errorf("unsupported controller: %s", gb.controller)
}

// ResumeSync 恢复 GitOps sync
func (gb *GitOpsBridge) ResumeSync(ctx context.Context, appName string) error {
    switch gb.controller {
    case GitOpsArgoCD:
        return gb.execArgoCD(ctx, "app", "set", appName, "--sync-policy", "automated")
    case GitOpsFlux:
        return gb.execFlux(ctx, "resume", "kustomization", appName)
    }
    return fmt.Errorf("unsupported controller: %s", gb.controller)
}
```

## 33.2 操作分级

| 操作 | 风险级别 | 说明 |
|------|---------|------|
| 检测 GitOps 管理状态 | L0 | 纯只读 |
| 暂停/恢复 sync | L2 | 需要确认，生产环境需要集群名确认 |
| 触发手动 sync | L1 | 同步到 Git 定义的状态（向安全方向变更） |
| 修改 GitOps 管理的资源 | L2 | 额外警告 + 回滚检测 |

---

# 第三十四部分：Agent Loop 全局超时与死循环防护

（设计已整合至 §8.6。本节记录 CLI 参数和配置。）

## 34.1 CLI 参数

```bash
# 全局超时
./ops-ai --timeout 10m "排查 payment 为什么慢"
./ops-ai --timeout 30s "快速检查 payment deployment 状态"

# 跳过死循环检测（调试用，不建议生产使用）
./ops-ai --no-deadloop-detect
```

## 34.2 死循环检测模式

| 模式 | 行为 |
|------|------|
| `warn` (默认) | 检测到重复模式 → 弹出警告 → 等待用户决定 |
| `strict` | 检测到重复模式 → 立即终止 Agent Loop |
| `off` | 不检测死循环（仅调试使用） |

---

# 第三十五部分：多集群上下文基础安全

## 35.1 问题场景

一个有 20+ 集群的 SRE，可能在不同 kubeconfig context 之间切换。没有上下文提示，误操作到错误集群的风险极高。

## 35.2 TUI 上下文展示

```
┌─ ops-ai ─── 🔴 prod-us (arn:aws:eks:us-west-2:123456:cluster/prod-us) ─── ns: payment ─── 14:32:05 ─┐
```

颜色方案：
- 🔴 红色：production 集群
- 🟡 黄色：staging 集群
- 🟢 绿色：dev / sandbox 集群
- ⚪ 灰色：未知/未分类

## 35.3 `/context` 命令

```
> /context

  当前上下文: prod-us  (arn:aws:eks:us-west-2:123456:cluster/prod-us)

  可用上下文:
  ┌──────────────────────────────────────────────────────────────┐
  │ 🔴 prod-us      arn:aws:eks:us-west-2:123456:cluster/prod   │
  │ 🔴 prod-eu      arn:aws:eks:eu-west-1:123456:cluster/prod    │
  │ 🟡 staging      arn:aws:eks:us-east-1:123456:cluster/staging │
  │ 🟢 dev-local     kind-dev-cluster                            │
  │ 🟢 sandbox       kind-sandbox                                │
  └──────────────────────────────────────────────────────────────┘

  切换: /context staging
```

## 35.4 刚切换上下文的缓冲期

刚切换到生产环境 5 分钟内，L2+ 操作额外要求确认：

```
  ⚠️  集群上下文最近切换: staging → prod-us (2 分钟前)
  你正在对生产集群执行 L2 操作。确认继续？
  输入集群名称确认: _
```

配置：

```yaml
# config.yaml
context:
  safety:
    switch_cooldown: 5m          # 切换后缓冲期
    require_name_confirm: true   # 缓冲期内 L2+ 需输入集群名
```

## 35.5 生产环境强认证（Phase 2 可选）

```yaml
context:
  production:
    # Phase 2: 生产环境可强制要求 MFA/审批
    require_mfa: false
    require_approval: false
```

---

# 第三十六部分：变更后自动验证

## 36.1 问题场景

`kubectl apply` 成功 ≠ 变更生效。典型盲区：

- ConfigMap 更新了但 Pod 没重启（旧配置仍在用）
- Deployment 滚动更新启动了，但新 Pod CrashLoopBackOff
- PVC 扩容了但 Pod 需要重启才能识别新容量
- HPA 修改了但没有足够时间观察效果

## 36.2 验证流程

```go
// PostOpValidator 变更后验证器
type PostOpValidator struct {
    clientset kubernetes.Interface
    config    PostOpValidationConfig
}

type PostOpValidationConfig struct {
    Enabled       bool          // 是否启用（default: true）
    CooldownSecs  int           // 变更后等待冷却期 (default: 10s)
    MaxRetries    int           // 最大验证重试次数 (default: 3)
    RetryInterval time.Duration // 重试间隔 (default: 10s)
}

// Validate 对单次操作执行变更后验证
func (v *PostOpValidator) Validate(ctx context.Context, op *ToolCallRecord) (*ValidationResult, error) {
    // 1. 冷却期：等待变更生效
    time.Sleep(time.Duration(v.config.CooldownSecs) * time.Second)
    
    // 2. 根据操作类型选择验证策略
    switch {
    case op.IsConfigMapChange():
        return v.validateConfigMapChange(ctx, op)
    case op.IsScaleChange():
        return v.validateScaleChange(ctx, op)
    case op.IsRolloutRestart():
        return v.validateRolloutRestart(ctx, op)
    default:
        return v.validateGenericChange(ctx, op)
    }
}
```

### 各操作类型的验证策略

```go
// ConfigMap 变更验证
func (v *PostOpValidator) validateConfigMapChange(ctx context.Context, op *ToolCallRecord) (*ValidationResult, error) {
    // 1. 检查 ConfigMap 内容是否已更新
    cm, _ := v.clientset.CoreV1().ConfigMaps(op.TargetResource.Namespace).Get(ctx, op.TargetResource.Name, metav1.GetOptions{})
    
    // 2. 关键：检查关联 Deployment 的 Pod 是否重启并使用新配置
    pods, _ := v.findPodsUsingConfigMap(ctx, op.TargetResource)
    
    notRestarted := []string{}
    for _, pod := range pods {
        if pod.CreationTimestamp.Time.Before(op.ExecutedAt) {
            notRestarted = append(notRestarted, pod.Name)
        }
    }
    
    if len(notRestarted) > 0 {
        return &ValidationResult{
            Status:   "warning",
            Message:  fmt.Sprintf("ConfigMap 已更新，但 %d 个 Pod 仍在使用旧配置: %s", len(notRestarted), strings.Join(notRestarted, ", ")),
            Action:   "建议执行 rollout restart 使新配置生效",
            CanAutoFix: true,
            AutoFixCmd: fmt.Sprintf("kubectl rollout restart deployment/%s", op.TargetResource.Name),
        }, nil
    }
    
    return &ValidationResult{Status: "pass"}, nil
}
```

### TUI 验证状态展示

```
  ✅ 变更已执行
  
  🔍 变更后验证 (10s 冷却期后):
  ┌──────────────────────────────────────────────────────────────┐
  │ deploy/payment-api: replicas 3 → 5                           │
  │   ✅ 5/5 Pods Running                                        │
  │   ✅ p99 延迟: 45ms → 32ms (恢复正常)                         │
  │ 变更生效: ✅ 已确认                                           │
  └──────────────────────────────────────────────────────────────┘

或:

  ⚠️  变更完成但验证发现问题:
  ┌──────────────────────────────────────────────────────────────┐
  │ ConfigMap payment-config: 已更新                              │
  │   ⚠️  3 个 Pod 仍在使用旧配置 (payment-7d9f8b-abc, ...)      │
  │ 建议: kubectl rollout restart deploy/payment-api           │
  │ [R] 自动执行 rollout restart  [I] 忽略  [X] 了解更多         │
  └──────────────────────────────────────────────────────────────┘
```

---

# 第三十七部分：依赖服务连通性诊断

## 37.1 问题场景

K8s 一切正常但应用 500 错误——最常见根因是依赖层问题：

- 数据库连接池满了（Pod Running 但连不上 DB）
- Redis 网络延迟或无响应
- 上游 API 超时
- 消息队列积压且未消费

传统排查需要运维手动 `kubectl exec` 进 Pod 测试这些连通性——ops-ai 应该自动做这件事。

## 37.2 `/health --deep` 命令

```
> /health --deep payment

  ═══════════════════════════════════════════════════════
  🔍 深度健康检查: payment namespace
  ═══════════════════════════════════════════════════════

  🟢 K8s 层:
  ┌───────────────────────────────────────────────────────┐
  │ deploy/payment-api  3/3 Running  ✅                  │
  │ deploy/payment-worker 2/2 Running ✅                 │
  │ svc/payment-api  ClusterIP 10.100.5.32  ✅            │
  └───────────────────────────────────────────────────────┘

  🟡 中间件层:
  ┌───────────────────────────────────────────────────────┐
  │ DB (RDS db-payment-prod):                             │
  │   连通性  ✅  延迟 2ms  连接数 45/200                │
  │ Redis (payment-cache-prod):                           │
  │   连通性  ✅  延迟 1ms  内存使用 62%                  │
  │ Kafka (payment-events):                               │
  │   连通性  ❌  consumer-group payment-worker lag: 12,847│
  │   ⚠️  消费延迟可能影响业务处理                          │
  └───────────────────────────────────────────────────────┘

  🔵 上游依赖:
  ┌───────────────────────────────────────────────────────┐
  │ Partner API (https://api.partner.com/health):         │
  │   可达性  ✅  HTTP 200  响应时间 120ms                │
  │ SMS Gateway (https://sms.provider.com/status):        │
  │   可达性  ❌  dial tcp: i/o timeout (10s)             │
  │   🔴  上游不可达! 检查安全组/防火墙规则                │
  └───────────────────────────────────────────────────────┘

  总结: K8s 层正常，但 Kafka consumer lag + SMS Gateway 不可达。
        建议优先排查 SMS Gateway 网络连通性。
```

## 37.3 实现策略

```go
// DependencyHealthChecker 依赖健康检查
type DependencyHealthChecker struct {
    dbProber    *DBProber           // DB 连接测试
    redisProber *RedisProber        // Redis 连接测试
    httpProber  *HTTPProber         // HTTP 可达性测试
    mqProber    *MQProber           // 消息队列检查
}

// CheckAll 从 Pod 视角检查所有依赖
func (c *DependencyHealthChecker) CheckAll(ctx context.Context, namespace string, deps []Dependency) (*DependencyHealth, error)
```

连接测试通过 `kubectl exec` 在 Pod 内执行（L1 白名单：nc, redis-cli PING, curl）：

```go
func (p *DBProber) Probe(ctx context.Context, podName, namespace, host, port string) (*ProbeResult, error) {
    // kubectl exec pod-name -- nc -zv host port -w 3
    cmd := fmt.Sprintf("nc -zv %s %s -w 3", host, port)
    return p.execInPod(ctx, podName, namespace, cmd)
}

func (p *RedisProber) Probe(ctx context.Context, podName, namespace, host, port string) (*ProbeResult, error) {
    // kubectl exec pod-name -- redis-cli -h host -p port PING
    cmd := fmt.Sprintf("redis-cli -h %s -p %s PING", host, port)
    return p.execInPod(ctx, podName, namespace, cmd)
}
```

## 37.4 依赖声明（方式一：annotation）

```yaml
apiVersion: v1
kind: Service
metadata:
  name: payment-api
  annotations:
    ops-ai.dependencies/db:     "rds:db-payment-prod:5432"
    ops-ai.dependencies/redis:  "redis:payment-cache-prod:6379"
    ops-ai.dependencies/http:   "https://api.partner.com/health"
```

## 37.5 配置（方式二：集中配置）

```yaml
# ~/.ops-ai/dependencies.yaml
payment:
  - type: db
    host: db-payment-prod.xxxx.us-east-1.rds.amazonaws.com
    port: 5432
    engine: postgres
  - type: redis
    host: payment-cache-prod.xxxx.cache.amazonaws.com
    port: 6379
  - type: http
    url: https://api.partner.com/health
    timeout: 5s
```

---

# 第三十八部分：集群内网络诊断

## 38.1 问题场景

K8s 网络问题是最难排查的运维痛点之一。Service A 调不通 Service B，根因可能在 NetworkPolicy、CoreDNS、kube-proxy、CNI 任何一环。

## 38.2 `/net-diag` 命令

```
> /net-diag deploy/payment-api → deploy/order-svc:8080

  ═══════════════════════════════════════════════════════
  🔍 网络连通性诊断: payment-api → order-svc:8080
  ═══════════════════════════════════════════════════════

  1. DNS 解析:
     order-svc.payment.svc.cluster.local
     → 10.100.5.32  ✅ (CoreDNS 正常)

  2. Service 端点:
     10.100.5.32:8080
     → [10.244.3.45:8080, 10.244.5.21:8080]  ✅ (2 个健康端点)

  3. NetworkPolicy 检查:
     ⚠️  order-svc 入站规则仅允许 frontend namespace
     ❌ payment namespace 流量被 NetworkPolicy "order-svc-allow" 阻断

  4. Pod 直连测试:
     10.244.3.45:8080 → 连通 ✅
     (Service 层可达，但 Pod 层被 NetworkPolicy 阻断)

  根因: NetworkPolicy 阻断跨 namespace 流量
  建议: 修改 NetworkPolicy order-svc-allow，添加 payment namespace 到 ingress rule
```

## 38.3 Go 接口

```go
// NetDiagRunner 网络诊断引擎
type NetDiagRunner struct {
    clientset       kubernetes.Interface
    dnsChecker      *DNSChecker
    svcEndpointChecker *ServiceEndpointChecker
    npChecker       *NetworkPolicyChecker
    podConnChecker  *PodConnectivityChecker
    cniDetector     *CNIDetector
}

type NetDiagRequest struct {
    Source      ResourceRef       // "deploy/payment-api"
    Target      ResourceRef       // "deploy/order-svc:8080" or "svc/order-svc"
    Protocol    string            // "tcp" | "udp" | "http"
    Port        int               // 8080
}

type NetDiagResult struct {
    Steps []NetDiagStep
    RootCause string
    Recommendation string
}

type NetDiagStep struct {
    Name    string               // "DNS Resolution"
    Status  string               // "pass" | "fail" | "warning" | "blocked"
    Detail  string
    Duration time.Duration
}

// DNS 检查：kubectl exec source-pod -- nslookup target-svc
// Service 端点：kubectl get endpoints target-svc
// NetworkPolicy：kubectl get networkpolicy -A | 分析匹配
// Pod 直连：kubectl exec source-pod -- nc -zv target-pod-ip port
// CNI 检测：kubectl get pods -n kube-system | grep -E "calico|cilium|flannel|weave"
```

## 38.4 常见诊断模式

| 模式 | 症状 | 诊断路径 | 常见根因 |
|------|------|---------|---------|
| DNS 解析失败 | `nslookup` 超时 | DNS → CoreDNS Pod → kube-dns Service | CoreDNS CrashLoopBackOff / kube-proxy 异常 |
| Service 无端点 | `get endpoints` 空 | Service selector → Pod labels | selector 不匹配 / Pod 未就绪 |
| NetworkPolicy 阻断 | Pod 直连通但 Service 不通 | NetworkPolicy ingress rules | 缺少 ingress allow rule |
| Pod 直连不通 | `nc -zv pod-ip port` 超时 | CNI → Node 路由 → Pod network ns | CNI 异常 / Node 网络分区 |
| 负载均衡不均 | 部分请求 502 | Service endpoints + iptables/IPVS | kube-proxy 规则未更新 |

---

# 第三十九部分：运维知识库集成（P2 预留）

## 39.1 设计概述（Phase 2）

运维知识库允许 ops-ai 复用历史排查经验，避免每次都"从零开始"。

```yaml
# ~/.ops-ai/knowledge/payment-oom.yaml
pattern: "payment-api OOMKilled"
root_cause: "JVM heap 不足，HPA 扩容速度跟不上流量 spike"
solution: "增加 initial heap 到 4G，配置 HPA 预热期缩短到 30s"
last_seen: 2026-06-20
tags: [payment, oom, jvm, hpa]
```

Agent 在排查时自动搜索知识库匹配的历史案例。

---



# 附录 A：一句话需求摘要（可直接贴 Jira/Linear）

**Epic**: 打造终端原生的运维 AI 副驾驶，Kubernetes + Terraform + Prometheus + Loki 跨工具链自然语言交互。

**核心差异化**：唯一做到"终端原生 + 跨工具链执行 + 五级安全护栏（含会话级爆炸半径控制 + GitOps 冲突感知）+ 告警自动排查 + 多云统一诊断 + 依赖层深度诊断"的运维 AI 工具。

**P0 功能清单（6-8 周）**：
1. Bubble Tea 终端对话 TUI（含多集群上下文安全展示）
2. K8s 全量操作（client-go, 精确命令列表见 §9.1）
3. 五级安全网关 + 环境上下文权重 + 会话级爆炸半径控制
4. 影响面预判（直接引用关联分析）
5. 自动快照 + 按资源类型差异化回滚
6. 多 LLM 后端（OpenAI/Claude/Ollama）
7. 操作审计日志（JSON Lines + 多 sink）
8. 集群健康概览（启动自动展示 + /health + /health --deep 依赖层诊断）
9. CI/CD 无交互模式（--no-tui --yes / --pipe）
10. 会话导出/导入（/export + /import，on-call 接力不丢上下文）
11. 云资源只读诊断 MVP（RDS/ALB/S3/Route53 L0 诊断）
12. alertd 告警接收守护进程（webhook + 路由规则 + 自动诊断）
13. **首次运行体验（启动自检 + /setup wizard，2 分钟开箱即用）**
14. **kubectl logs -f 流式日志（模式匹配终止 + 多 Pod 聚合）**
15. **Agent Loop 全局超时 + 死循环检测（5 分钟 deadline + 模式匹配）**
16. **容器化部署（Dockerfile + K8s CronJob 模板 + CI/CD 集成示例）**
17. **GitOps 冲突感知（ArgoCD/Flux 检测 + 回滚警告）**
18. **变更后自动验证（按操作类型差异化 + 冷却期）**
19. **集群内网络诊断（/net-diag 四层诊断）**

**目标用户**：5-50 人团队的 SRE/DevOps/平台工程师

**盈利模式**：开源 CLI 核心免费 → 企业版（审计/SSO/私有部署）付费

---

# 附录 B: Changelog

## v1.7 → v1.8（2026-06-25）— 生产安全版

补齐 7 项生产安全缺口（3 阻断 + 4 重要）：

| 变更项 | v1.7 | v1.8 |
|--------|------|------|
| **会话级爆炸半径控制** | 仅逐条安全网关，无聚合视角 | §5.14/§32 会话操作计数 + namespace 聚合 + 熔断机制 + TUI 展示 |
| **GitOps 冲突感知** | 仅检测 Config Drift，不处理冲突 | §5.15/§33 ArgoCD/Flux 管理检测 + 冲突警告 + pause/resume bridge |
| **Agent Loop 全局超时** | 仅步数限制，无时间 deadline | §8.6/§34 context.WithDeadline + 5m 默认 + 死循环检测 + 超时摘要 |
| **多集群上下文安全** | P2 预留，无设计 | §35 TUI 颜色编码 + 切换冷却期 + /context 命令 + 集群名确认 |
| **变更后自动验证** | 无验证，kubectl apply 成功=完成 | §36 按操作类型差异化 + 冷却期 + 自动检测 + 修复建议 |
| **依赖服务连通性诊断** | /health 仅 K8s 层 | §37 /health --deep + DB/Redis/MQ/HTTP 连通性 + 三层健康报告 |
| **集群内网络诊断** | 无 | §38 /net-diag + DNS→Service→NetworkPolicy→Pod 四层诊断 |

### 配套更新

| 更新项 | v1.7 | v1.8 |
|--------|------|------|
| P0 功能清单 | 16 项 | 19 项（+会话爆炸半径 +GitOps冲突 +Agent超时 +多集群安全 +网络诊断） |
| P1 Beta 功能 | Terraform/监控/CMDB/告警/云深度 + Pre-flight + 成本归属 | +多集群上下文基础安全 +变更后验证 +依赖诊断 +网络诊断 |
| P2 GA 功能 | 协作/编排/回滚/告警闭环 + 插件 | +运维知识库/Runbook 集成 |
| 风险表 | 38 项 | 46 项 |
| 安全网关 | 13 个检查点（§5.2-§5.13） | 15 个检查点（+§5.14 爆炸半径 +§5.15 GitOps） |
| 技术选型 | distroless + Docker Buildx + GHCR | 无新增外部依赖 |
| 目录结构 | +onboard/+preflight/+cost/+deploy | +blastradius/+gitops/+netdiag/+deps/+verify/+context |
| MVP 验收标准 | 16 条 | 23 条（+7 项生产安全验收） |
| 路线图 | W1-W8 含自检/checkpoint/pre-flight/流式日志/容器化 | +W1 多集群上下文 +W2 GitOps 检测 +W3 全局超时 +W4 变更验证 +W6-7 依赖诊断/网络诊断 |

**v1.8 定位变更**：产品可交付 → **生产安全版**。运维用 ops-ai 不会因为连续操作清空 namespace、不会被 GitOps 回滚困惑、不会排查超时后无上下文、不会对错误集群操作、不会改了配置但 Pod 没重启、不会不知道 DB 断了。从"能交付"到"敢用在生产"的最后闭环。

---

## v1.6 → v1.7（2026-06-25）— 产品可交付版

补齐 6 项产品交付阻断缺口（3 阻断 + 3 重要）：

| 变更项 | v1.6 | v1.7 |
|--------|------|------|
| **首次运行体验** | 无定义，配置缺失即 panic | §26 4 阶段启动自检 + 诊断建议 + /setup wizard 交互式配置 |
| **kubectl logs -f 流式日志** | 仅静态截断（--tail, --since） | §27 流式实时渲染 + 模式匹配终止 + 多 Pod 聚合 + 硬超时安全约束 |
| **Pre-flight 统一编排** | 零散独立函数，无协调/超时/降级 | §28 统一 PreFlightChecker 接口 + 并行执行 + 独立超时 + 降级策略矩阵 |
| **Agent Loop 崩溃恢复** | 崩溃即丢上下文 | §29 增量 checkpoint + 崩溃检测 + 自动会话恢复 + panic recovery wrapper |
| **用户/团队成本归属** | 仅有团队总成本预估 | §30 每次调用记录 token + 按用户/团队聚合 + 预算告警 + /cost 命令 |
| **容器化部署模型** | 仅 brew/go install | §31 Dockerfile (distroless) + K8s CronJob 模板 + GitHub Actions/GitLab CI 集成 |

### 配套更新

| 更新项 | v1.6 | v1.7 |
|--------|------|------|
| P0 功能清单 | 13 项 | 16 项 |
| P1 Beta 功能 | Terraform/监控/CMDB/告警/云资源深度 | +Pre-flight 统一编排 + 用户/团队成本归属 |
| P2 GA 功能 | 协作/编排/回滚/告警闭环 | +插件/扩展机制 + 多集群跨 context 桥接 |
| 风险表 | 33 项 | 38 项 |
| 技术选型 | AWS SDK 等 14 项 | +distroless + Docker Buildx + GHCR |
| 目录结构 | 标准包 + alertd + cloud | +onboard + preflight + cost + deploy/ |
| MVP 验收标准 | 12 条 | 16 条 |
| 路线图 W1/W3/W4/W7-8 | 原始里程碑 | +自检引擎 + checkpoint + pre-flight 编排 + 流式日志 + 容器化 |

**v1.7 定位变更**：企业可推广 → **产品可交付**。运维拿到二进制 → 2 分钟引导完成 → 排查过程中崩溃自动恢复 → 流式日志实时诊断 → 容器化部署进 pipeline → 月底 /cost 看团队账单。从"功能完整"到"体验完整"的最后一公里。

---

## v1.5 → v1.6（2026-06-25）— 企业可推广版

| 变更项 | v1.5 | v1.6 |
|--------|------|------|
| 会话共享与协作 | ❌ 两个运维接力排查时上下文完全丢失 | §23 会话导出/导入（/export /import）+ ParentSessionID 追溯链 + Phase 2 Live 协作 |
| 告警驱动自动触发 | ❌ 需要人工打开 ops-ai 手动排查 | §24 alertd 守护进程 + webhook 接口 + 告警路由规则引擎 + 自动诊断 + 结果回写 |
| 非K8s云资源诊断 | ❌ K8s 排障卡在云资源边界 | §25 CloudProvider 抽象 + AWS MVP (RDS/ALB/S3/Route53 L0 诊断) + K8s↔云资源映射 |
| 风险管理 | 30 项 | 33 项（新增会话接力/告警触发/云资源诊断 3 项风险 + 缓解） |
| P0 功能 | 11 项 | 13 项：增加会话导出/导入 + 云资源只读诊断 MVP |
| P1 功能 | 6 项（无变化，但范围扩展） | 告警 Webhook 触发 + 云资源深度诊断 纳入 P1 |
| 开放问题 | 6 项 | 7 项（新增云 Provider 扩展优先级问题） |

### 版本演进全景

| | v1.0 | v1.1 | v1.2 | v1.3 | v1.4 | v1.5 | **v1.6** | **v1.7** | **v1.8** |
|---|------|------|------|------|------|------|------|------|------|
| 定位 | 产品方向 | 可搭脚手架 | 可开始编码 | 可上线无意外 | 可交付使用 | 可企业部署 | 可全员推广 | **可产品交付** | **可生产安全** |
| 安全模型 | 雏形 | 五级+环境 | +Secret | +Webhook/PDB/PSA/Operator | +Quota/NS防呆/NetworkPolicy | +审计 | +告警自动排查安全护栏 | +Onboarding/Pre-flight/流式日志 | **+爆炸半径/GitOps/超时防护** |
| 开发就绪 | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 生产就绪 | ❌ | ❌ | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 企业可用 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 可部署性 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 用户信任 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 合规审计 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ |
| CI/CD 集成 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ |
| 开箱即用 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ |
| 团队协作 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** | ✅ | ✅ |
| 告警联动 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** | ✅ | ✅ |
| 多云诊断 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** | ✅ | ✅ |
| 容错自治 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** | ✅ |
| 生产安全 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** |

## v1.4 → v1.5（2026-06-25）— 企业可用版

| 变更项 | v1.4 | v1.5 |
|--------|------|------|
| 操作审计日志 | ❌ 仅本地 SQLite，无合规性 | §21 统一审计：JSON Lines 格式 + 多 sink（文件/Loki/ELK/webhook）+ 异步不阻塞 + fallback 兜底 |
| 集群健康概览 | ❌ 用户不知从何问起 | §9.8 启动自动展示 + /health 命令 + --overview flag + 5 维度并行检查 |
| CI/CD 无交互模式 | ❌ 无法集成 pipeline | §22 --no-tui --yes (L0-L2 自动确认) + --pipe (纯文本可消费) + 退出码语义化 |
| 风险管理 | 27 项 | 30 项（新增审计/CI/CD/健康概览 3 项风险 + 缓解） |
| MVP 验收 | 6 项 | 9 项：增加审计写入验证 + 健康概览展示 + CI 模式自动确认 |
| P0 功能 | 8 项 | 11 项：增加审计日志 + 健康概览 + CI/CD 模式 |

### 版本演进全景

| | v1.0 | v1.1 | v1.2 | v1.3 | v1.4 | **v1.5** |
|---|------|------|------|------|------|------|
| 定位 | 产品方向 | 可搭脚手架 | 可开始编码 | 可上线无意外 | 可交付使用 | **可企业部署** |
| 安全模型 | 雏形 | 五级+环境 | +Secret | +Webhook/PDB/PSA/Operator | +Quota/NS防呆/NetworkPolicy | +审计 |
| 开发就绪 | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 生产就绪 | ❌ | ❌ | ⚠️ | ✅ | ✅ | ✅ |
| 企业可用 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| 可部署性 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 用户信任 | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| 合规审计 | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** |
| CI/CD 集成 | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** |
| 开箱即用 | ❌ | ❌ | ❌ | ❌ | ❌ | **✅** |

## v1.3 → v1.4（2026-06-25）— 可交付版

| 变更项 | v1.3 | v1.4 |
|--------|------|------|
| RBAC 权限清单 | ❌ 完全缺失 | §20 ReadOnly/ReadWrite ClusterRole + 绑定示例 + 部署命令 + 安全建议 |
| --dry-run 预览模式 | ❌ 无 | §19 完整规划不执行 + TUI 标识 + 实现伪代码 |
| kubectl port-forward | ❌ 不支持 | §9.1.4 L1 支持 + 自动 curl 验证 + 30s TTL |
| ResourceQuota/LimitRange | ❌ 不感知 | §5.11 pre-flight 配额检查 + 超配额解释 |
| NetworkPolicy/Service Mesh | ❌ 不感知 | §5.12 网络策略感知 + Istio/Linkerd sidecar 检测 |
| ImagePullBackOff 诊断 | ❌ 笼统提示 | §9.6 六类错误分类表 + 自动排查链路 |
| 生产 namespace 确认 | ❌ 无主动防呆 | §5.13 L2+ 操作 namespace 输入确认 |
| kubectl cp 文件传输 | ❌ 不支持 | §9.1.5 按方向分级：拉出 L0 / 写入 L2 |
| kubectl edit 交互模型 | ❌ 未定义 | §9.1.6 外部 $EDITOR + 智能路由到 patch |
| K8s API 版本弃用 | ⚠️ 仅 CRD | §9.7 全量弃用映射 + 启动时 api-resources 缓存 |
| 风险管理 | 16 项 | 27 项（新增 11 项交付阻断风险 + 缓解） |
| MVP 验收 | 4 项 | 6 项：增加 --dry-run 全链路走通 + RBAC 自检通过 |

## v1.2 → v1.3（2026-06-25）— 生产集群实战版

| 变更项 | v1.2 | v1.3 |
|--------|------|------|
| Admission Webhook | 不感知，被拒后盲目重试 | §5.7 pre-flight 检查 + 拒绝原因解析 + 合规替代方案 |
| PDB 阻塞 | drain 时静默等待超时 | §5.8 pre-drain PDB 预检 + 阻塞预警 + 绕过建议 |
| PSA (Pod Security) | 不知道 namespace PSA label | §5.9 PSA level 感知 + 自动合规配置 + restricted 规则知识 |
| Operator 管控 | 直接修改被回滚不知原因 | §5.10 ownerReferences 检测 + CR 操作引导 |
| kubectl exec | L4 一刀切但缺 -it 说明 | §9.1.2 交互式终端引导（建议用户手动执行） |
| CRD API 版本 | 无感知 | §9.1.3 版本选择策略（v1 > v1beta1 > v1alpha1） |
| 节点智能诊断 | cordon/drain 操作，无诊断 | §9.4 完整诊断流程 + 5 类 conditions 分析 + 云/裸金属差异 |
| Config Drift | ArgoCD list 仅状态 | §9.5 ArgoCD/Flux diff + drift 报告 + 修复建议 |
| GitOps 场景 | "同步状态检查" | §9.5 ArgoCD/Flux 完整命令列表（L0-L2） |
| GPU 调度 | 不识别 GPU 资源 | §11.1 规则 8 GPU 容量检查 + 自然语言解释 |
| Cluster Policy 知识 | 仅 K8s 通用最佳实践 | §11.1 规则 5 注入 Webhook/PSA/PDB/Operator 感知 |
| 离线模式 | 完全依赖云 LLM，不可用 | §17 三层策略：自动降级 + 本地模型 + 离线安装包 |
| Prometheus/Loki 连接 | 未定义配置格式 | §18 YAML 配置 + mTLS/Bearer Token + 自动发现（Phase 2） |
| 风险管理 | 10 项 | 16 项（新增 6 项集群实战风险 + 缓解） |
| 开放问题 | 4 项 | 6 项（新增离线模型选型 + GitOps 集成深度） |

## v1.1 → v1.2（2026-06-25）

| 变更项 | v1.1 | v1.2 |
|--------|------|------|
| Secret 安全 | 仅一句 "values redacted by default" | §5.6 三层策略：LLM 层 redact + Agent 层内存闭环 + 审计层加密快照 |
| 上下文窗口管理 | 缺失 | §8.5 四级缓解：截断→摘要→压缩→硬上限 + TUI 实时预算展示 |
| kubectl exec | L4 一刀切禁止 | §9.1.1 白名单策略：诊断命令 L1、可确认 L2、破坏性 L4 |
| Helm 集成 | 未决策 | §9.3 分阶段混合：只读 SDK + 写操作 exec 二进制+白名单 |
| 测试策略 | "kind 集成测试" | §14 四层金字塔：单元 → 行为模拟 → 集成 → E2E + 安全回归套件 |
| LLM 成本控制 | 缺失 | §15 四策略：本地优先路由 + TUI 展示 + 月度预算 + 减少冗余 |
| MVP 验收标准 | 1 项功能验收 | 4 项：功能 + 安全回归 + 覆盖率 + Secret redact |
| 开发路线图 | W6-7 "kind 集成测试" | W6-7 明确测试量级：100+ 单元 / 40+ 行为模拟 / 安全回归 |

## v1.0 → v1.1（2026-06-25）

| 变更项 | v1.0 | v1.1 |
|--------|------|------|
| 安全分级 | 固定五级，无环境上下文 | 操作固有风险 + 环境上下文权重（dev -1, prod +1, prod+核心服务 +2） |
| 安全分类错误 | terraform plan 误归类为 L2 | 修正为 L0（纯只读） |
| 影响面分析 | "展示受影响的服务" | 完整伪代码：直接引用关联分析（ConfigMap/Secret→Deployment→Svc→Ingress） |
| 回滚模型 | 所有资源统一 kubectl apply snapshot | 按资源类型差异化：Helm → helm rollback, StatefulSet → apply + rollout restart, PVC → 仅规格+警告, ConfigMap/Secret → apply + 提示重启 Pod |
| Agent Loop | 未定义 | 完整 6 步循环、终止条件、并行规则、错误处理分级 |
| K8s 命令列表 | "50+ 子命令" | 精确列表（§9.1）：L0 17 种、L1 4 种、L2 10 种、L3 3 种、L4 禁区清单 |
| K8s Client | 未决策 | 决策：client-go（非 exec kubectl），含理由矩阵 |
| Go 接口 | TypeScript 伪代码 | 完整 Go struct 和 interface：Tool, SafetyGate, Agent, LLMClient, ToolContext, ToolResult, DryRunResult, SnapshotResult |
| System Prompt | 缺失 | 完整 9 条规则模板 + 动态变量注入 |
| 运维场景覆盖 | 3 个场景 | 9 个分类场景完整列表（§3.3） |
| 项目结构 | 未定义 | 完整 MVP 目录树（§10.2） |
| 技术选型 | 概括性推荐 | 精确到版本号（§13） |

---

*文档结束 — v1.8 生产安全版。运维拿到二进制 → 2 分钟引导完成 → 上下文安全指示 → 排查不炸 namespace → GitOps 不打架 → 超时自动保存进度 → 改完自动验证 → /net-diag 秒定位网络问题 → 月底 /cost 看账单。敢用在生产环境了。*
