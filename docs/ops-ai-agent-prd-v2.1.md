# 运维 AI Agent 产品需求文档 (PRD) v2.1

> **文档用途**：面向产品设计师、架构师、开发团队的完整需求规格说明
> **版本**：v2.1 — 全栈运维版（补齐多集群、GitOps、容器运行时、链路追踪、运行时安全、云厂商、中间件、SLO、GPU、Serverless、Vault、CNI、Ingress、内核诊断、弹性体系。从"规模化生产可靠"到"全栈深度运维"。）
> **日期**：2026-06-25
> **变更**：v2.0 → v2.1 补齐了 v2.0 运维视角差距分析中的 15 项缺口（3 P0 + 5 P1 + 7 P2），详见末尾 Changelog
> **前置文档**：本 PRD 基于 v2.0 全部功能（§1-§69）进行增量扩展

---

# 第一部分：给所有人的 Executive Summary

## 我们要做什么

v2.0 实现了"Agent 规模化生产可靠"——高可用、自升级、大规模性能、服务网格、发布策略、变更窗口、告警降噪等 12 项缺口全部补齐。v2.1 的核心目标是实现**全栈深度运维**的最后一层覆盖：

1. **"Agent 必须懂多集群和混合云"** — 多集群管理、跨集群诊断、混合云统一运维
2. **"Agent 必须懂现代交付模式"** — GitOps（ArgoCD/Flux）、Helm/Kustomize 深度诊断
3. **"Agent 必须覆盖基础设施全栈"** — 容器运行时、CNI、内核、云厂商、弹性体系
4. **"Agent 必须懂应用全栈"** — 链路追踪、中间件、Serverless、GPU
5. **"Agent 必须懂安全与合规"** — 运行时安全、Vault、SLO/错误预算

## 一句话定位

> **v2.0 让 Agent 规模化生产可靠；v2.1 让 Agent 全栈深度运维。**

## v2.1 与 v2.0 的关系

v2.1 是 v2.0 的**增量扩展**。所有 v2.0 的高可用、自升级、性能优化、服务网格、发布策略、变更窗口、告警降噪均保持不变。v2.1 新增第 70-84 部分，解决 v2.0 差距分析中的 15 项缺口。

---

# 第二部分：差距分析 → 解决方案映射

| 优先级 | 编号 | 缺口 | 核心风险 | 解决方案 |
|--------|------|------|----------|----------|
| **P0** | 1 | 多集群/混合云管理 | 企业 3-10 个集群，Agent 只能连一个 | §70 |
| **P0** | 2 | GitOps（ArgoCD/Flux）深度运维 | 80%+ 团队用 GitOps，同步失败高频 on-call | §71 |
| **P0** | 3 | 容器运行时深度诊断 | ContainerCreating 是最常见的 Pod 异常 | §72 |
| **P1** | 4 | 分布式链路追踪 | 微服务故障没有链路追踪无法定位 | §73 |
| **P1** | 5 | 运行时安全（Falco/seccomp） | 安全合规是必选项 | §74 |
| **P1** | 6 | 云厂商深度集成 | Agent 部署在云上，云资源故障无法排查 | §75 |
| **P1** | 7 | 中间件/有状态服务深度运维 | Redis/Kafka/ES 是 K8s 最常见有状态服务 | §76 |
| **P1** | 8 | SRE/SLO/错误预算管理 | SRE 文化的核心 | §77 |
| **P2** | 9 | GPU/异构计算运维 | AI/ML 负载快速增长 | §78 |
| **P2** | 10 | Serverless/Knative 支持 | 部分团队使用 | §79 |
| **P2** | 11 | Vault/密钥管理深度集成 | 企业级安全基线 | §80 |
| **P2** | 12 | CNI 深度诊断 | 网络问题占 K8s 运维 30%+ | §81 |
| **P2** | 13 | Ingress/云负载均衡器深度运维 | 504/502 是最常见的用户侧报错 | §82 |
| **P2** | 14 | 内核/OS 级诊断 | 节点级问题的底层根因 | §83 |
| **P2** | 15 | 弹性体系（CA/Karpenter）深度 | 流量突增时不扩容是 P0 事故 | §84 |

---

# 第三部分：P0 — 阻断级（不解决无法进入现代运维体系）

---

## 70. 多集群与混合云统一管理（v2.1 新增）

### 70.1 问题场景

v2.0 全文**零匹配** "多集群/联邦/混合云"。在中大型企业中，这是不可接受的架构缺口：

- 生产环境通常有 3-10 个集群（prod-us, prod-eu, prod-asia, staging, dev, dr, edge）
- Agent 每次只能连接一个 kubeconfig 上下文，跨集群问题需要手动切换
- Service Mesh 多集群联邦（Istio Multi-Primary、Linkerd Multi-Cluster）故障无法跨集群诊断
- **实际运维场景**："payment 服务在 prod-us 集群正常，在 prod-eu 频繁超时。需要对比两个集群的 Pod 状态、ConfigMap、网络策略差异"

### 70.2 设计目标

- 多 kubeconfig 上下文管理：同时注册多个集群
- 跨集群资源对比：同一服务在不同集群的状态差异
- 多集群批量操作：在多个集群上执行相同诊断
- 集群间流量拓扑：Service Mesh 跨集群流量可视化

### 70.3 Go 接口定义

```go
// MultiClusterManager 多集群管理器
type MultiClusterManager struct {
    clusters    map[string]*ClusterContext
    current     string
    config      MultiClusterConfig
}

type ClusterContext struct {
    Name        string
    Kubeconfig  string
    ContextName string
    Region      string
    Environment string  // prod | staging | dev
    Labels      map[string]string
    Client      kubernetes.Interface
    MeshClient  *ServiceMeshClient  // 可选
}

// ClusterSwitcher 集群切换器
type ClusterSwitcher struct{}

func (s *ClusterSwitcher) Switch(clusterName string) error {
    // 切换当前 kubeconfig 上下文
}

// CrossClusterDiff 跨集群差异检测
type CrossClusterDiff struct {
    clusters []string
}

type ResourceDiffResult struct {
    ResourceType   string
    ResourceName   string
    Namespace      string
    Clusters       map[string]interface{}  // cluster -> resource state
    Differences    []FieldDiff
    Consistent     bool
}

// MultiClusterExecutor 多集群执行器
type MultiClusterExecutor struct {
    clusters []string
}

func (e *MultiClusterExecutor) Execute(ctx context.Context, operation func(kubernetes.Interface) error) map[string]error {
    results := make(map[string]error)
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    for _, clusterName := range e.clusters {
        wg.Add(1)
        go func(name string) {
            defer wg.Done()
            client := getClusterClient(name)
            err := operation(client)
            mu.Lock()
            results[name] = err
            mu.Unlock()
        }(clusterName)
    }
    
    wg.Wait()
    return results
}
```

### 70.4 多集群注册与切换

```bash
ops-ai cluster add prod-us --kubeconfig ~/.kube/prod-us --region us-east-1 --env prod
ops-ai cluster add prod-eu --kubeconfig ~/.kube/prod-eu --region eu-west-1 --env prod
ops-ai cluster add staging --kubeconfig ~/.kube/staging --region us-east-1 --env staging
ops-ai cluster list
ops-ai cluster switch prod-us
```

### 70.5 跨集群差异检测

```
ops-ai cluster diff --clusters prod-us,prod-eu --resource deployment/payment-api --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  🌐  跨集群差异检测 — payment-api
  ═══════════════════════════════════════════════════════

  资源: deployment/payment-api (namespace: payment)

  差异列表
  ─────────────────────────────────────────────────────
  字段                  prod-us              prod-eu              状态
  ─────────────────────────────────────────────────────
  replicas              5                    3                    ❌
  image                 v2.4.0               v2.3.1               ❌
  memory limit          1Gi                  512Mi                ❌
  env.PAYMENT_TIMEOUT   5000                 3000                 ⚠️
  serviceAccount        payment-api          payment-api          ✅

  诊断结论
  ─────────────────────────────────────────────────────
  prod-eu 落后于 prod-us：
  - 镜像版本低 1 个小版本（v2.3.1 vs v2.4.0）
  - 副本数少 2 个（3 vs 5）
  - 内存限制只有一半（512Mi vs 1Gi）

  这可能导致 prod-eu 性能问题和 OOM 风险。

  [S] 同步到 prod-eu（L3，需确认）  [E] 导出报告  [Q] 返回
```

### 70.6 多集群批量诊断

```
ops-ai cluster exec --clusters prod-us,prod-eu,prod-asia -- ops-ai node status

执行结果:
prod-us:   ✅ 所有节点健康
prod-eu:   ⚠️  worker-03 NotReady（磁盘压力）
prod-asia: ✅ 所有节点健康
```

### 70.7 配置项

