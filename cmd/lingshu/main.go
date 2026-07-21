package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lingshu/lingshu/pkg/agent"
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
	noTUI := flag.Bool("no-tui", false, "Disable TUI mode, use plain text output")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("lingshu version %s\n", Version)
		os.Exit(0)
	}

	if *noTUI {
		query := strings.Join(flag.Args(), " ")
		runNoTUI(query)
		return
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
func runNoTUI(query string) {
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
		return
	}

	// ---- Initialize ----
	fmt.Println("[1/4] Loading LLM config...")
	if err := config.LoadLLMConfig(); err != nil {
		fmt.Printf("  ⚠ Failed to load LLM config: %v\n", err)
	}
	providerCfg := config.GetCurrentProviderConfig()
	if providerCfg == nil {
		fmt.Println("  ✗ No LLM provider configured.")
		fmt.Println("  Set up ~/.lingshu/config.yaml or set OPENAI_API_KEY env var.")
		os.Exit(1)
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
		}
	} else {
		fmt.Println("  ⚠ Skipping K8s tools (no cluster access)")
	}

	secGateway := &noTUISecurityGateway{
		gateway: security.NewDefaultSecurityGateway(security.DefaultGatewayConfig()),
	}
	fmt.Println("  ✓ Security gateway ready (L0-L4)")

	loopCfg := agent.DefaultLoopConfig()
	loopCfg.ConfirmationHandler = func(req agent.ConfirmationRequest) bool {
		fmt.Printf("\n  ⚠ CONFIRMATION REQUIRED [%s]: %s\n", req.RiskLevel, req.Message)
		fmt.Print("  Confirm? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
	}

	agentLoop := agent.NewDefaultAgentLoop(loopCfg, llmRouter, toolRegistry, secGateway)
	fmt.Println("[4/4] Agent loop initialized.")
	fmt.Println()

	// ---- Execute ----
	fmt.Printf("🔍 Query: %s\n", query)
	fmt.Println(strings.Repeat("─", 60))

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

	fmt.Println(strings.Repeat("─", 60))
	if err != nil {
		fmt.Printf("\n✗ Agent execution failed: %v\n", err)
		os.Exit(1)
	}

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
