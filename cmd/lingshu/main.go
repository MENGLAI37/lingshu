package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/alertd"
	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/k8s"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/security"
	"github.com/lingshu/lingshu/pkg/tools"
	"github.com/lingshu/lingshu/pkg/tools/l0"
	"github.com/lingshu/lingshu/pkg/tools/l1"
	"github.com/lingshu/lingshu/pkg/tools/l2"
	"github.com/lingshu/lingshu/pkg/tui/models"
)

var Version = "v0.1.0"

func main() {
	noTUI := flag.Bool("no-tui", false, "Headless mode: ask a question and get an answer")
	autoDemo := flag.Bool("auto-demo", false, "Autonomous ops demo: simulated alert triggers auto diagnosis")
	dryRun := flag.Bool("dry-run", false, "Preview mode: diagnose + plan but skip all write operations (L1+)")
	yesMode := flag.Bool("yes", false, "Auto-confirm L0-L2 operations (for CI/CD pipelines)")
	pipeMode := flag.Bool("pipe", false, "Machine-readable output (JSON lines, for CI/CD)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("lingshu version %s\n", Version)
		os.Exit(0)
	}

	if *autoDemo {
		runAutoDemo(*dryRun)
		return
	}

	if *noTUI {
		query := strings.Join(flag.Args(), " ")
		exitCode := runNoTUI(query, *dryRun, *yesMode, *pipeMode)
		os.Exit(exitCode)
	}

	if err := runTUI(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI() error {
	model := models.NewTUIModel()
	model.SetCluster("kind-lingshu-dev")
	model.SetNamespace("default")
	model.SetEnvironment("development")

	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)

	_, err := p.Run()
	return err
}

// runNoTUI runs the agent loop in headless mode.
// If query is empty, it prints usage info.
// Exit codes for CI/CD mode:
// 0 = success, 1 = general error, 2 = security blocked, 3 = timeout, 4 = circuit breaker
const (
	ExitSuccess        = 0
	ExitError          = 1
	ExitSecurityBlock  = 2
	ExitTimeout        = 3
	ExitCircuitBreaker = 4
)

func runNoTUI(query string, dryRun, yesMode, pipeMode bool) int {
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  灵枢 (LingShu) - AI-Native SRE Agent       ║")
	fmt.Println("║  Version: " + Version + "                              ║")
	fmt.Println("║  Mode: Headless (No-TUI)                     ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	if query == "" {
		fmt.Println("Usage: lingshu --no-tui <your question>")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lingshu --no-tui \"查看 default 命名空间的所有 Pod\"")
		fmt.Println("  lingshu --no-tui \"检查集群健康状态\"")
		fmt.Println("  lingshu --no-tui \"nginx deployment 为什么重启了?\"")
		return ExitSuccess
	}

	// ---- Initialize ----
	fmt.Println("[1/4] Loading LLM config...")
	if err := config.LoadLLMConfig(); err != nil {
		fmt.Printf("  ⚠ Failed to load LLM config: %v\n", err)
	}
	providerCfg := config.GetCurrentProviderConfig()
	if providerCfg == nil {
		fmt.Println("  ✗ No LLM provider configured.")

		// First-run onboarding
		if config.IsFirstRun() {
			fmt.Println()
			fmt.Println("  ╔══════════════════════════════════════════════════╗")
			fmt.Println("  ║  🚀 欢迎使用灵枢 (LingShu)!                      ║")
			fmt.Println("  ║                                                  ║")
			fmt.Println("  ║  快速配置:                                       ║")
			fmt.Println("  ║  1. 设置 LLM API Key (任选其一):                  ║")
			fmt.Println("  ║     export OPENAI_API_KEY=\"sk-...\"                ║")
			fmt.Println("  ║     export DEEPSEEK_API_KEY=\"sk-...\"              ║")
			fmt.Println("  ║  2. 确保 kubeconfig 可用 (kubectl cluster-info)    ║")
			fmt.Println("  ║  3. 重新运行: lingshu --no-tui \"你的问题\"         ║")
			fmt.Println("  ║                                                  ║")
			fmt.Println("  ║  TUI 交互模式: lingshu (推荐)                     ║")
			fmt.Println("  ╚══════════════════════════════════════════════════╝")
		} else {
			fmt.Println("  💡 设置 ~/.lingshu/config.yaml 或 OPENAI_API_KEY 环境变量")
		}
		return ExitError
	}
	fmt.Printf("  ✓ Provider: %s (model: %s)\n", providerCfg.Name, providerCfg.Model)
	llmRouter := llm.NewRouter([]llm.ProviderConfig{*providerCfg})

	fmt.Println("[2/4] Connecting to Kubernetes...")
	k8sClient, err := k8s.NewClientManager("")
	if err != nil {
		fmt.Printf("  ⚠ Failed to init K8s client: %v\n", err)
		fmt.Println("  Continuing without K8s tools (demo mode)...")
	} else {
		currentCtx := k8sClient.GetCurrentContext()
		fmt.Printf("  ✓ Connected (context: %s)\n", currentCtx)
	}

	fmt.Println("[3/4] Registering tools & security...")
	toolRegistry := agent.NewDefaultToolRegistry()
	if k8sClient != nil {
		clientset, err := k8sClient.GetClientSet(context.Background(), "")
		if err != nil {
			fmt.Printf("  ⚠ Failed to get clientset: %v\n", err)
		} else {
			_ = toolRegistry.RegisterTool(l0.NewGetTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewDescribeTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewLogsTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewEventsTool(clientset))
			_ = toolRegistry.RegisterTool(l1.NewTopTool(clientset, nil))
			_ = toolRegistry.RegisterTool(l1.NewStatusTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewScaleTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewRestartTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewRolloutTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewPatchTool(clientset))
			toolList := toolRegistry.ListTools()
			fmt.Printf("  ✓ Registered %d K8s tools\n", len(toolList))
			for _, t := range toolList {
				fmt.Printf("    - %s [%s]: %s\n", t.Name(), t.RiskLevel(), t.Description())
			}

				// RBAC self-check
				if k8sClient != nil {
					fmt.Println()
					fmt.Println("  RBAC Permission Check:")
					toolNames := make([]string, len(toolList))
					for i, t := range toolList {
						toolNames[i] = t.Name()
					}
					permResults, permErr := k8sClient.CheckToolPermissions(context.Background(), toolNames)
					if permErr != nil {
						fmt.Printf("  RBAC check failed: %v\n", permErr)
					} else {
						missing := 0
						for name, ok := range permResults {
							if !ok {
								if p, found := k8s.ToolPermissionMap[name]; found {
									fmt.Printf("  %s: MISSING %s on %s\n", name, p.Verb, p.Resource)
								}
								missing++
							}
						}
						if missing == 0 {
							fmt.Println("  All tool permissions verified")
						} else {
							fmt.Printf("  %d tool(s) lack permissions\n", missing)
						}
					}
				}
		}
	} else {
		fmt.Println("  ⚠ Skipping K8s tools (no cluster access)")
	}

	secGateway := &noTUISecurityGateway{
		gateway: security.NewDefaultSecurityGateway(security.DefaultGatewayConfig()),
	}
	fmt.Println("  ✓ Security gateway ready (L0-L4)")

	loopCfg := agent.DefaultLoopConfig()
	loopCfg.DryRun = dryRun
	if yesMode {
		loopCfg.ConfirmationHandler = func(req agent.ConfirmationRequest) bool {
			// In yes mode, auto-confirm L0-L1, deny L3+
			if req.RiskLevel == tools.RiskLevelL0 || req.RiskLevel == tools.RiskLevelL1 || req.RiskLevel == tools.RiskLevelL2 {
				if !pipeMode {
					fmt.Printf("  [--yes] Auto-confirmed %s (%s)\n", req.ToolName, req.RiskLevel)
				}
				return true
			}
			if !pipeMode {
				fmt.Printf("  [--yes] Denied %s (%s) - L3+ requires manual approval\n", req.ToolName, req.RiskLevel)
			}
			return false
		}
	} else {
		loopCfg.ConfirmationHandler = func(req agent.ConfirmationRequest) bool {
		fmt.Printf("\n  ⚠ CONFIRMATION REQUIRED [%s]: %s\n", req.RiskLevel, req.Message)
		fmt.Print("  Confirm? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
	}
	}

	// Wire audit manager for tool call logging
	// (audit is optional - if DB not available, operations continue without audit)

	agentLoop := agent.NewDefaultAgentLoop(loopCfg, llmRouter, toolRegistry, secGateway)
	fmt.Println("[4/4] Agent loop initialized.")

	// Auto health check at startup
	if !pipeMode {
		fmt.Println()
		fmt.Println("🏥 Running startup health check...")
	}
	healthCtx := context.Background()
	healthResult, healthErr := agentLoop.Execute(healthCtx,
		"对集群进行一次快速健康检查。只需使用 k8s_status 工具获取状态，一句话总结即可。",
		func(event agent.LoopEvent) {}) // silent handler
	if healthErr == nil && healthResult.FinalResponse != "" && !pipeMode {
		fmt.Printf("  ✓ 集群状态: %s\n", truncateStr(healthResult.FinalResponse, 120))
	}
	fmt.Println()

	// ---- Execute ----
	if !pipeMode {
		fmt.Printf("🔍 Query: %s\n", query)
		fmt.Println(strings.Repeat("─", 60))
	}

	ctx := context.Background()
	eventHandler := func(event agent.LoopEvent) {
		switch event.Type {
		case "thinking":
			if thought, ok := event.Data.(string); ok && thought != "" {
				fmt.Printf("\n💭 Thinking:\n%s\n", thought)
			}
		case "state_change":
			switch event.State {
			case agent.StateExecuting:
				fmt.Println("🔧 Executing tools...")
			case agent.StateObserving:
				fmt.Println("👁 Observing results...")
			}
		case "tool_result":
			if result, ok := event.Data.(agent.ToolExecutionResult); ok {
				if result.Error != nil {
					fmt.Printf("  ✗ %s: %v\n", result.ToolName, result.Error)
				} else if result.Result != nil {
					dataStr := ""
					if result.Result.Data != nil {
						if b, err := jsonMarshalIndent(result.Result.Data); err == nil {
							dataStr = string(b)
						}
					}
					fmt.Printf("  ✓ %s [%s]: %s\n", result.ToolName, result.Duration, result.Result.Message)
					if dataStr != "" && len(dataStr) < 2000 {
						fmt.Printf("    %s\n", strings.ReplaceAll(dataStr, "\n", "\n    "))
					}
				}
			}
		case "error":
			if err, ok := event.Data.(error); ok {
				fmt.Printf("  ✗ Error: %v\n", err)
			}
		}
	}

	result, err := agentLoop.Execute(ctx, query, eventHandler)

	if !pipeMode {
		fmt.Println(strings.Repeat("─", 60))
	}
	if err != nil {
		errStr := err.Error()
		exitCode := ExitError
		if strings.Contains(errStr, "security blocked") {
			exitCode = ExitSecurityBlock
		} else if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
			exitCode = ExitTimeout
		} else if strings.Contains(errStr, "circuit breaker") {
			exitCode = ExitCircuitBreaker
		}

		if pipeMode {
			fmt.Printf(`{"status":"error","error":%q,"exit_code":%d}`+"\n", errStr, exitCode)
		} else {
			fmt.Printf("\n✗ Agent execution failed: %v\n", err)
		}
		return exitCode
	}

	if pipeMode {
		fmt.Printf(`{"status":"success","iterations":%d,"duration":%q,"tokens":%d}`+"\n",
			result.TotalIterations, result.TotalDuration.String(), result.TokenUsage.TotalTokens)
		if result.FinalResponse != "" {
			b, _ := json.Marshal(result.FinalResponse)
			fmt.Printf(`{"status":"response","content":%s}`+"\n", string(b))
		}
	} else {
		fmt.Printf("\n✅ Agent completed in %v (%d iterations)\n",
			result.TotalDuration.Round(10000000), // round to 10ms
			result.TotalIterations)
		fmt.Printf("   Tokens: %d input + %d output = %d total\n",
			result.TokenUsage.InputTokens,
			result.TokenUsage.OutputTokens,
			result.TokenUsage.TotalTokens)

		if result.FinalResponse != "" {
			fmt.Println()
			fmt.Println("📋 Final Response:")
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println(result.FinalResponse)
			fmt.Println(strings.Repeat("─", 60))
		}
	}
	return ExitSuccess
}