```yaml
# ~/.ops-ai/config.yaml
multi_cluster:
  enabled: true
  
  # 集群注册
  clusters:
    - name: "prod-us"
      kubeconfig: "~/.kube/prod-us"
      context: "prod-us"
      region: "us-east-1"
      env: "prod"
      labels:
        mesh: "enabled"
        
    - name: "prod-eu"
      kubeconfig: "~/.kube/prod-eu"
      context: "prod-eu"
      region: "eu-west-1"
      env: "prod"
      
  # 默认集群
  default_cluster: "prod-us"
  
  # 跨集群诊断
  cross_cluster:
    max_concurrent: 5                   # 最大并发诊断集群数
    timeout: "30s"
```

### 70.8 System Prompt 补充

```
## 多集群管理知识

当运维在多个集群环境中排查问题时：

1. **集群切换**：
   - 使用 `ops-ai cluster switch <name>` 切换当前上下文
   - 所有后续操作在切换后的集群上执行

2. **跨集群差异**：
   - 使用 `ops-ai cluster diff` 对比同一资源在不同集群的状态
   - 重点关注镜像版本、副本数、资源配置、环境变量差异

3. **批量诊断**：
   - 使用 `ops-ai cluster exec` 在多个集群上执行相同诊断
   - 结果汇总展示，便于快速定位异常集群

4. **Service Mesh 多集群**：
   - 如果集群启用了多集群联邦，自动检测跨集群流量
   - 诊断跨集群服务发现的连通性
```

---

## 71. GitOps 深度运维 — ArgoCD/Flux/Helm/Kustomize（v2.1 新增）

### 71.1 问题场景

v2.0 **零匹配** "ArgoCD/Flux/Helm/Kustomize"。在 GitOps 成为标配的时代，这是严重缺失：

- ArgoCD Application 同步失败、健康检查异常、OutOfSync 是高频 on-call
- Helm Chart 版本冲突、values 配置错误、依赖管理问题
- Kustomize overlay 合并错误、patch 应用失败
- **实际运维场景**："ArgoCD 显示 payment-app 为 Degraded，但 kubectl get pods 全部 Running。问题可能在 ArgoCD 健康检查脚本或 resource hooks"

### 71.2 设计目标

- ArgoCD Application 状态诊断和同步辅助
- Flux Kustomization/HelmRelease 状态诊断
- Helm Chart 依赖冲突检测
- Kustomize overlay 诊断和渲染预览

### 71.3 Go 接口定义

```go
// GitOpsManager GitOps 管理器
type GitOpsManager struct {
    k8sClient      kubernetes.Interface
    argoClient     argocd.Interface
    fluxClient     fluxcd.Interface
    helmClient     helm.Interface
    config         GitOpsConfig
}

// ArgoCDDiagnostics ArgoCD 诊断
type ArgoCDDiagnostics struct {
    argoClient argocd.Interface
}

type ArgoCDAppStatus struct {
    Name        string
    Namespace   string
    SyncStatus  string  // Synced | OutOfSync
    HealthStatus string // Healthy | Degraded | Missing | Progressing | Suspended
    SyncError   string
    Resources   []ArgoCDResource
    LastSync    time.Time
    Issues      []string
}

type ArgoCDResource struct {
    Kind      string
    Name      string
    Namespace string
    Status    string
    Health    string
    Diff      string
}

// FluxDiagnostics Flux 诊断
type FluxDiagnostics struct {
    fluxClient fluxcd.Interface
}

type FluxKustomizationStatus struct {
    Name      string
    Namespace string
    Ready     bool
    Status    string
    Message   string
    LastAppliedTime time.Time
    Issues    []string
}

// HelmDiagnostics Helm 诊断
type HelmDiagnostics struct {
    helmClient helm.Interface
}

type HelmChartStatus struct {
    ReleaseName string
    Namespace   string
    Chart       string
    Version     string
    Status      string  // deployed | failed | pending
    Values      map[string]interface{}
    Issues      []string
}

// KustomizeDiagnostics Kustomize 诊断
type KustomizeDiagnostics struct{}

type KustomizeRenderResult struct {
    BasePath   string
    Overlays   []string
    Resources  []string
    Errors     []string
    Warnings   []string
}
```

### 71.4 ArgoCD Application 诊断

```
ops-ai gitops argo status payment-app -n argocd
```

```
  ═══════════════════════════════════════════════════════
  🔄  ArgoCD Application 诊断 — payment-app
  ═══════════════════════════════════════════════════════

  同步状态: ❌ OutOfSync
  健康状态: 🟡 Degraded
  上次同步: 2024-06-25 08:00:00

  资源状态
  ─────────────────────────────────────────────────────
  资源                          状态      健康      差异
  ─────────────────────────────────────────────────────
  Deployment/payment-api        Synced    Healthy   ✅
  Service/payment-api           Synced    Healthy   ✅
  ConfigMap/payment-config      Synced    Healthy   ✅
  Ingress/payment-api           OutOfSync Missing   ❌

  问题分析
  ─────────────────────────────────────────────────────
  Ingress/payment-api 在 Git 仓库中存在，但集群中缺失。
  可能原因:
  1. Ingress 资源被手动删除
  2. Ingress 的 admission webhook 拒绝创建（如缺少 required 注解）
  3. Ingress 依赖的 Secret（TLS 证书）不存在

  建议行动
  ─────────────────────────────────────────────────────
  1. 检查 ingress-nginx admission controller 日志
  2. 确认 TLS Secret 存在
  3. 尝试手动同步: argocd app sync payment-app

  [S] 手动同步（L2）  [D] 查看详情  [Q] 返回
```

### 71.5 Helm Chart 诊断

```
ops-ai gitops helm status payment-api -n payment
```

```
  ═══════════════════════════════════════════════════════
  📦  Helm Release 诊断 — payment-api
  ═══════════════════════════════════════════════════════

  Release: payment-api
  Chart:   payment-api-2.4.0
  Status:  ⚠️  failed

  失败原因
  ─────────────────────────────────────────────────────
  Error: template: payment-api/templates/ingress.yaml:15:28:
         executing "payment-api/templates/ingress.yaml" at <.Values.ingress.host>:
         nil pointer evaluating interface {}.host

  诊断结论
  ─────────────────────────────────────────────────────
  values.yaml 中缺少 ingress.host 配置项。

  修复建议
  ─────────────────────────────────────────────────────
  在 values.yaml 中添加:
    ingress:
      host: "api.payment.company.com"

  [F] 修复并重新部署（L3）  [Q] 返回
```

### 71.6 Kustomize 渲染预览

```
ops-ai gitops kustomize render --path ./overlays/prod
```

```
  ═══════════════════════════════════════════════════════
  🎨  Kustomize 渲染预览 — overlays/prod
  ═══════════════════════════════════════════════════════

  基础路径: base/
  覆盖层:   overlays/prod

  应用资源 (3)
  ─────────────────────────────────────────────────────
  1. Deployment/payment-api
     补丁: replicas 3 → 5
           resources.memory 512Mi → 1Gi
           
  2. Service/payment-api
     无变更
     
  3. ConfigMap/payment-config
     新增键: ENV=production

  错误和警告
  ─────────────────────────────────────────────────────
  ⚠️  Warning: 未找到 base/ingress.yaml 的 patch 目标
     建议: 检查 kustomization.yaml 中的 patches 配置

  [A] 应用渲染结果（L3）  [Q] 返回
```

### 71.7 L0 命令扩展

```
ops-ai gitops argo list [-n <ns>]           # 列出 ArgoCD Applications
ops-ai gitops argo status <app>             # ArgoCD Application 状态
ops-ai gitops argo sync <app>               # 手动同步（L2）
ops-ai gitops flux list                     # 列出 Flux Kustomizations
ops-ai gitops flux status <kustomization>   # Flux 状态
ops-ai gitops helm list [-n <ns>]           # 列出 Helm Releases
ops-ai gitops helm status <release>         # Helm Release 状态
ops-ai gitops kustomize render --path <p>   # Kustomize 渲染预览
```

### 71.8 配置项

```yaml
# ~/.ops-ai/config.yaml
gitops:
  enabled: true
  
  argocd:
    namespace: "argocd"
    api_server: "https://argocd-server.argocd.svc"
    
  flux:
    namespace: "flux-system"
    
  helm:
    history_max: 10                     # 保留 10 个历史版本
    
  kustomize:
    path: "./kustomize"                 # 默认 kustomize 路径
```

### 71.9 System Prompt 补充

```
## GitOps 运维知识

当运维处理 GitOps 相关问题时：

1. **ArgoCD**：
   - OutOfSync = 集群状态与 Git 仓库不一致
   - Degraded = 资源存在但健康检查失败
   - Missing = 资源在 Git 中定义但在集群中不存在
   - 健康检查脚本是自定义的，可能误判

2. **Helm**：
   - template 错误通常因为 values.yaml 缺少必需字段
   - 升级前使用 helm diff 预览变更
   - 依赖 chart 版本冲突需要更新 Chart.lock

3. **Kustomize**：
   - patch 目标不存在时会静默忽略（需注意）
   - 使用 `ops-ai gitops kustomize render` 预览渲染结果
   - overlay 合并顺序影响最终结果
```

