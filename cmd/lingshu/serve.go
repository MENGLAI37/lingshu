package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/alertd"
	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/gitops"
	"github.com/lingshu/lingshu/pkg/k8s"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/scheduler"
	"github.com/lingshu/lingshu/pkg/security"
	"github.com/lingshu/lingshu/pkg/tools/l0"
	"github.com/lingshu/lingshu/pkg/tools/l1"
	"github.com/lingshu/lingshu/pkg/tools/l2"

	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// runServe starts the autonomous daemon with REPL.
func runServe(dryRun, yesMode bool) {
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  灵枢 (LingShu) - 自主运维守护进程                  ║")
	fmt.Println("║  Version: " + Version + "                                     ║")
	fmt.Println("║  Mode: Daemon (--serve)                             ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("🎯 自主运维链路:")
	fmt.Println("   外部告警 → webhook 接收 → 自主诊断 → 风险评估 → 自动修复 → 审计留痕")
	fmt.Println()

	// ---- Phase 1: LLM + K8s + Tools + Security ----
	fmt.Println("═══ [1/5] 初始化核心组件 ═══")

	if err := config.LoadLLMConfig(); err != nil {
		fmt.Printf("  ⚠ LLM config: %v\n", err)
	}
	providerCfg := config.GetCurrentProviderConfig()
	if providerCfg == nil {
		fmt.Println("  ✗ 未配置 LLM Provider")
		fmt.Println("    请设置 OPENAI_API_KEY 或 DEEPSEEK_API_KEY 环境变量")
		return
	}
	fmt.Printf("  ✓ LLM: %s (model: %s)\n", providerCfg.Name, providerCfg.Model)
	llmRouter := llm.NewRouter([]llm.ProviderConfig{*providerCfg})

	k8sClient, err := k8s.NewClientManager("")
	if err != nil {
		fmt.Printf("  ⚠ K8s 连接失败: %v\n", err)
		fmt.Println("  ⚠ 继续运行但无集群操作能力...")
	} else {
		fmt.Printf("  ✓ K8s 已连接 (context: %s)\n", k8sClient.GetCurrentContext())
	}

	toolRegistry := agent.NewDefaultToolRegistry()
	var gitopsDetector *gitops.Detector
	if k8sClient != nil {
		clientset, err := k8sClient.GetClientSet(context.Background(), "")
		if err != nil {
			fmt.Printf("  ⚠ Clientset 获取失败: %v\n", err)
		} else {
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
			gitopsDetector = gitops.NewDetector(clientset)
			fmt.Printf("  ✓ 已注册 %d 个 K8s 工具\n", len(toolRegistry.ListTools()))
		}
	}

	secGateway := &noTUISecurityGateway{
		gateway: security.NewDefaultSecurityGateway(security.DefaultGatewayConfig()),
	}

	var approvalPolicy agent.AutoApprovePolicy
	if yesMode {
		approvalPolicy = agent.AutoApproveAll
	} else {
		approvalPolicy = agent.AutoApproveSafe
	}
	fmt.Printf("  ✓ 安全网关 (L0-L4) | 自主确认策略: %s\n", approvalPolicy)

	// ---- Phase 2: DB + Session + Audit ----
	fmt.Println("═══ [2/5] 初始化数据层 ═══")
	sessionMgr := initSessionIfAvailable()
	auditMgr := initAuditIfAvailable()
	if sessionMgr != nil {
		fmt.Println("  ✓ 会话管理就绪")
	}
	if auditMgr != nil {
		fmt.Println("  ✓ 审计管理器就绪")
		defer func() { _ = auditMgr.Close() }()
	}

	// ---- Phase 3: Agent Loop + Autonomous Engine ----
	fmt.Println("═══ [3/5] 初始化自主引擎 ═══")

	loopCfg := agent.DefaultLoopConfig()
	loopCfg.DryRun = dryRun
	loopCfg.MaxIterations = 20
	loopCfg.MaxL2Operations = 10

	agentLoop := agent.NewDefaultAgentLoop(loopCfg, llmRouter, toolRegistry, secGateway)
	if gitopsDetector != nil {
		agentLoop.SetGitOpsDetector(gitopsDetector)
	}
	if sessionMgr != nil {
		agentLoop.SetSessionManager(sessionMgr)
	}
	if auditMgr != nil {
		agentLoop.SetAuditManager(auditMgr)
	}

	autoEngine := agent.NewAutonomousEngine(agentLoop, secGateway, auditMgr)
	autoEngine.SetApprovalPolicy(approvalPolicy)
	agentLoop.SetConfirmationHandler(autoEngine.Confirmer())
	fmt.Println("  ✓ 自主引擎就绪")

	initSnapshotIfAvailable(agentLoop, k8sClient)
	initRAGIfAvailable(agentLoop)
	workflowEngine := initWorkflowIfAvailable(toolRegistry)
	schedulerInst := initSchedulerIfAvailable(toolRegistry)
	initCacheIfAvailable()

	if workflowEngine != nil {
		fmt.Printf("  ✓ Workflow 引擎 (%d 个工作流)\n", len(workflowEngine.ListWorkFlows()))
	}
	if schedulerInst != nil {
		fmt.Printf("  ✓ 调度器 (%d 个任务)\n", len(schedulerInst.ListJobs()))
	}

	// ---- Phase 4: Alertd Server ----
	fmt.Println("═══ [4/5] 启动告警 Webhook 服务器 ═══")

	cfg, _ := config.Load("")
	if cfg == nil {
		cfg = &config.Config{}
	}
	alertdServer, err := alertd.Init(cfg)
	if err != nil {
		fmt.Printf("  ✗ alertd 初始化失败: %v\n", err)
		return
	}
	alertdServer.RegisterHandler(autoEngine.HandleAlert)
	fmt.Println("  ✓ 自主引擎已注册为告警处理器")

	if err := alertdServer.Start(); err != nil {
		fmt.Printf("  ✗ alertd 启动失败: %v\n", err)
		return
	}

	addr := alertdServer.GetAddr()
	fmt.Println()
	fmt.Println("  🔔 告警 Webhook 端点:")
	fmt.Printf("     Generic Alert:     http://%s/api/v1/alerts\n", addr)
	fmt.Printf("     AlertManager:       http://%s/api/v1/webhook/alertmanager\n", addr)
	fmt.Printf("     PagerDuty:          http://%s/api/v1/webhook/pagerduty\n", addr)
	fmt.Printf("     Health Check:       http://%s/healthz\n", addr)

	// ---- Phase 5: Startup Health Check + Cluster Watcher ----
	fmt.Println()
	fmt.Println("═══ [5/5] 启动自主巡检 ═══")

	if len(toolRegistry.ListTools()) == 0 {
		fmt.Println("  ⚠ 无可用工具，跳过集群健康检查")
	} else {
		healthCtx := context.WithValue(context.Background(), "environment", "development")
		healthCtx = context.WithValue(healthCtx, "on_call", true)
		healthCtx = context.WithValue(healthCtx, "namespace", "default")
		healthResult, healthErr := agentLoop.Execute(healthCtx,
			"快速检查集群状态：使用已有的 K8s 工具获取节点和 Pod 状态，一句话总结即可。",
			func(event agent.LoopEvent) {})
		if healthErr == nil && healthResult.FinalResponse != "" {
			fmt.Printf("  ✓ 集群状态: %s\n", truncateStr(healthResult.FinalResponse, 120))
		} else if healthErr != nil {
			fmt.Printf("  ⚠ 健康检查失败: %v\n", healthErr)
		}
	}

	watcherStop := make(chan struct{})
	if k8sClient != nil {
		go startClusterWatcher(k8sClient, autoEngine, watcherStop)
	}

	if schedulerInst != nil && k8sClient != nil {
		_ = schedulerInst.AddJob(&scheduler.ScheduledJob{
			ID:       "autonomous-health-check",
			Name:     "自主健康检查",
			Type:     scheduler.JobTypeHealthCheck,
			Interval: 5 * time.Minute,
			ToolName: "k8s_status",
			ToolParams: map[string]interface{}{
				"resource_type": "nodes",
			},
			Enabled: true,
		})
		fmt.Println("  ✓ 定时健康检查已注册 (每5分钟)")
	}

	fmt.Println("  🔍 集群自主巡检已启动 — 实时监控 Pod 异常、CrashLoop、OOMKilled")
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  🟢 灵枢守护进程已就绪                               ║")
	fmt.Println("║                                                      ║")
	fmt.Println("║  🔍 后台自主巡检中   💬 输入查询即交互               ║")
	fmt.Println("║  输入 exit 退出   Ctrl+C 安全退出                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	// ---- REPL: 交互 + 后台自主巡检 并存 ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("lingshu> ")

replLoop:
	for scanner.Scan() {
		select {
		case sig := <-sigCh:
			fmt.Printf("\n\n  🛑 收到 %s 信号, 正在安全退出...\n", sig.String())
			break replLoop
		default:
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			fmt.Print("lingshu> ")
			continue
		}

		switch strings.ToLower(input) {
		case "exit", "quit":
			break replLoop
		case "help", "?":
			fmt.Println()
			fmt.Println("  💡 可用命令:")
			fmt.Println("     <自然语言查询>   直接描述你想了解的集群问题")
			fmt.Println("     help, ?          显示此帮助")
			fmt.Println("     status           查看守护进程状态")
			fmt.Println("     exit, quit       安全退出")
			fmt.Println()
			fmt.Print("lingshu> ")
			continue
		case "status":
			fmt.Println()
			fmt.Printf("  🟢 守护进程: 运行中\n")
			fmt.Printf("  📡 Webhook:  http://%s/api/v1/alerts\n", addr)
			fmt.Printf("  🔍 巡检:    活跃 (每30秒)\n")
			fmt.Printf("  🔧 工具:    %d 个已注册\n", len(toolRegistry.ListTools()))
			if schedulerInst != nil {
				fmt.Printf("  ⏰ 定时任务: %d 个\n", len(schedulerInst.ListJobs()))
			}
			fmt.Println()
			fmt.Print("lingshu> ")
			continue
		}

		// Execute user query
		fmt.Println()
		ctx := context.WithValue(context.Background(), "environment", "development")
		ctx = context.WithValue(ctx, "on_call", false)
		ctx = context.WithValue(ctx, "namespace", "default")
		ctx = context.WithValue(ctx, "change_window", true)

		result, err := agentLoop.Execute(ctx, input, func(event agent.LoopEvent) {
			switch event.Type {
			case "tool_result":
				if r, ok := event.Data.(agent.ToolExecutionResult); ok {
					if r.Error != nil {
						fmt.Printf("  ✗ %s [%s]: %v\n", r.ToolName, r.Duration, r.Error)
					} else {
						fmt.Printf("  ✓ %s [%s]\n", r.ToolName, r.Duration)
					}
				}
			case "confirmation_request":
				fmt.Println("  🔒 操作需要安全策略审批")
			}
		})

		fmt.Println()
		if err != nil {
			fmt.Printf("  ✗ 执行错误: %v\n", err)
		} else {
			fmt.Printf("  %s\n", result.FinalResponse)
		}
		fmt.Println()

		fmt.Print("lingshu> ")
	}

	fmt.Println()

	// ---- Graceful Shutdown ----
	close(watcherStop)
	fmt.Println("  ⏳ 停止集群巡检...")
	fmt.Println("  ⏳ 停止告警服务器...")
	if err := alertdServer.Stop(); err != nil {
		fmt.Printf("  ⚠ alertd 停止异常: %v\n", err)
	}
	fmt.Println("  ✓ 告警服务器已停止")

	if schedulerInst != nil {
		fmt.Println("  ⏳ 停止调度器...")
		_ = schedulerInst.Stop()
		fmt.Println("  ✓ 调度器已停止")
	}

	fmt.Println()
	fmt.Println("  ✅ 灵枢守护进程已安全退出")
}