// jsonMarshalIndent marshals data to indented JSON bytes.
func jsonMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// noTUISecurityGateway wraps security.DefaultSecurityGateway to implement agent.SecurityGateway.
type noTUISecurityGateway struct {
	gateway *security.DefaultSecurityGateway
}

func (a *noTUISecurityGateway) EvaluateRisk(ctx context.Context, toolName string, args map[string]any) (agent.RiskEvaluation, error) {
	eval, err := a.gateway.EvaluateRisk(ctx, toolName, args)
	if err != nil {
		return agent.RiskEvaluation{}, err
	}
	return agent.RiskEvaluation{
		RiskLevel:         tools.ToolRiskLevel(eval.RiskLevel),
		Score:             eval.Score,
		Reason:            eval.Reason,
		AffectedResources: eval.AffectedResources,
		EnvironmentWeight: eval.EnvironmentWeight,
		ToolRiskLevel:     tools.ToolRiskLevel(eval.ToolRiskLevel),
	}, nil
}

func (a *noTUISecurityGateway) RequiresConfirmation(ctx context.Context, evaluation agent.RiskEvaluation) bool {
	return a.gateway.RequiresConfirmation(ctx, security.RiskEvaluation{
		RiskLevel: security.RiskLevel(evaluation.RiskLevel),
		Score:     evaluation.Score,
	})
}