---

## 72. 容器运行时深度诊断（v2.1 新增）

### 72.1 问题场景

v2.0 **零匹配** "containerd/CRI-O/运行时"。ContainerCreating 是 K8s 最常见的 Pod 异常状态，根因通常在运行时层：

- 镜像拉取失败（containerd 镜像仓库配置、认证、网络）
- OCI runtime 创建失败（seccomp/AppArmor 阻止、cgroups 配置错误）
- 沙箱创建失败（CNI 未就绪、IP 分配耗尽）
- **实际运维场景**："Pod 一直处于 ContainerCreating，describe 显示 'Failed to create shim task: OCI runtime create failed: container_linux.go:380: starting container process caused: apply caps: operation not permitted'"

### 72.2 设计目标

- containerd/CRI-O 状态检查和健康诊断
- 镜像拉取失败根因分析
- OCI runtime 错误诊断
- 运行时日志收集和分析

### 72.3 Go 接口定义

```go
// ContainerRuntimeManager 容器运行时管理器
type ContainerRuntimeManager struct {
    k8sClient     kubernetes.Interface
    runtimeType   string  // containerd | cri-o | docker
    nodeClient    *NodeRuntimeClient
}

// RuntimeStatusChecker 运行时状态检查器
type RuntimeStatusChecker struct {
    k8sClient kubernetes.Interface
}

type RuntimeStatus struct {
    NodeName       string
    RuntimeType    string
    RuntimeVersion string
    CRIStatus      string  // Ready | NotReady
    ContainerCount int
    ImageCount     int
    Issues         []RuntimeIssue
}

type RuntimeIssue struct {
    Type        string  // image-pull-failed | oci-runtime-error | cni-error | sandbox-error
    Severity    string
    Description string
    Suggestion  string
}

// ImagePullDiagnoser 镜像拉取诊断器
type ImagePullDiagnoser struct {
    k8sClient kubernetes.Interface
}

type ImagePullResult struct {
    Image       string
    Node        string
    Status      string  // success | failed | pending
    Error       string
    RootCause   string
    Suggestions []string
}

// OCIRuntimeDiagnoser OCI 运行时诊断器
type OCIRuntimeDiagnoser struct {
    k8sClient kubernetes.Interface
}

type OCIRuntimeError struct {
    ErrorMessage string
    ExitCode     int
    RootCause    string
    Suggestions  []string
}
```

### 72.4 运行时状态检查

```
ops-ai runtime status
ops-ai runtime status --node worker-01
```

```
  ═══════════════════════════════════════════════════════
  🐳  容器运行时状态 — worker-01
  ═══════════════════════════════════════════════════════

  运行时: containerd v1.7.2
  CRI 状态: ✅ Ready
  容器数:  45
  镜像数:  128

  镜像仓库状态
  ─────────────────────────────────────────────────────
  registry.company.com       ✅ 正常
  docker.io                  ⚠️  限速（429 Too Many Requests）
  gcr.io                     ❌  超时（网络不通）

  问题检测
  ─────────────────────────────────────────────────────
  ⚠️  docker.io 达到拉取速率限制
  影响: 从 docker.io 拉取的镜像会失败或延迟
  建议: 配置镜像仓库代理或缓存（如 Harbor）

  [Q] 返回
```

### 72.5 Pod 创建失败诊断

```go
func (d *OCIRuntimeDiagnoser) DiagnosePodCreationFailure(ctx context.Context, podName, namespace string) (*RuntimeIssue, error) {
    pod, err := d.k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }
    
    // 检查 Pod 事件
    events, _ := d.k8sClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
        FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
    })
    
    for _, event := range events.Items {
        if event.Reason == "FailedCreatePodSandBox" || event.Reason == "Failed" {
            // 分析错误消息
            if strings.Contains(event.Message, "OCI runtime create failed") {
                return d.analyzeOCIError(event.Message)
            }
            if strings.Contains(event.Message, "ImagePullBackOff") {
                return d.analyzeImagePullError(pod)
            }
            if strings.Contains(event.Message, "Failed to create shim task") {
                return d.analyzeShimError(event.Message)
            }
        }
    }
    
    return nil, nil
}

func (d *OCIRuntimeDiagnoser) analyzeOCIError(msg string) *RuntimeIssue {
    if strings.Contains(msg, "operation not permitted") {
        return &RuntimeIssue{
            Type:        "oci-runtime-error",
            Severity:    "critical",
            Description: "OCI runtime 权限不足",
            Suggestion:  "检查 seccomp/AppArmor 配置，或确认容器不需要特权能力",
        }
    }
    if strings.Contains(msg, "no such file or directory") {
        return &RuntimeIssue{
            Type:        "oci-runtime-error",
            Severity:    "high",
            Description: "容器启动命令或文件不存在",
            Suggestion:  "检查 Dockerfile 中的 ENTRYPOINT/CMD 是否正确",
        }
    }
    return &RuntimeIssue{
        Type:        "oci-runtime-error",
        Severity:    "high",
        Description: msg,
        Suggestion:  "查看节点上的 containerd/cri-o 日志获取详细信息",
    }
}
```

### 72.6 L0 命令扩展

```
ops-ai runtime status [--node <node>]       # 运行时状态
ops-ai runtime diagnose <pod> [-n <ns>]     # Pod 创建失败诊断
ops-ai runtime images --node <node>         # 节点镜像列表
ops-ai runtime logs --node <node>           # 运行时日志
```

### 72.7 配置项

```yaml
# ~/.ops-ai/config.yaml
container_runtime:
  enabled: true
  
  # 自动检测运行时类型
  auto_detect: true
  
  # 镜像仓库
  image_registries:
    - "registry.company.com"
    - "docker.io"
    
  # 镜像缓存
  image_cache:
    enabled: true
    type: "harbor"                    # harbor | docker-registry
    url: "https://harbor.company.com"
    
  # 运行时日志
  runtime_logs:
    enabled: true
    containerd_log_path: "/var/log/containers/containerd.log"
    cri_o_log_path: "/var/log/crio/crio.log"
```

### 72.8 System Prompt 补充

```
## 容器运行时诊断知识

当 Pod 处于 ContainerCreating 或无法启动时：

1. **常见错误**：
   - ImagePullBackOff → 镜像不存在、认证失败、网络不通
   - CrashLoopBackOff → 应用启动失败（查看容器日志）
   - OCI runtime create failed → seccomp/AppArmor 阻止、权限不足
   - Failed to create pod sandbox → CNI 未就绪、IP 分配耗尽

2. **运行时状态**：
   - containerd/cri-o 状态可以通过 `ops-ai runtime status` 查看
   - 关注镜像仓库的健康状态

3. **镜像拉取**：
   - docker.io 有速率限制，建议配置镜像缓存
   - 私有仓库需要 imagePullSecret
```

---

# 第四部分：P1 — 重要级（现代应用架构必需）

---

## 73. 分布式链路追踪与 APM 集成（v2.1 新增）

### 73.1 问题场景

v2.0 **零匹配** "OpenTelemetry/Jaeger/链路追踪"。微服务架构下没有链路追踪几乎无法定位跨服务问题：

- 一个 API 请求经过 5-10 个微服务，哪个环节引入了延迟？
- 错误率上升时，是上游服务问题还是下游数据库问题？
- **实际运维场景**："用户报告下单接口偶尔 5 秒超时，涉及 order-api、payment-service、inventory-service、notification-service，需要链路追踪定位瓶颈"

### 73.2 设计目标

- OpenTelemetry/Jaeger/Zipkin 集成
- 链路查询和可视化
- 延迟/错误率按服务维度分析
- 慢链路自动检测

### 73.3 Go 接口定义

```go
// TracingManager 链路追踪管理器
type TracingManager struct {
    jaegerClient  *JaegerClient
    otelClient    *OTelClient
    config        TracingConfig
}

// TraceQuery 链路查询
type TraceQuery struct {
    jaegerClient *JaegerClient
}

type TraceResult struct {
    TraceID     string
    Duration    time.Duration
    Services    []string
    Spans       []Span
    Errors      int
    SlowSpans   []Span
}

type Span struct {
    SpanID       string
    ParentID     string
    ServiceName  string
    Operation    string
    Duration     time.Duration
    Tags         map[string]string
    Status       string  // ok | error
}

// SlowTraceDetector 慢链路检测器
type SlowTraceDetector struct {
    threshold time.Duration
}

// ServiceDependencyGraph 服务依赖图
type ServiceDependencyGraph struct {
    Nodes []ServiceNode
    Edges []ServiceEdge
}
```

### 73.4 链路查询

```
ops-ai trace query --service payment-api --operation POST /api/v1/pay --duration > 2s --limit 10
```

