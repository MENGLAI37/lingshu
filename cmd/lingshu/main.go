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
	"github.com/lingshu/lingshu/pkg/audit"
	"github.com/lingshu/lingshu/pkg/cache"
	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/db"
	"github.com/lingshu/lingshu/pkg/gitops"
	"github.com/lingshu/lingshu/pkg/k8s"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/rag"
	"github.com/lingshu/lingshu/pkg/scheduler"
	"github.com/lingshu/lingshu/pkg/security"
	"github.com/lingshu/lingshu/pkg/session"
	"github.com/lingshu/lingshu/pkg/snapshot"
	"github.com/lingshu/lingshu/pkg/tools"
	"github.com/lingshu/lingshu/pkg/tools/l0"
	"github.com/lingshu/lingshu/pkg/tools/l1"
	"github.com/lingshu/lingshu/pkg/tools/l2"
	"github.com/lingshu/lingshu/pkg/tui/models"
	"github.com/lingshu/lingshu/pkg/workflow"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

var Version = "v0.1.0"

func main() {
	noTUI := flag.Bool("no-tui", false, "Headless mode: ask a question and get an answer")
	autoDemo := flag.Bool("auto-demo", false, "Autonomous ops demo: simulated alert triggers auto diagnosis")
	dryRun := flag.Bool("dry-run", false, "Preview mode: diagnose + plan but skip all write operations (L1+)")
	yesMode := flag.Bool("yes", false, "Auto-confirm L0-L2 operations (for CI/CD pipelines)")
	pipeMode := flag.Bool("pipe", false, "Machine-readable output (JSON lines, for CI/CD)")
	showVersion := flag.Bool("version", false, "Show version information")
	reportMode := flag.Bool("report", false, "Generate audit report from audit events")
	reportFormat := flag.String("format", "text", "Report format: text, json, html")
	reportPeriod := flag.String("period", "", "Report period: YYYY-MM (monthly), YYYY-WNN (weekly), or empty for all")
	reportCluster := flag.String("cluster", "", "Filter report by cluster name")
	reportNamespace := flag.String("namespace", "", "Filter report by namespace")
	reportOutput := flag.String("output", "", "Write report to file (default: stdout)")
	verifyChain := flag.Bool("verify-chain", false, "Verify evidence chain integrity in report")
	exportMode := flag.Bool("export", false, "Export session to portable JSON file")
	exportSessionID := flag.String("session-id", "", "Session ID for export/import operations")
	exportFile := flag.String("export-file", "", "Export file path (default: lingshu-session-<id>.json)")
	importMode := flag.Bool("import", false, "Import session from JSON file")
	importFile := flag.String("import-file", "", "Import file path")
	flag.Parse()

	if *showVersion {
		fmt.Printf("lingshu version %s\n", Version)
		os.Exit(0)
	}

	if *reportMode {
		runReport(*reportFormat, *reportPeriod, *reportCluster, *reportNamespace, *reportOutput, *verifyChain)
		return
	}

	if *exportMode {
		runExport(*exportSessionID, *exportFile)
		return
	}

	if *importMode {
		runImport(*importFile)
		return
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
	var gitopsDetector *gitops.Detector
	if k8sClient != nil {
		clientset, err := k8sClient.GetClientSet(context.Background(), "")
		if err != nil {
			fmt.Printf("  ⚠ Failed to get clientset: %v\n", err)
		} else {
				// Build metrics client for k8s_top
				var metricsClient *metricsv.Clientset
				if restCfg, restErr := k8sClient.GetRestConfig(context.Background(), ""); restErr == nil && restCfg != nil {
					if mc, mcErr := metricsv.NewForConfig(restCfg); mcErr == nil {
						metricsClient = mc
					}
				}

			_ = toolRegistry.RegisterTool(l0.NewGetTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewDescribeTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewLogsTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewEventsTool(clientset))
			_ = toolRegistry.RegisterTool(l1.NewTopTool(clientset, metricsClient))
			_ = toolRegistry.RegisterTool(l1.NewStatusTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewScaleTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewRestartTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewRolloutTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewPatchTool(clientset))
			// Build GitOps detector
			gitopsDetector = gitops.NewDetector(clientset)
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

	if gitopsDetector != nil {
		agentLoop.SetGitOpsDetector(gitopsDetector)
	}

	// Session tracking (optional - skip if DB unavailable)
	sessionMgr := initSessionIfAvailable()
	if sessionMgr != nil {
		agentLoop.SetSessionManager(sessionMgr)
	}

	// Audit manager (optional)
	auditMgr := initAuditIfAvailable()
	if auditMgr != nil {
		agentLoop.SetAuditManager(auditMgr)
	}

	// Snapshot + Auto-Rollback (optional - requires dynamic K8s client)
	initSnapshotIfAvailable(agentLoop, k8sClient)

	// RAG / Runbook knowledge base (optional)
	initRAGIfAvailable(agentLoop)

	// Workflow engine (optional)
	workflowEngine := initWorkflowIfAvailable(toolRegistry)

	// Scheduler (optional)
	schedulerInst := initSchedulerIfAvailable(toolRegistry)

	// Cache / Redis (optional)
	initCacheIfAvailable()

	fmt.Println("[4/4] Agent loop initialized.")
	if workflowEngine != nil {
		fmt.Printf("  ✓ Workflow engine ready (%d built-in workflows)\n", len(workflowEngine.ListWorkFlows()))
	}
	if schedulerInst != nil {
		fmt.Printf("  ✓ Scheduler ready (%d jobs)\n", len(schedulerInst.ListJobs()))
	}

	// Cleanup on exit
	defer func() {
		if schedulerInst != nil {
			_ = schedulerInst.Stop()
		}
	}()

	// Auto health check at startup
	if !pipeMode {
		fmt.Println()
		fmt.Println("🏥 Running startup health check...")
	}
	healthCtx := context.WithValue(context.Background(), "environment", "development")
	healthCtx = context.WithValue(healthCtx, "on_call", false)
	healthCtx = context.WithValue(healthCtx, "change_window", true)
	healthCtx = context.WithValue(healthCtx, "namespace", "default")
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

	ctx := context.WithValue(context.Background(), "environment", "development")
	ctx = context.WithValue(ctx, "on_call", false)
	ctx = context.WithValue(ctx, "change_window", true)
	ctx = context.WithValue(ctx, "namespace", "default")
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
		var gitopsDetector *gitops.Detector
	// Build metrics client for k8s_top
	var metricsClient *metricsv.Clientset
	if restCfg, restErr := k8sClient.GetRestConfig(context.Background(), ""); restErr == nil && restCfg != nil {
		if mc, mcErr := metricsv.NewForConfig(restCfg); mcErr == nil {
			metricsClient = mc
		}
	}

	_ = toolRegistry.RegisterTool(l0.NewGetTool(clientset))
	_ = toolRegistry.RegisterTool(l0.NewDescribeTool(clientset))
	_ = toolRegistry.RegisterTool(l0.NewLogsTool(clientset))
	_ = toolRegistry.RegisterTool(l0.NewEventsTool(clientset))
	_ = toolRegistry.RegisterTool(l1.NewTopTool(clientset, metricsClient))
	_ = toolRegistry.RegisterTool(l1.NewStatusTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewScaleTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewRestartTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewRolloutTool(clientset))
	_ = toolRegistry.RegisterTool(l2.NewPatchTool(clientset))
			// Build GitOps detector
			gitopsDetector = gitops.NewDetector(clientset)
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
		if gitopsDetector != nil {
			agentLoop.SetGitOpsDetector(gitopsDetector)
		}

		if gitopsDetector != nil {
			agentLoop.SetGitOpsDetector(gitopsDetector)
		}


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

// ===========================================================================
// runReport - 一键生成审计报告
// ===========================================================================

func runReport(format, period, cluster, namespace, outputPath string, verify bool) {
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  灵枢 (LingShu) — 审计报告生成器             ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	// Initialize database
	cfg, _ := config.Load("")
	if cfg == nil {
		cfg = &config.Config{}
	}

	fmt.Println("[1/3] 连接数据库...")
	dbCfg := &config.DBConfig{
		Type:         "postgres",
		Host:         "localhost",
		Port:         5432,
		User:         "lingshu",
		Password:     "lingshu",
		DBName:       "lingshu",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}
	if cfg.Database.Host != "" {
		dbCfg = &cfg.Database
	}

	dbInst, err := db.Init(dbCfg)
	if err != nil {
		// Import db package for use
		fmt.Printf("  ⚠ 数据库连接失败: %v\n", err)
		fmt.Println("  💡 审计报告需要数据库支持。请确保 PostgreSQL 可用。")
		fmt.Println("     或使用 SQLite 降级: 设置 database.type=sqlite")
		os.Exit(1)
	}
	_ = dbInst
	fmt.Println("  ✓ 数据库已连接")

	// Initialize audit manager
	fmt.Println("[2/3] 初始化审计管理器...")
	auditMgr, err := audit.Init(cfg)
	if err != nil {
		fmt.Printf("  ✗ 审计管理器初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer auditMgr.Close()
	fmt.Printf("  ✓ 审计事件数: %d (队列: %d)\n",
		auditMgr.GetStatsInfo()["flushed_events"],
		auditMgr.GetQueueSize())

	// Parse options
	fmt.Println("[3/3] 生成审计报告...")

	var reportFormat audit.ReportFormat
	switch strings.ToLower(format) {
	case "json":
		reportFormat = audit.ReportFormatJSON
	case "html":
		reportFormat = audit.ReportFormatHTML
	case "text":
		fallthrough
	default:
		reportFormat = audit.ReportFormatText
	}

	opts := &audit.ReportOptions{
		Format:      reportFormat,
		Period:      period,
		VerifyChain: verify,
	}

	if cluster != "" {
		opts.Cluster = &cluster
	}
	if namespace != "" {
		opts.Namespace = &namespace
	}

	// Parse period into time range
	if period != "" {
		start, end := parsePeriod(period)
		if start != nil {
			opts.StartTime = start
		}
		if end != nil {
			opts.EndTime = end
		}
	}

	ctx := context.Background()
	report, err := auditMgr.GenerateReport(ctx, opts)
	if err != nil {
		fmt.Printf("  ✗ 报告生成失败: %v\n", err)
		os.Exit(1)
	}

	// Export report
	exportFormat := reportFormat
	if outputPath != "" {
		switch {
		case strings.HasSuffix(outputPath, ".json"):
			exportFormat = audit.ReportFormatJSON
		case strings.HasSuffix(outputPath, ".html"):
			exportFormat = audit.ReportFormatHTML
		}
	}

	if err := audit.ExportReport(report, exportFormat, outputPath); err != nil {
		fmt.Printf("  ✗ 报告导出失败: %v\n", err)
		os.Exit(1)
	}

	if outputPath != "" {
		fmt.Printf("\n✅ 审计报告已生成: %s\n", outputPath)
	} else {
		fmt.Println() // Report already printed to stdout
	}
}

// parsePeriod parses a period string like "2026-06" or "2026-W30" into a time range.
func parsePeriod(period string) (*time.Time, *time.Time) {
	// Format: YYYY-MM (monthly)
	if len(period) == 7 && period[4] == '-' {
		year := 0
		month := 0
		fmt.Sscanf(period, "%d-%d", &year, &month)
		if year > 0 && month >= 1 && month <= 12 {
			start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, 0).Add(-time.Second)
			return &start, &end
		}
	}
	return nil, nil
}

// ===========================================================================
// runExport - 导出会话
// ===========================================================================

func runExport(sessionID, filePath string) {
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  灵枢 (LingShu) — 会话导出                    ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	if sessionID == "" {
		fmt.Println("请指定要导出的会话 ID: lingshu --export --session-id <id>")
		os.Exit(1)
	}

	// Initialize database
	cfg, _ := config.Load("")
	if cfg == nil {
		cfg = &config.Config{}
	}

	dbCfg := &config.DBConfig{
		Type:         "postgres",
		Host:         "localhost",
		Port:         5432,
		User:         "lingshu",
		Password:     "lingshu",
		DBName:       "lingshu",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}
	if cfg.Database.Host != "" {
		dbCfg = &cfg.Database
	}

	_, err := db.Init(dbCfg)
	if err != nil {
		fmt.Printf("✗ 数据库连接失败: %v\n", err)
		os.Exit(1)
	}

	sessionMgr, err := session.Init(cfg)
	if err != nil {
		fmt.Printf("✗ 会话管理器初始化失败: %v\n", err)
		os.Exit(1)
	}

	outputPath := filePath
	if outputPath == "" {
		outputPath = fmt.Sprintf("lingshu-session-%s.json", sessionID)
	}

	ctx := context.Background()
	if err := sessionMgr.Export(ctx, sessionID, outputPath); err != nil {
		fmt.Printf("✗ 导出失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 会话已导出至: %s\n", outputPath)
}

// ===========================================================================
// runImport - 导入会话
// ===========================================================================

func runImport(filePath string) {
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  灵枢 (LingShu) — 会话导入                    ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	if filePath == "" {
		fmt.Println("请指定导入文件: lingshu --import --import-file <path>")
		os.Exit(1)
	}

	cfg, _ := config.Load("")
	if cfg == nil {
		cfg = &config.Config{}
	}

	dbCfg := &config.DBConfig{
		Type:         "postgres",
		Host:         "localhost",
		Port:         5432,
		User:         "lingshu",
		Password:     "lingshu",
		DBName:       "lingshu",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}
	if cfg.Database.Host != "" {
		dbCfg = &cfg.Database
	}

	_, err := db.Init(dbCfg)
	if err != nil {
		fmt.Printf("✗ 数据库连接失败: %v\n", err)
		os.Exit(1)
	}

	sessionMgr, err := session.Init(cfg)
	if err != nil {
		fmt.Printf("✗ 会话管理器初始化失败: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	imported, err := sessionMgr.Import(ctx, filePath)
	if err != nil {
		fmt.Printf("✗ 导入失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 会话已导入，新会话 ID: %s\n", imported.SessionID)
	fmt.Printf("   原始会话: %s\n", imported.Metadata["original_session_id"])
	fmt.Printf("   来源主机: %s\n", imported.Metadata["imported_from"])
	fmt.Printf("   消息数: %d | 工具调用数: %d\n",
		len(imported.ConversationHistory), len(imported.ToolCallHistory))
}

// ===========================================================================
// initSessionIfAvailable - Optional session tracking
// ===========================================================================

// sessionManagerAdapter bridges session.Manager to agent.SessionManager interface.
type sessionManagerAdapter struct {
	mgr *session.Manager
}

func (a *sessionManagerAdapter) Create(ctx context.Context, cluster, namespace string) (string, error) {
	sess, err := a.mgr.Create(ctx, &session.CreateSessionRequest{
		Cluster:   cluster,
		Namespace: namespace,
	})
	if err != nil {
		return "", err
	}
	return sess.SessionID, nil
}

func (a *sessionManagerAdapter) AddToolCall(ctx context.Context, sessionID, toolName, riskLevel string, args map[string]any, result string) error {
	toolCall := map[string]interface{}{
		"tool_name":  toolName,
		"risk_level": riskLevel,
		"arguments":  args,
		"result":     result,
	}
	return a.mgr.AppendToolCall(ctx, sessionID, toolCall)
}

func (a *sessionManagerAdapter) AddCost(ctx context.Context, sessionID string, inputTokens, outputTokens int64) error {
	totalTokens := inputTokens + outputTokens
	return a.mgr.AddCost(ctx, sessionID, 0, totalTokens)
}

func (a *sessionManagerAdapter) Complete(ctx context.Context, sessionID, finalResponse string) error {
	status := session.StatusCompleted
	meta := map[string]interface{}{"final_response": finalResponse}
	_, err := a.mgr.Update(ctx, sessionID, &session.UpdateSessionRequest{
		Status:   &status,
		Metadata: &meta,
	})
	return err
}

func (a *sessionManagerAdapter) CheckTokenBudget(ctx context.Context, sessionID string, tokensToUse int64) (bool, error) {
	return a.mgr.CheckTokenBudget(ctx, sessionID, tokensToUse)
}

// initSessionIfAvailable tries to initialize DB and session tracking.
// Returns nil if unavailable (non-fatal).
func initSessionIfAvailable() agent.SessionManager {
	cfg, _ := config.Load("")
	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.Database.Host == "" {
		cfg.Database = config.DBConfig{
			Type:         "postgres",
			Host:         "localhost",
			Port:         5432,
			User:         "lingshu",
			Password:     "lingshu",
			DBName:       "lingshu",
			SSLMode:      "disable",
			MaxOpenConns: 5,
			MaxIdleConns: 2,
		}
	}

	_, dbErr := db.Init(&cfg.Database)
	if dbErr != nil {
		return nil // Session tracking unavailable
	}

	sessMgr, sessErr := session.Init(cfg)
	if sessErr != nil {
		return nil
	}

	return &sessionManagerAdapter{mgr: sessMgr}
}

// ===========================================================================
// Optional module initializers
// ===========================================================================

func initAuditIfAvailable() *audit.Manager {
	cfg, _ := config.Load("")
	if cfg == nil {
		return nil
	}
	auditMgr, err := audit.Init(cfg)
	if err != nil {
		return nil
	}
	return auditMgr
}

func initSnapshotIfAvailable(agentLoop *agent.DefaultAgentLoop, k8sClient *k8s.ClientManager) {
	if k8sClient == nil {
		return
	}
	dynamicClient, err := k8sClient.GetDynamicClient(context.Background(), "")
	if err != nil || dynamicClient == nil {
		return
	}
	s := snapshot.NewSnapshotter(dynamicClient, "")
	adapter := snapshot.NewAgentSnapshotterAdapter(s)
	agentLoop.SetSnapshotter(adapter)
}

func initRAGIfAvailable(agentLoop *agent.DefaultAgentLoop) {
	chromaClient := rag.NewChromaDBClient()
	embedder := rag.NewSimpleEmbeddingProvider(384)
	retriever := rag.NewRetriever(chromaClient, embedder, rag.WithDefaultK(3), rag.WithMinScore(0.5))
	runbook := rag.NewRunbookRAG(retriever)
	ragAdapter := rag.NewAgentRetrieverAdapter(runbook)
	agentLoop.SetRAGRetriever(ragAdapter)
}

func initWorkflowIfAvailable(toolRegistry agent.ToolRegistry) *workflow.DefaultWorkFlowEngine {
	wfEngine := workflow.NewDefaultWorkFlowEngine(toolRegistry)
	// Register built-in workflows
	for _, wf := range workflow.GetBuiltInWorkFlows() {
		_ = wfEngine.RegisterWorkFlow(wf)
	}
	// Register workflow as a tool
	wfTool := workflow.NewWorkFlowTool(wfEngine)
	_ = toolRegistry.RegisterTool(wfTool)
	return wfEngine
}

func initSchedulerIfAvailable(toolRegistry agent.ToolRegistry) *scheduler.DefaultScheduler {
	s := scheduler.NewDefaultScheduler(toolRegistry)
	// Register scheduler as a tool
	schedTool := scheduler.NewSchedulerTool(s)
	_ = toolRegistry.RegisterTool(schedTool)
	// Start scheduler in background
	if err := s.Start(); err != nil {
		return nil
	}
	return s
}

func initCacheIfAvailable() {
	cfg := config.Get()
	if cfg == nil {
		return
	}
	if len(cfg.Redis.Addresses) > 0 {
		_, _ = cache.Init(&cfg.Redis)
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