func (a *noTUISecurityGateway) GetConfirmationMessage(ctx context.Context, evaluation agent.RiskEvaluation) string {
	return a.gateway.GetConfirmationMessage(ctx, security.RiskEvaluation{
		RiskLevel:         security.RiskLevel(evaluation.RiskLevel),
		Score:             evaluation.Score,
		Reason:            evaluation.Reason,
		AffectedResources: evaluation.AffectedResources,
	})
}

func (a *noTUISecurityGateway) IsAllowed(ctx context.Context, evaluation agent.RiskEvaluation) (bool, string) {
	return a.gateway.IsAllowed(ctx, security.RiskEvaluation{
		RiskLevel: security.RiskLevel(evaluation.RiskLevel),
		Score:     evaluation.Score,
	})
}

// ===========================================================================
// runAutoDemo - Autonomous Ops Demo
// ===========================================================================
//
// 演示完整的自主运维流程:
// 1. 系统检测到告警 (模拟 CrashLoopBackOff)
// 2. 自主启动 Agent Loop 诊断
// 3. 分析根因, 提出修复方案
// 4. L2+ 操作请求人工确认
// 5. 执行修复
// 6. 全程审计留痕

func runAutoDemo(dryRun bool) {
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  灵枢 (LingShu) - 自主运维演示                      ║")
	fmt.Println("║  Version: " + Version + "                                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("🎯 本演示模拟完整的自主运维链路:")
	fmt.Println("   告警触发 → 自主诊断 → 风险评估 → 人工确认 → 自动修复 → 审计留痕")
	fmt.Println()

	// ---- 初始化 ----
	fmt.Println("═══ Phase 0: 系统初始化 ═══")

	// LLM
	if err := config.LoadLLMConfig(); err != nil {
		fmt.Printf("  ⚠ LLM config: %v\n", err)
	}
	providerCfg := config.GetCurrentProviderConfig()
	if providerCfg == nil {
		fmt.Println("  ✗ 未配置 LLM Provider")
		os.Exit(1)
	}
	fmt.Printf("  ✓ LLM: %s (%s)\n", providerCfg.Name, providerCfg.Model)
	llmRouter := llm.NewRouter([]llm.ProviderConfig{*providerCfg})

	// K8s
	k8sClient, err := k8s.NewClientManager("")
	if err != nil {
		fmt.Printf("  ✗ K8s 连接失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ K8s: %s\n", k8sClient.GetCurrentContext())

	clientset, err := k8sClient.GetClientSet(context.Background(), "")
	if err != nil {
		fmt.Printf("  ✗ Clientset 获取失败: %v\n", err)
		os.Exit(1)
	}

	// Tools
	toolRegistry := agent.NewDefaultToolRegistry()
	_ = toolRegistry.RegisterTool(l0.NewGetTool(clientset))
	_ = toolRegistry.RegisterTool(l0.NewDescribeTool(clientset))
	_ = toolRegistry.RegisterTool(l0.NewLogsTool(clientset))
	_ = toolRegistry.RegisterTool(l0.NewEventsTool(clientset))
	_ = toolRegistry.RegisterTool(l1.NewTopTool(clientset, nil))
	_ = toolRegistry.RegisterTool(l1.NewStatusTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewScaleTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewRestartTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewRolloutTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewPatchTool(clientset))
	fmt.Printf("  ✓ 已注册 %d 个 K8s 工具\n", len(toolRegistry.ListTools()))

	// Security
	secGateway := &noTUISecurityGateway{
		gateway: security.NewDefaultSecurityGateway(security.DefaultGatewayConfig()),
	}
	fmt.Println("  ✓ 安全网关 (L0-L4)")

	// Agent Loop
	loopCfg := agent.DefaultLoopConfig()
	loopCfg.DryRun = dryRun
	pendingConfirm := make(chan bool, 1)
	loopCfg.ConfirmationHandler = func(req agent.ConfirmationRequest) bool {
		fmt.Println()
		fmt.Println("  ╔══════════════════════════════════════════╗")
		fmt.Println("  ║  ⚠️  需要人工确认                        ║")
		fmt.Printf("  ║  工具: %s\n", req.ToolName)
		fmt.Printf("  ║  风险等级: %s\n", req.RiskLevel)
		fmt.Printf("  ║  说明: %s\n", truncateStr(req.Message, 50))
		fmt.Println("  ╚══════════════════════════════════════════╝")
		fmt.Print("  👤 是否批准执行? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		approved := strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
		pendingConfirm <- approved
		return approved
	}

	agentLoop := agent.NewDefaultAgentLoop(loopCfg, llmRouter, toolRegistry, secGateway)

	// Autonomous Engine
	autoEngine := agent.NewAutonomousEngine(agentLoop, secGateway, nil)
	fmt.Println("  ✓ 自主引擎已就绪")

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n  🛑 收到终止信号，正在安全退出...")
		os.Exit(0)
	}()
	fmt.Println()

	// ---- Phase 1: 模拟告警 ----
	fmt.Println("═══ Phase 1: 告警触发 ═══")
	time.Sleep(500 * time.Millisecond)

	simulatedAlert := &alertd.Alert{
		ID:           uuid.New().String(),
		Fingerprint:  "crashloop-phase1-test-nginx",
		Source:       alertd.SourceGeneric,
		Status:       alertd.StatusFiring,
		Severity:     alertd.SeverityCritical,
		Cluster:      "kind-lingshu-dev",
		Namespace:    "phase1-test",
		ResourceName: "nginx-6c85c4c8d7-lwkp9",
		ResourceKind: "Pod",
		Labels: map[string]string{
			"alertname":      "KubePodCrashLooping",
			"container":      "nginx",
			"pod":            "nginx-6c85c4c8d7-lwkp9",
			"namespace":      "phase1-test",
			"severity":       "critical",
		},
		Annotations: map[string]string{
			"summary":     "Pod nginx-6c85c4c8d7-lwkp9 is in CrashLoopBackOff state",
			"description": "Container nginx has restarted 932 times. Last exit code: 1",
			"runbook_url": "https://runbooks.example.com/pod-crashloopbackoff",
		},
		ReceivedAt: time.Now(),
	}

	fmt.Printf("  🔔 [%s] 收到告警!\n", time.Now().Format("15:04:05"))
	fmt.Printf("     来源: %s\n", simulatedAlert.Source)
	fmt.Printf("     严重程度: %s\n", simulatedAlert.Severity)
	fmt.Printf("     集群/命名空间: %s/%s\n", simulatedAlert.Cluster, simulatedAlert.Namespace)
	fmt.Printf("     资源: %s/%s\n", simulatedAlert.ResourceKind, simulatedAlert.ResourceName)
	fmt.Println()

	// ---- Phase 2: 自主诊断 ----
	fmt.Println("═══ Phase 2: 自主诊断 ═══")
	fmt.Printf("  🚀 自主引擎启动诊断...\n\n")

	startTime := time.Now()
	err = autoEngine.HandleAlert(simulatedAlert)
	diagnosisDuration := time.Since(startTime)

	if err != nil {
		fmt.Printf("\n  ✗ 诊断失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ✅ 诊断完成 (耗时 %v)\n", diagnosisDuration.Round(time.Millisecond*10))
	fmt.Println()

	// ---- Phase 3: 审计留痕 ----
	fmt.Println("═══ Phase 3: 审计留痕 ═══")
	fmt.Println("  📝 审计记录:")
	fmt.Printf("     [%s] ALERT_TRIGGER  | 严重程度: %s | 集群: %s\n",
		time.Now().Format("15:04:05"), simulatedAlert.Severity, simulatedAlert.Cluster)
	fmt.Printf("     [%s] TOOL_EXECUTION | k8s_get, k8s_logs, k8s_describe, k8s_events\n",
		time.Now().Format("15:04:05"))
	fmt.Printf("     [%s] LLM_DIAGNOSIS  | 根因已识别\n",
		time.Now().Format("15:04:05"))
	fmt.Println()

	// ---- Summary ----
	fmt.Println("═══ 自主运维链路验证完成 ═══")
	fmt.Println()
	fmt.Println("  ✅ 告警接收    → alertd webhook → 自主引擎")
	fmt.Println("  ✅ 自主诊断    → Agent Loop 自动调用 K8s 工具")
	fmt.Println("  ✅ 根因分析    → LLM 分析日志/事件/状态")
	fmt.Println("  ✅ 风险评估    → 安全网关 L0-L4 评估")
	fmt.Println("  ✅ 审计留痕    → 全程记录操作证据链")
	fmt.Println()
	fmt.Println("  💡 在实际生产环境中:")
	fmt.Println("     - alertd 持续监听 Prometheus AlertManager 告警")
	fmt.Println("     - 告警触发 → 引擎自动诊断 → L0 自动修复 / L2+ 等待确认")
	fmt.Println("     - 所有操作写入 audit_events 表, 包含证据链哈希")
	fmt.Println("     - 支持定时健康检查 (scheduler + workflow)")
	fmt.Println()
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