```
  ═══════════════════════════════════════════════════════
  🔗  链路追踪查询 — payment-api POST /api/v1/pay
  ═══════════════════════════════════════════════════════

  查询条件: duration > 2s，最近 10 条

  慢链路列表
  ─────────────────────────────────────────────────────
  Trace ID          耗时      服务数    错误
  ─────────────────────────────────────────────────────
  abc123def456      5.2s      5         0
  xyz789uvw012      4.8s      5         0
  
  链路详情: abc123def456
  ─────────────────────────────────────────────────────
  服务              操作                    耗时      状态
  ─────────────────────────────────────────────────────
  order-api         POST /api/v1/pay        5.2s      ok
  ├─ payment-service  POST /charge            4.9s      ok
  │   ├─ payment-db   SQL SELECT              0.1s      ok
  │   └─ redis-cache  GET payment:session     4.5s      ok  ← 瓶颈
  └─ inventory-service POST /reserve           0.2s      ok

  🔍 瓶颈定位: redis-cache GET 耗时 4.5s（预期 < 10ms）
  可能原因:
  1. Redis 大 Key 问题
  2. Redis 节点网络延迟
  3. Redis 连接池耗尽

  [D] 诊断 Redis  [Q] 返回
```

### 73.5 配置项

```yaml
# ~/.ops-ai/config.yaml
tracing:
  enabled: true
  
  jaeger:
    endpoint: "http://jaeger-query:16686"
    
  opentelemetry:
    endpoint: "http://otel-collector:4317"
    
  thresholds:
    slow_trace: "500ms"
    error_rate: 0.01
```

---

## 74. 运行时安全与容器加固（v2.1 新增）

### 74.1 问题场景

v2.0 **零匹配** "Falco/运行时安全/seccomp/AppArmor"。安全运维是现代 K8s 的必选项：

- Falco 告警：检测到容器异常行为（如 unexpected outbound connection、privileged escalation）
- seccomp/AppArmor 配置错误导致合法应用被阻止
- **实际运维场景**："Falco 告警：payment-api 容器尝试访问 /etc/shadow，需要判断是应用 bug 还是攻击行为"

### 74.2 设计目标

- Falco 告警集成和分析
- seccomp/AppArmor 配置审计
- 容器安全基线检查（non-root、read-only rootfs、no privileged）
- 运行时事件响应

### 74.3 Go 接口定义

```go
// RuntimeSecurityManager 运行时安全管理器
type RuntimeSecurityManager struct {
    falcoClient   *FalcoClient
    k8sClient     kubernetes.Interface
    config        RuntimeSecurityConfig
}

// FalcoAlertAnalyzer Falco 告警分析器
type FalcoAlertAnalyzer struct {
    falcoClient *FalcoClient
}

type FalcoAlert struct {
    Rule      string
    Priority  string  // Emergency | Alert | Critical | Error | Warning | Notice | Informational | Debug
    Output    string
    Time      time.Time
    Fields    map[string]string
}

// SecurityBaselineChecker 安全基线检查器
type SecurityBaselineChecker struct {
    k8sClient kubernetes.Interface
}

type SecurityBaselineResult struct {
    PodName     string
    Namespace   string
    Checks      []SecurityCheck
    Score       int  // 0-100
}

type SecurityCheck struct {
    Name        string
    Passed      bool
    Severity    string
    Description string
    Remediation string
}
```

### 74.4 Falco 告警分析

```
ops-ai security falco alerts --since 1h
```

```
  ═══════════════════════════════════════════════════════
  🛡️  Falco 告警 — 最近 1 小时
  ═══════════════════════════════════════════════════════

  告警列表
  ─────────────────────────────────────────────────────
  时间        优先级    规则                    Pod
  ─────────────────────────────────────────────────────
  08:15:23    Warning   Unexpected outbound     payment-api-7d8f9
                        connection
  08:22:01    Notice    Read sensitive file     order-worker-3a2b1
                        untrusted

  告警详情: payment-api-7d8f9
  ─────────────────────────────────────────────────────
  规则: Unexpected outbound connection
  优先级: Warning
  连接: 10.0.1.23:54321 → 8.8.8.8:53 (UDP/DNS)

  分析
  ─────────────────────────────────────────────────────
  容器尝试向 8.8.8.8 发送 DNS 查询。
  可能原因:
  1. 应用配置了外部 DNS 服务器（如 Google DNS）
  2. 恶意软件尝试 C2 通信
  3. 正常的 SDK/库行为

  建议: 检查应用配置，确认是否需要访问外部 DNS
  如果是恶意的，建议隔离 Pod 并启动安全调查

  [I] 隔离 Pod  [Q] 返回
```

### 74.5 安全基线检查

```
ops-ai security baseline --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  🛡️  安全基线检查 — payment namespace
  ═══════════════════════════════════════════════════════

  总体评分: 72/100

  Pod: payment-api-7d8f9
  ─────────────────────────────────────────────────────
  检查项                        状态    严重度
  ─────────────────────────────────────────────────────
  以非 root 用户运行             ❌      Critical
  rootfs 只读                   ❌      High
  禁止 privileged               ✅      -
  禁止 hostNetwork              ✅      -
  禁止 hostPID                  ✅      -
  seccomp 已启用                ⚠️      Medium

  修复建议
  ─────────────────────────────────────────────────────
  1. 设置 securityContext.runAsNonRoot: true
  2. 设置 securityContext.readOnlyRootFilesystem: true
  3. 配置 seccompProfile: RuntimeDefault
```

### 74.6 配置项

```yaml
# ~/.ops-ai/config.yaml
runtime_security:
  enabled: true
  
  falco:
    endpoint: "http://falco-sidekick:2801"
    
  baseline:
    checks:
      - run_as_non_root
      - read_only_rootfs
      - no_privileged
      - no_host_network
      - seccomp_enabled
      - drop_all_capabilities
```

---

## 75. 云厂商深度集成（v2.1 新增）

### 75.1 问题场景

v2.0 **零匹配** "AWS/Azure/GCP/阿里云/腾讯云"。Agent 部署在云上是绝大多数场景：

- 云负载均衡器（ALB/NLB/CLB）健康检查失败导致流量无法到达 Pod
- 云数据库（RDS/Cloud SQL）连接问题
- 云存储（S3/GCS/OSS）挂载到 PV 的故障
- **实际运维场景**："ELB 健康检查显示目标组所有实例 Unhealthy，但 Pod 本身正常，问题可能在 Security Group 或 Target Group 配置"

### 75.2 设计目标

- 多云厂商资源状态查询（EC2/VM、LB、RDS、S3 等）
- 云 K8s 服务（EKS/AKS/GKE）特定问题诊断
- 云监控集成（CloudWatch/Azure Monitor/GCP Monitoring）

### 75.3 Go 接口定义

```go
// CloudProviderManager 云厂商管理器
type CloudProviderManager struct {
    provider CloudProvider
    config   CloudConfig
}

type CloudProvider interface {
    Name() string
    GetLoadBalancer(ctx context.Context, name string) (*LoadBalancerStatus, error)
    GetVM(ctx context.Context, instanceID string) (*VMStatus, error)
    GetRDS(ctx context.Context, instanceID string) (*RDSStatus, error)
    GetCloudWatchMetrics(ctx context.Context, namespace, metricName string) ([]MetricDatapoint, error)
}

type LoadBalancerStatus struct {
    Name        string
    Type        string  // ALB | NLB | CLB
    State       string
    Targets     []LBTarget
    HealthCheck LBHealthCheck
}

type LBTarget struct {
    ID      string
    State   string  // healthy | unhealthy
    Reason  string
}

// EKSManager EKS 管理器
type EKSManager struct {
    eksClient *eks.Client
}
```

### 75.4 云负载均衡器诊断

```
ops-ai cloud lb status --name k8s-payment-alb
```

```
  ═══════════════════════════════════════════════════════
  ☁️  云负载均衡器诊断 — k8s-payment-alb (AWS ALB)
  ═══════════════════════════════════════════════════════

  ALB 状态: 🟢 active
  DNS:      k8s-payment-alb-123456789.us-east-1.elb.amazonaws.com

  目标组: tg-payment-api (port 80)
  ─────────────────────────────────────────────────────
  目标                状态        原因
  ─────────────────────────────────────────────────────
  i-0abc123def456    ❌ unhealthy  Health check failed: 503
  i-0xyz789uvw012    ❌ unhealthy  Health check failed: 503
  i-0mno345pqr678    ❌ unhealthy  Health check failed: 503

  健康检查配置
  ─────────────────────────────────────────────────────
  路径: /healthz
  端口: 8080
  间隔: 30s
  超时: 5s

  诊断结论
  ─────────────────────────────────────────────────────
  所有目标 unhealthy，但 Pod 本身 Running。
  可能原因:
  1. health check 路径 /healthz 不存在（实际可能是 /health）
  2. health check 端口 8080 错误（实际可能是 80）
  3. Security Group 阻止了 ALB 到节点的流量

  建议: 检查 Service 的 healthCheckNodePort 和 targetPort 配置

  [Q] 返回
```

### 75.5 配置项

```yaml
# ~/.ops-ai/config.yaml
cloud_provider:
  enabled: true
  
  aws:
    region: "us-east-1"
    profile: "default"
    
  azure:
    subscription_id: "xxx"
    resource_group: "k8s-rg"
    
  gcp:
    project_id: "my-project"
    zone: "us-central1-a"
    
  aliyun:
    region: "cn-beijing"
```

---

## 76. 中间件与有状态服务深度运维（v2.1 新增）

### 76.1 问题场景

v2.0 对 Redis、Kafka、MySQL、MongoDB 等中间件几乎没有设计：

- K8s 上部署的 Redis Cluster：节点故障、主从切换、内存满、大 Key 问题
- Kafka：Broker 故障、Topic 分区不均衡、消费者 Lag 突增
- Elasticsearch：索引分配失败、磁盘水位告警、查询性能劣化
- **实际运维场景**："Redis 内存使用率达到 95%，eviction 策略导致大量缓存失效，数据库压力激增"

### 76.2 设计目标

- Redis 诊断：内存使用、大 Key、慢查询、主从同步、Cluster 分片
- Kafka 诊断：Broker 健康、Topic 分区、消费者 Lag、ISR 状态
- Elasticsearch 诊断：集群健康、索引分配、磁盘水位、查询性能
- MongoDB 诊断：副本集状态、连接数、慢查询

### 76.3 Go 接口定义

```go
// MiddlewareManager 中间件管理器
type MiddlewareManager struct {
    k8sClient     kubernetes.Interface
    config        MiddlewareConfig
}

// RedisDiagnoser Redis 诊断器
type RedisDiagnoser struct {
    k8sClient kubernetes.Interface
}

type RedisStatus struct {
    Version      string
    Mode         string  // standalone | sentinel | cluster
    MemoryUsed   int64
    MemoryMax    int64
    MemoryUsage  float64
    ConnectedClients int
    KeyCount     int64
    BigKeys      []BigKey
    SlowLogs     []SlowLog
    Replication  ReplicationStatus
    Issues       []string
}

type BigKey struct {
    Key       string
    Type      string
    Size      int64
}

// KafkaDiagnoser Kafka 诊断器
type KafkaDiagnoser struct {
    k8sClient kubernetes.Interface
}

type KafkaStatus struct {
    Brokers       int
    Topics        int
    UnderReplicatedPartitions int
    OfflinePartitions int
    ConsumerGroups []ConsumerGroup
    Issues         []string
}

// ElasticsearchDiagnoser ES 诊断器
type ElasticsearchDiagnoser struct {
    k8sClient kubernetes.Interface
}

type ESStatus struct {
    ClusterName   string
    Status        string  // green | yellow | red
    Nodes         int
    Indices       int
    Shards        int
    UnassignedShards int
    DiskUsage     float64
    Issues        []string
}
```

### 76.4 Redis 诊断

```
ops-ai middleware redis status --service redis-cache -n cache
```

```
  ═══════════════════════════════════════════════════════
  🗄️  Redis 诊断 — redis-cache (cluster mode)
  ═══════════════════════════════════════════════════════

  版本: 7.0.12
  模式: Cluster (6 nodes)
  状态: 🟡 yellow (1 node missing)

  内存
  ─────────────────────────────────────────────────────
  已使用:    3.8GB / 4GB (95%)
  最大内存:   4GB (maxmemory 配置)
  策略:      allkeys-lru

  ⚠️  内存使用率达到 95%，即将触发 eviction

  大 Key (Top 5)
  ─────────────────────────────────────────────────────
  Key                    类型      大小
  ─────────────────────────────────────────────────────
  payment:sessions       hash      512MB
  user:profiles          hash      256MB
  order:cache            string    128MB

  慢查询 (最近 10 条)
  ─────────────────────────────────────────────────────
  命令                    耗时      时间
  ─────────────────────────────────────────────────────
  KEYS payment:*          2.5s      08:15:23
  HGETALL payment:sessions 1.2s      08:22:01

  问题分析
  ─────────────────────────────────────────────────────
  1. 内存即将耗尽，eviction 会频繁发生
  2. payment:sessions 是一个大 Key（512MB），建议拆分
  3. KEYS 命令在生产环境使用，会导致阻塞

  建议
  ─────────────────────────────────────────────────────
  1. 增加 maxmemory 到 6GB 或优化缓存策略
  2. 将 payment:sessions 按用户 ID 分片存储
  3. 禁止 KEYS 命令，改用 SCAN

  [A] 应用建议（L2）  [Q] 返回
```

### 76.5 Kafka 诊断

```
ops-ai middleware kafka status --namespace kafka
```

```
  ═══════════════════════════════════════════════════════
  🗄️  Kafka 诊断 — kafka cluster
  ═══════════════════════════════════════════════════════

  Brokers: 3 (1 offline)
  Topics:  24
  
  分区状态
  ─────────────────────────────────────────────────────
  Under-replicated:  12
  Offline:           3

  ⚠️  broker-2 离线，导致 3 个分区不可用

  消费者 Lag (Top 5)
  ─────────────────────────────────────────────────────
  Consumer Group          Topic              Lag
  ─────────────────────────────────────────────────────
  payment-processor       payment-events     1,234,567  🔴
  order-aggregator        order-events       456,789    🟠

  问题分析
  ─────────────────────────────────────────────────────
  1. broker-2 离线，需要恢复
  2. payment-processor 消费者 lag 严重，可能消费能力不足

  [Q] 返回
```

### 76.6 配置项

```yaml
# ~/.ops-ai/config.yaml
middleware:
  enabled: true
  
  redis:
    default_timeout: "5s"
    big_key_threshold_bytes: 1048576    # 1MB
    slow_log_threshold_ms: 100
    
  kafka:
    default_timeout: "10s"
    lag_alert_threshold: 100000
    
  elasticsearch:
    default_timeout: "10s"
    disk_watermark_high: 0.85
    
  mongodb:
    default_timeout: "5s"
```

---

## 77. SRE/SLO/错误预算管理（v2.1 新增）

### 77.1 问题场景

v2.0 **零匹配** "SLO/SLI/错误预算"。SRE 文化的核心被完全忽略：

- 如何定义和监控服务的 SLO？
- 错误预算消耗速度如何？是否允许发布？
- **实际运维场景**："本月 payment-api 的错误预算已消耗 80%，是否允许明天发布新版本？"

### 77.2 设计目标

- SLO 定义和监控
- SLI 自动计算（可用性、延迟、错误率、吞吐量）
- 错误预算追踪和告警
- 发布决策辅助（基于剩余错误预算）

### 77.3 Go 接口定义

```go
// SREManager SRE 管理器
type SREManager struct {
    promClient    promv1.API
    config        SREConfig
}

// SLIDefinition SLI 定义
type SLIDefinition struct {
    Name        string
    Service     string
    Metric      string  // availability | latency | error_rate | throughput
    Expression  string  // PromQL
    Target      float64
    Window      string  // 30d | 7d | 1d
}

// SLODefinition SLO 定义
type SLODefinition struct {
    Name        string
    Service     string
    SLIs        []SLIDefinition
    Objective   float64  // 如 99.9
    Window      string   // 30d
}

// ErrorBudget 错误预算
type ErrorBudget struct {
    SLOName     string
    TotalBudget float64  // 总允许错误数（如 0.1% = 43.2 分钟/月）
    UsedBudget  float64  // 已消耗
    Remaining   float64  // 剩余
    BurnRate    float64  // 消耗速率（x = 按当前速率将在 1/x 窗口内耗尽）
    Status      string   // healthy | at_risk | exhausted
}

// ReleaseGate 发布门禁
type ReleaseGate struct {
    sreManager *SREManager
}

func (g *ReleaseGate) CanRelease(service string) (bool, string) {
    budget := g.sreManager.GetErrorBudget(service)
    
    if budget.Status == "exhausted" {
        return false, fmt.Sprintf("错误预算已耗尽（%s），禁止发布", budget.SLOName)
    }
    if budget.Status == "at_risk" {
        return false, fmt.Sprintf("错误预算风险（剩余 %.1f%%），建议暂缓发布", budget.Remaining*100)
    }
    
    return true, fmt.Sprintf("错误预算充足（剩余 %.1f%%），允许发布", budget.Remaining*100)
}
```

### 77.4 SLO 监控面板

```
ops-ai sre status --service payment-api
```

```
  ═══════════════════════════════════════════════════════
  📊  SRE 状态 — payment-api
  ═══════════════════════════════════════════════════════

  SLO: 可用性 99.9% (30 天窗口)

  SLI 状态
  ─────────────────────────────────────────────────────
  指标        目标        当前        状态
  ─────────────────────────────────────────────────────
  可用性       99.9%       99.95%     ✅
  错误率       < 0.1%      0.05%      ✅
  P99 延迟    < 200ms     150ms      ✅

  错误预算
  ─────────────────────────────────────────────────────
  总预算:     43.2 分钟/月 (99.9% 允许的不可用时间)
  已消耗:     12.5 分钟 (28.9%)
  剩余:       30.7 分钟 (71.1%) ✅
  消耗速率:   0.5x (按当前速率将在 60 天内耗尽)

  发布门禁
  ─────────────────────────────────────────────────────
  状态: ✅ 允许发布
  原因: 错误预算充足（剩余 71.1%）

  [Q] 返回
```

### 77.5 配置项

```yaml
# ~/.ops-ai/config.yaml
sre:
  enabled: true
  
  slos:
    - name: "payment-api-availability"
      service: "payment-api"
      slis:
        - name: "availability"
          metric: "up{job='payment-api'}"
          target: 0.999
      window: "30d"
      
    - name: "payment-api-latency"
      service: "payment-api"
      slis:
        - name: "latency-p99"
          metric: "histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{job='payment-api'}[5m]))"
          target: 0.2
      window: "30d"
      
  release_gate:
    enabled: true
    min_error_budget_remaining: 0.3    # 至少剩余 30% 错误预算才允许发布
```

---

# 第五部分：P2 — 增强级（专业场景覆盖）

---

## 78. GPU 与异构计算运维（v2.1 新增）

### 78.1 问题场景

AI/ML 负载在 K8s 上快速增长，GPU 运维是新兴痛点：

- GPU 节点调度失败（nvidia-device-plugin 问题、GPU 资源不足）
- CUDA 版本不匹配、驱动问题
- MIG（Multi-Instance GPU）配置管理
- **实际运维场景**："训练任务 Pod 一直处于 Pending，describe 显示 '0/5 nodes are available: 5 Insufficient nvidia.com/gpu'"

### 78.2 设计目标

- GPU 节点状态监控
- nvidia-device-plugin 健康检查
- GPU 资源分配和利用率分析
- CUDA 版本兼容性检查

### 78.3 Go 接口定义

```go
// GPUManager GPU 管理器
type GPUManager struct {
    k8sClient kubernetes.Interface
}

// GPUStatus GPU 状态
type GPUStatus struct {
    NodeName     string
    GPUCount     int
    GPUModel     string
    DriverVersion string
    CUDAVersion  string
    Allocated    int
    Utilization  float64
    MemoryUsage  float64
    Issues       []string
}

// GPUPodDiagnoser GPU Pod 诊断器
type GPUPodDiagnoser struct {
    k8sClient kubernetes.Interface
}
```

### 78.4 GPU 诊断

```
ops-ai gpu status
ops-ai gpu diagnose <pod> -n <ns>
```

```
  ═══════════════════════════════════════════════════════
  🎮  GPU 状态
  ═══════════════════════════════════════════════════════

  节点: gpu-node-01
  GPU:  NVIDIA A100 (8 卡)
  驱动: 535.54.03
  CUDA: 12.2

  资源分配
  ─────────────────────────────────────────────────────
  卡      状态      分配 Pod              利用率    显存
  ─────────────────────────────────────────────────────
  gpu-0   ✅ 可用    -                     -         -
  gpu-1   ❌ 已占用  training-job-abc      95%       40GB/40GB
  gpu-2   ❌ 已占用  inference-svc-def     45%       18GB/40GB
  ...

  [Q] 返回
```

---

## 79. Serverless / Knative 支持（v2.1 新增）

### 79.1 问题场景

部分团队使用 Knative 或云厂商 Serverless：

- Revision 管理、自动缩容到零、冷启动问题
- 事件触发器（Eventing）故障
- **实际运维场景**："Knative Service 在流量低时缩容到 0，下次请求时冷启动耗时 30 秒"

### 79.2 设计目标

- Knative Serving Revision 管理
- 冷启动问题诊断
- Eventing 通道和订阅诊断

### 79.3 Go 接口定义

```go
// KnativeManager Knative 管理器
type KnativeManager struct {
    k8sClient      kubernetes.Interface
    servingClient  servingv1.Interface
    eventingClient eventingv1.Interface
}

// KnativeServiceStatus Knative Service 状态
type KnativeServiceStatus struct {
    Name         string
    Namespace    string
    Revisions    []RevisionStatus
    Traffic      []TrafficTarget
    ScaleToZero  bool
    ColdStartLatency time.Duration
}
```

### 79.4 Knative 诊断

```
ops-ai knative status --service inference-api -n ml
```

```
  ═══════════════════════════════════════════════════════
  ⚡  Knative Service — inference-api
  ═══════════════════════════════════════════════════════

  状态: ✅ Ready
  
  Revision
  ─────────────────────────────────────────────────────
  版本           流量     状态      年龄
  ─────────────────────────────────────────────────────
  inference-003   100%     ✅ Ready   2h
  inference-002   0%       ✅ Ready   1d
  inference-001   0%       ✅ Ready   7d

  自动扩缩容
  ─────────────────────────────────────────────────────
  最小副本: 0
  最大副本: 100
  当前副本: 1
  Scale-to-zero: 启用

  冷启动
  ─────────────────────────────────────────────────────
  平均冷启动: 15s
  ⚠️  冷启动时间较长，建议: 
      - 设置 minScale=1 避免缩容到零
      - 优化容器镜像大小
      - 使用预热策略

  [Q] 返回
```

---

## 80. Vault 与密钥管理深度集成（v2.1 新增）

### 80.1 问题场景

企业级密钥管理是安全基线：

- Vault 集成：动态 Secret、PKI 证书签发
- 云厂商 Secret Manager
- **实际运维场景**："应用无法连接数据库，Vault 日志显示 'lease expired'，需要重新签发数据库凭据"

### 80.2 设计目标

- Vault 健康检查
- Secret 动态签发状态监控
- PKI 证书管理
- 与 ESO（External Secrets Operator）联动

### 80.3 Go 接口定义

```go
// VaultManager Vault 管理器
type VaultManager struct {
    vaultClient *vault.Client
    config      VaultConfig
}

// VaultStatus Vault 状态
type VaultStatus struct {
    Sealed      bool
    Standby     bool
    Version     string
    ClusterName string
    Leases      []LeaseStatus
}

type LeaseStatus struct {
    ID        string
    Path      string
    TTL       time.Duration
    ExpiresAt time.Time
}
```

### 80.4 Vault 诊断

```
ops-ai vault status
ops-ai vault leases --path database/creds/
```

```
  ═══════════════════════════════════════════════════════
  🔐  Vault 状态
  ═══════════════════════════════════════════════════════

  状态: ✅ Unsealed
  版本: 1.15.0
  集群: vault-prod

  Lease 状态
  ─────────────────────────────────────────────────────
  路径                          数量    即将过期
  ─────────────────────────────────────────────────────
  database/creds/payment        12      2 (1h 内)
  database/creds/order          8       0
  pki/issue/payment-tls         5       1 (30m 内)

  ⚠️  2 个 database lease 将在 1 小时内过期
  建议: 检查相关应用是否配置了自动续租

  [Q] 返回
```

---

## 81. CNI 深度诊断（v2.1 新增）

### 81.1 问题场景

网络问题占 K8s 运维问题的 30% 以上：

- CNI 插件故障导致 Pod 无法获取 IP
- Calico BGP 邻居状态异常
- Cilium eBPF 程序加载失败
- **实际运维场景**："Pod 一直处于 ContainerCreating，describe 显示 'Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox: plugin type="calico" failed'"

### 81.2 设计目标

- CNI 插件健康检查
- Calico BGP 邻居状态
- Cilium eBPF 程序状态
- 跨节点网络连通性诊断

### 81.3 Go 接口定义

```go
// CNIManager CNI 管理器
type CNIManager struct {
    k8sClient kubernetes.Interface
}

// CNIStatus CNI 状态
type CNIStatus struct {
    Plugin      string  // calico | cilium | flannel | weave
    Version     string
    NodesReady  int
    NodesTotal  int
    Issues      []CNIssue
}

type CNIssue struct {
    Type        string
    Node        string
    Severity    string
    Description string
    Suggestion  string
}
```

### 81.4 CNI 诊断

```
ops-ai cni status
ops-ai cni diagnose --node worker-01
```

```
  ═══════════════════════════════════════════════════════
  🌐  CNI 状态 — Calico v3.26.0
  ═══════════════════════════════════════════════════════

  插件: Calico
  节点就绪: 8/10

  BGP 邻居状态
  ─────────────────────────────────────────────────────
  节点        邻居        状态      ASN
  ─────────────────────────────────────────────────────
  worker-01   worker-02   ✅ Up     64512
  worker-01   worker-03   ❌ Down   64512
  worker-02   worker-03   ❌ Down   64512

  ⚠️  worker-03 BGP 连接异常
  可能原因:
  1. worker-03 calico-node Pod 未运行
  2. worker-03 网络不通
  3. BGP 密码配置不一致

  [Q] 返回
```

---

## 82. Ingress 与云负载均衡器深度运维（v2.1 新增）

### 82.1 问题场景

504/502 是最常见的用户侧报错，根因通常在 Ingress 或云负载均衡器层：

- Ingress-NGINX/Traefik 控制器故障
- 证书自动续期失败
- 云 LB 与 K8s Service 映射问题
- **实际运维场景**："用户报告访问 API 返回 504 Gateway Timeout，但直接访问 Pod 正常。问题可能在 Ingress 超时配置或云 LB 健康检查"

### 82.2 设计目标

- Ingress 控制器健康检查
- Ingress 规则冲突检测
- 证书续期状态监控
- 云负载均衡器与 K8s Service 映射诊断

### 82.3 Go 接口定义

```go
// IngressManager Ingress 管理器
type IngressManager struct {
    k8sClient     kubernetes.Interface
    cloudProvider CloudProvider
}

// IngressStatus Ingress 状态
type IngressStatus struct {
    Name        string
    Namespace   string
    Class       string
    Rules       []IngressRuleStatus
    TLS         []TLSStatus
    Backend     BackendStatus
    Issues      []string
}

type TLSStatus struct {
    SecretName  string
    ExpiresAt   time.Time
    DaysLeft    int
    Valid       bool
}

type BackendStatus struct {
    ServiceName string
    Endpoints   int
    Healthy     int
}
```

### 82.4 Ingress 诊断

```
ops-ai ingress status --namespace payment
```

```
  ═══════════════════════════════════════════════════════
  🌐  Ingress 状态 — payment namespace
  ═══════════════════════════════════════════════════════

  Ingress: payment-api
  Class:   nginx
  
  规则
  ─────────────────────────────────────────────────────
  主机                      路径        后端服务
  ─────────────────────────────────────────────────────
  api.payment.com           /           payment-api:80 ✅

  TLS
  ─────────────────────────────────────────────────────
  Secret: payment-tls
  有效期:  45 天
  自动续期: cert-manager ✅

  后端健康
  ─────────────────────────────────────────────────────
  Service: payment-api
  Endpoints: 3/3 ✅

  问题: 无

  [Q] 返回
```

---

## 83. 内核与操作系统级诊断（v2.1 新增）

### 83.1 问题场景

节点级问题的底层根因通常在操作系统层：

- 节点内核参数（sysctl）配置错误导致网络/性能问题
- cgroups v1/v2 压力导致 OOM 误判
- systemd 服务故障导致 kubelet 无法启动
- **实际运维场景**："节点频繁 OOMKilled Pod，但节点内存使用率只有 60%。排查发现 cgroup memory.pressure 导致内核误判"

### 83.2 设计目标

- 节点内核参数（sysctl）审计
- cgroups 压力监控
- systemd 服务状态检查
- NTP/时区同步检查

### 83.3 Go 接口定义

```go
// NodeOSManager 节点 OS 管理器
type NodeOSManager struct {
    k8sClient kubernetes.Interface
}

// NodeOSStatus 节点 OS 状态
type NodeOSStatus struct {
    NodeName      string
    KernelVersion string
    OSImage       string
    Sysctl        map[string]string
    Cgroups       CgroupsStatus
    Systemd       []SystemdService
    NTP           NTPStatus
    Issues        []string
}

type CgroupsStatus struct {
    Version    string
    MemoryPressure float64
    CPUPressure    float64
    IOPressure     float64
}

type SystemdService struct {
    Name    string
    Status  string
    Active  bool
}

type NTPStatus struct {
    Synced      bool
    Offset      time.Duration
    Source      string
}
```

### 83.4 内核诊断

```
ops-ai node os --node worker-01
```

```
  ═══════════════════════════════════════════════════════
  🖥️  节点 OS 诊断 — worker-01
  ═══════════════════════════════════════════════════════

  内核: 5.15.0-1035-aws
  OS:   Ubuntu 22.04.3 LTS

  Sysctl
  ─────────────────────────────────────────────────────
  参数                          当前值      推荐值      状态
  ─────────────────────────────────────────────────────
  net.ipv4.ip_forward           1           1           ✅
  vm.overcommit_memory          0           1           ⚠️
  kernel.pid_max                32768       65536       ⚠️

  ⚠️  vm.overcommit_memory=0 可能导致内存分配失败
  建议: 设置为 1（允许 overcommit）

  Cgroups 压力
  ─────────────────────────────────────────────────────
  内存压力:  85% ⚠️
  CPU 压力:   15% ✅
  IO 压力:    30% ✅

  ⚠️  cgroup 内存压力高，可能导致 OOM 误判

  Systemd
  ─────────────────────────────────────────────────────
  kubelet      ✅ active
  containerd   ✅ active
  chronyd      ⚠️  inactive

  ⚠️  chronyd 未运行，可能导致时间漂移

  [Q] 返回
```

---

## 84. 弹性体系深度运维 — HPA/VPA/CA/Karpenter（v2.1 新增）

### 84.1 问题场景

流量突增时不扩容是 P0 事故：

- HPA 不扩容的原因（metrics 不可用、target 配置错误、cooldown 期）
- Cluster Autoscaler 不扩容节点的原因（taints、资源碎片、max node limit）
- Karpenter 的 provisioner 配置问题
- **实际运维场景**："流量突增时 HPA 显示 Current 50%/Target 60%，但 replicas 不增加。原因是 metrics-server 未部署"

### 84.2 设计目标

- HPA 状态诊断和不扩容原因分析
- Cluster Autoscaler 状态和不扩容原因分析
- Karpenter 诊断
- 弹性体系联动分析

### 84.3 Go 接口定义

```go
// ElasticityManager 弹性管理器
type ElasticityManager struct {
    k8sClient kubernetes.Interface
}

// HPAStatus HPA 状态
type HPAStatus struct {
    Name        string
    Namespace   string
    Target      string
    MinReplicas int32
    MaxReplicas int32
    CurrentReplicas int32
    DesiredReplicas int32
    Metrics     []HPAMetric
    Conditions  []HPACondition
    Issues      []string
}

type HPACondition struct {
    Type    string
    Status  string
    Reason  string
    Message string
}

// ClusterAutoscalerStatus CA 状态
type ClusterAutoscalerStatus struct {
    Healthy        bool
    NodeGroups     []NodeGroupStatus
    ScaleUpStatus  string
    ScaleDownStatus string
    Issues         []string
}

type NodeGroupStatus struct {
    Name        string
    MinSize     int
    MaxSize     int
    CurrentSize int
    TargetSize  int
}

// KarpenterStatus Karpenter 状态
type KarpenterStatus struct {
    Healthy     bool
    Provisioners []ProvisionerStatus
    Issues      []string
}

type ProvisionerStatus struct {
    Name       string
    Ready      bool
    NodeCount  int
    Constraints []string
}
```

### 84.4 HPA 诊断

```
ops-ai elasticity hpa diagnose --name payment-api -n payment
```

```
  ═══════════════════════════════════════════════════════
  📈  HPA 诊断 — payment-api
  ═══════════════════════════════════════════════════════

  目标: Deployment/payment-api
  副本: 3 / 3-10
  
  指标状态
  ─────────────────────────────────────────────────────
  指标类型        当前值      目标值      状态
  ─────────────────────────────────────────────────────
  CPU 利用率       50%         60%         🟢
  内存利用率       45%         80%         🟢

  条件
  ─────────────────────────────────────────────────────
  AbleToScale:     True ✅
  ScalingActive:   False ❌
  Reason:          the HPA was unable to compute the replica count:
                   missing request for cpu

  🔴 问题: Pod 没有设置 CPU request，HPA 无法计算利用率

  修复建议
  ─────────────────────────────────────────────────────
  为 Pod 的容器添加 resources.requests.cpu：
    resources:
      requests:
        cpu: "100m"

  [A] 修复（L2）  [Q] 返回
```

### 84.5 Cluster Autoscaler 诊断

```
ops-ai elasticity ca status
```

```
  ═══════════════════════════════════════════════════════
  📈  Cluster Autoscaler 状态
  ═══════════════════════════════════════════════════════

  状态: 🟢 Healthy

  节点组
  ─────────────────────────────────────────────────────
  节点组              当前    目标    最小    最大
  ─────────────────────────────────────────────────────
  worker-nodes        5       5       3       20
  gpu-nodes           1       1       0       5

  扩容事件 (最近 10)
  ─────────────────────────────────────────────────────
  时间              节点组        变化    原因
  ─────────────────────────────────────────────────────
  2h ago            worker-nodes  +2      资源不足
  5h ago            worker-nodes  -1      利用率低

  [Q] 返回
```

### 84.6 配置项

```yaml
# ~/.ops-ai/config.yaml
elasticity:
  enabled: true
  
  hpa:
    diagnose_missing_requests: true
    diagnose_metrics_server: true
    
  cluster_autoscaler:
    namespace: "kube-system"
    
  karpenter:
    namespace: "karpenter"
```

---

# 第六部分：开发路线图 v2.1

## Phase 1: P0 阻断级（3 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 1 | §70 多集群/混合云 | 多集群上下文、差异检测、批量执行 |
| 1 | §71 GitOps 深度运维 | ArgoCD/Flux/Helm/Kustomize 诊断 |
| 2 | §72 容器运行时深度诊断 | containerd/CRI-O 状态、镜像拉取、OCI 错误诊断 |
| 3 | P0 集成测试 | 多集群 + GitOps 场景测试 |

## Phase 2: P1 重要级（4 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 4 | §73 分布式链路追踪 | OpenTelemetry/Jaeger 集成、链路查询、慢链路检测 |
| 4 | §74 运行时安全 | Falco 告警、seccomp/AppArmor 审计、安全基线 |
| 5 | §75 云厂商深度集成 | AWS/Azure/GCP/阿里云负载均衡器、数据库、监控 |
| 5 | §76 中间件深度运维 | Redis/Kafka/ES/MongoDB 诊断 |
| 6 | §77 SRE/SLO/错误预算 | SLO 监控、错误预算追踪、发布门禁 |
| 7 | P1 集成测试 | 全栈场景测试 |

## Phase 3: P2 增强级（3 周）

| 周 | 任务 | 交付物 |
|----|------|--------|
| 8 | §78 GPU + §79 Serverless | GPU 诊断、Knative 支持 |
| 8 | §80 Vault + §81 CNI | Vault 集成、CNI 诊断 |
| 9 | §82 Ingress + §83 内核 + §84 弹性体系 | Ingress 诊断、OS 诊断、HPA/CA/Karpenter |
| 10 | 全量回归测试 + 文档完善 | 最终测试报告 |

## 总工期

**10 周**（约 2.5 个月）

---

# 附录 A：Changelog

## v2.0 → v2.1（2026-06-25）— 全栈运维版

### P0 — 阻断级（3 项）
- **§70 多集群与混合云统一管理**：新增多 kubeconfig 上下文管理、跨集群资源差异检测、多集群批量诊断
- **§71 GitOps 深度运维**：新增 ArgoCD Application 诊断、Flux Kustomization 诊断、Helm Chart 诊断、Kustomize 渲染预览
- **§72 容器运行时深度诊断**：新增 containerd/CRI-O 状态检查、镜像拉取失败诊断、OCI runtime 错误分析

### P1 — 重要级（5 项）
- **§73 分布式链路追踪与 APM 集成**：新增 OpenTelemetry/Jaeger 集成、链路查询与可视化、慢链路检测、服务依赖图
- **§74 运行时安全与容器加固**：新增 Falco 告警分析、seccomp/AppArmor 审计、安全基线检查、运行时事件响应
- **§75 云厂商深度集成**：新增 AWS/Azure/GCP/阿里云负载均衡器诊断、云数据库状态、云监控集成
- **§76 中间件与有状态服务深度运维**：新增 Redis 诊断（内存/大 Key/慢查询）、Kafka 诊断（分区/Lag）、Elasticsearch 诊断、MongoDB 诊断
- **§77 SRE/SLO/错误预算管理**：新增 SLO 定义与监控、SLI 自动计算、错误预算追踪、发布门禁（基于剩余错误预算）

### P2 — 增强级（7 项）
- **§78 GPU 与异构计算运维**：新增 GPU 节点状态、nvidia-device-plugin 诊断、CUDA 版本检查、MIG 配置
- **§79 Serverless / Knative 支持**：新增 Knative Service Revision 管理、冷启动诊断、Eventing 诊断
- **§80 Vault 与密钥管理深度集成**：新增 Vault 健康检查、Lease 监控、PKI 证书管理
- **§81 CNI 深度诊断**：新增 Calico BGP 状态、Cilium eBPF 状态、跨节点网络连通性诊断
- **§82 Ingress 与云负载均衡器深度运维**：新增 Ingress 控制器健康、证书续期监控、云 LB 映射诊断、504/502 根因分析
- **§83 内核与操作系统级诊断**：新增 sysctl 审计、cgroups 压力监控、systemd 服务检查、NTP 同步
- **§84 弹性体系深度运维**：新增 HPA 不扩容原因分析、Cluster Autoscaler 状态、Karpenter 诊断

### 版本演进全景

```
v1.0 概念验证版  →  v1.1 可交互版  →  v1.2 安全初版  →  v1.3 生产集群实战版
    ↓
v1.4 可交付版  →  v1.5 企业可用版  →  v1.6 企业可推广版  →  v1.7 产品可交付版
    ↓
v1.8 生产安全版  →  v1.9 生产可靠版 + 企业就绪  →  v2.0 规模化生产版
    ↓
v2.1 全栈运维版
    (多集群 + GitOps + 容器运行时 + 链路追踪 + 运行时安全 + 云厂商 + 中间件 + SLO + GPU + Serverless + Vault + CNI + Ingress + 内核 + 弹性体系)
```

---

# 附录 B：v2.1 运维视角审查结论

经过对 v2.1 的全面审查，从一线 SRE/DevOps 的实际生产视角出发，**v2.1 已覆盖现代 K8s 运维的绝大多数核心场景**：

## 已覆盖的场景（✅）

| 类别 | 覆盖内容 |
|------|----------|
| **基础运维** | Pod/Deployment/Service/ConfigMap/Secret 全生命周期 |
| **可观测性** | Prometheus/Loki/日志/指标/告警/链路追踪（OTel/Jaeger） |
| **安全** | RBAC/审计/Secret 脱敏/合规/CIS/运行时安全（Falco/seccomp）/供应链安全/Vault |
| **网络** | Service/Ingress/NetworkPolicy/CNI/Calico/Cilium/服务网格（Istio/Linkerd） |
| **存储** | PVC/PV/CSI/Snapshot/StatefulSet/有状态服务 |
| **发布** | 金丝雀/蓝绿/A-B 测试（Argo Rollouts/Flagger）/GitOps（ArgoCD/Flux）/Helm/Kustomize |
| **弹性** | HPA/VPA/Cluster Autoscaler/Karpenter |
| **集群管理** | 节点/Namespace/资源配额/多集群/混合云/集群升级 |
| **灾难恢复** | etcd 备份/Velero/CSI 快照/跨集群切换 |
| **事件管理** | 告警自动修复/事件全生命周期/War Room/Postmortem |
| **成本** | 容量规划/Right-sizing/费用预算 |
| **合规** | PCI-DSS/SOC2/等保2.0/CIS Benchmark/数据驻留 |
| **中间件** | Redis/Kafka/ES/MongoDB |
| **特殊场景** | GPU/Serverless（Knative）/边缘计算（K3s）/Windows 节点 |
| **基础设施** | 云厂商（AWS/Azure/GCP/阿里云）/容器运行时/内核/OS |
| **SRE 文化** | SLO/SLI/错误预算/发布门禁 |
| **Agent 自身** | 高可用/自升级/性能优化/调试/插件/国际化 |

## 仍存在的边缘场景（P3 — 极小众）

以下场景在 v2.1 中仍未专门设计，但影响范围极小或可通过现有工具间接覆盖：

1. **裸金属 K8s（Bare Metal）**：节点硬件故障诊断（IPMI/Redfish）—— 可通过节点诊断 + SSH 间接覆盖
2. **VMware/vSphere 集成**：vSphere CSI/Cloud Provider 特定问题 —— 可通过云厂商通用框架扩展
3. **特殊硬件（FPGA/TPU）**：比 GPU 更小众的异构计算 —— 可通过 GPU 框架类比扩展
4. **遗留系统（OpenShift 3.x）**：已停止维护的版本 —— 不在支持范围内
5. **极端安全环境（Air-gapped）**：完全断网环境 —— 可通过本地模型 + 离线镜像支持

## 最终结论

> **v2.1 已覆盖现代 K8s 生产运维的 95%+ 核心场景。剩余 5% 为极小众边缘场景，不影响 Agent 进入生产环境。**

建议后续版本（v2.2/v3.0） focus 于：
- **AI 能力增强**：更智能的根因分析、预测性运维、自然语言交互优化
- **生态集成扩展**：更多云厂商、更多中间件、更多 GitOps 工具
- **用户体验优化**：更直观的可视化、更智能的推荐、更低的误报率

---

**文档版本**：v2.1
**日期**：2026-06-25
**变更**：基于 v2.0 运维视角差距分析，补齐 15 项缺口（3 P0 + 5 P1 + 7 P2）
**审查结论**：v2.1 已覆盖现代 K8s 运维绝大多数核心场景，达到"规模化生产可靠 + 全栈深度运维"目标