package workflow

func GetBuiltInWorkFlows() []*WorkFlow {
	return []*WorkFlow{
		PodRestartDiagnosisWorkflow(),
		DeploymentHealthCheckWorkflow(),
		NodeResourceAlertWorkflow(),
	}
}

func PodRestartDiagnosisWorkflow() *WorkFlow {
	return &WorkFlow{
		ID:          "pod_restart_diagnosis",
		Name:        "Pod 重启诊断工作流",
		Description: "自动化诊断 Pod 重启原因并尝试修复",
		Version:     "1.0",
		StartStep:   "get_pod_status",
		Variables: map[string]interface{}{
			"namespace": "default",
			"pod_name":  "",
			"restart_count": 0,
			"has_issues":   false,
		},
		Steps: map[string]*Step{
			"get_pod_status": {
				ID:          "get_pod_status",
				Name:        "获取 Pod 状态",
				Description: "查询目标 Pod 的当前状态和重启次数",
				Condition: &Condition{
					Type: ConditionAlways,
				},
				Actions: []Action{
					{
						Type:       ActionToolCall,
						ToolName:   "k8s_get",
						ToolParams: map[string]interface{}{"resource_type": "pod", "name": "{{pod_name}}", "namespace": "{{namespace}}"},
					},
				},
				NextStep: "check_restart_count",
			},
			"check_restart_count": {
				ID:          "check_restart_count",
				Name:        "检查重启次数",
				Description: "判断 Pod 是否存在重启问题",
				Condition: &Condition{
					Type:     ConditionGreater,
					Variable: "restart_count",
					Expected: 0,
				},
				Actions: []Action{
					{
						Type:         ActionSetVariable,
						VariableName: "has_issues",
						VariableValue: true,
					},
					{
						Type:       ActionLog,
						Message:    "检测到 Pod 重启，开始诊断...",
					},
				},
				NextStep: "get_pod_logs",
				OnFailure: "no_issues",
			},
			"get_pod_logs": {
				ID:          "get_pod_logs",
				Name:        "获取 Pod 日志",
				Description: "获取 Pod 最近日志以分析错误原因",
				Condition: &Condition{
					Type:     ConditionEquals,
					Variable: "has_issues",
					Expected: true,
				},
				Actions: []Action{
					{
						Type:       ActionToolCall,
						ToolName:   "k8s_logs",
						ToolParams: map[string]interface{}{"pod_name": "{{pod_name}}", "namespace": "{{namespace}}", "tail_lines": 200},
					},
				},
				NextStep: "get_pod_events",
			},
			"get_pod_events": {
				ID:          "get_pod_events",
				Name:        "获取相关事件",
				Description: "查询 Pod 相关的 Kubernetes 事件",
				Condition: &Condition{
					Type: ConditionAlways,
				},
				Actions: []Action{
					{
						Type:       ActionToolCall,
						ToolName:   "k8s_events",
						ToolParams: map[string]interface{}{"namespace": "{{namespace}}"},
					},
				},
				NextStep: "describe_pod",
			},
			"describe_pod": {
				ID:          "describe_pod",
				Name:        "描述 Pod",
				Description: "获取 Pod 的详细描述信息",
				Condition: &Condition{
					Type: ConditionAlways,
				},
				Actions: []Action{
					{
						Type:       ActionToolCall,
						ToolName:   "k8s_describe",
						ToolParams: map[string]interface{}{"resource_type": "pod", "name": "{{pod_name}}", "namespace": "{{namespace}}"},
					},
				},
				NextStep: "check_memory_issue",
			},
			"check_memory_issue": {
				ID:          "check_memory_issue",
				Name:        "检查内存问题",
				Description: "判断是否是 OOM 导致重启",
				Condition: &Condition{
					Type:     ConditionContains,
					Variable: "last_logs",
					Expected: "OOMKilled",
				},
				Actions: []Action{
					{
						Type:       ActionLog,
						Message:    "检测到 OOMKilled，建议调整资源限制",
					},
				},
				NextStep: "suggest_fix",
				OnFailure: "check_crash_loop",
			},
			"check_crash_loop": {
				ID:          "check_crash_loop",
				Name:        "检查 CrashLoopBackOff",
				Description: "判断是否是 CrashLoopBackOff",
				Condition: &Condition{
					Type:     ConditionContains,
					Variable: "pod_status",
					Expected: "CrashLoopBackOff",
				},
				Actions: []Action{
					{
						Type:       ActionLog,
						Message:    "检测到 CrashLoopBackOff，应用进程持续崩溃",
					},
				},
				NextStep: "suggest_fix",
				OnFailure: "suggest_fix",
			},
			"suggest_fix": {
				ID:          "suggest_fix",
				Name:        "建议修复方案",
				Description: "根据诊断结果给出修复建议",
				Condition: &Condition{
					Type: ConditionAlways,
				},
				Actions: []Action{
					{
						Type:       ActionLog,
						Message:    "诊断完成。请查看日志和事件分析重启原因，可能的修复方案：1) 调整资源限制 2) 修复应用代码 3) 检查配置 4) 查看依赖服务",
					},
					{
						Type: ActionExit,
					},
				},
				NextStep: "",
			},
			"no_issues": {
				ID:          "no_issues",
				Name:        "无问题",
				Description: "Pod 没有重启问题",
				Condition: &Condition{
					Type: ConditionAlways,
				},
				Actions: []Action{
					{
						Type:       ActionLog,
						Message:    "Pod 运行正常，未检测到重启问题",
					},
					{
						Type: ActionExit,
					},
				},
				NextStep: "",
			},
		},
	}
}

func DeploymentHealthCheckWorkflow() *WorkFlow {
	return &WorkFlow{
		ID:          "deployment_health_check",
		Name:        "Deployment 健康检查工作流",
		Description: "检查 Deployment 的健康状态",
		Version:     "1.0",
		StartStep:   "get_deployment",
		Variables: map[string]interface{}{
			"namespace":     "default",
			"deployment_name": "",
			"ready_replicas": 0,
			"available_replicas": 0,
			"desired_replicas":   0,
		},
		Steps: map[string]*Step{
			"get_deployment": {
				ID:          "get_deployment",
				Name:        "获取 Deployment",
				Description: "查询 Deployment 状态",
				Condition: &Condition{Type: ConditionAlways},
				Actions: []Action{
					{
						Type:       ActionToolCall,
						ToolName:   "k8s_get",
						ToolParams: map[string]interface{}{"resource_type": "deployment", "name": "{{deployment_name}}", "namespace": "{{namespace}}"},
					},
				},
				NextStep: "check_availability",
			},
			"check_availability": {
				ID:   "check_availability",
				Name: "检查可用性",
				Condition: &Condition{
					Type:     ConditionNotEquals,
					Variable: "ready_replicas",
					Expected: "{{desired_replicas}}",
				},
				Actions: []Action{
					{Type: ActionLog, Message: "Deployment 不可用，副本数不匹配"},
				},
				NextStep: "get_events",
				OnFailure: "health_ok",
			},
			"get_events": {
				ID:   "get_events",
				Name: "获取事件",
				Condition: &Condition{Type: ConditionAlways},
				Actions: []Action{
					{Type: ActionToolCall, ToolName: "k8s_events", ToolParams: map[string]interface{}{"namespace": "{{namespace}}"}},
				},
				NextStep: "exit",
			},
			"health_ok": {
				ID:   "health_ok",
				Name: "健康正常",
				Condition: &Condition{Type: ConditionAlways},
				Actions: []Action{
					{Type: ActionLog, Message: "Deployment 健康状态正常"},
					{Type: ActionExit},
				},
				NextStep: "",
			},
			"exit": {
				ID:   "exit",
				Name: "退出",
				Condition: &Condition{Type: ConditionAlways},
				Actions: []Action{{Type: ActionExit}},
				NextStep: "",
			},
		},
	}
}

func NodeResourceAlertWorkflow() *WorkFlow {
	return &WorkFlow{
		ID:          "node_resource_alert",
		Name:        "节点资源告警工作流",
		Description: "检查节点资源使用情况",
		Version:     "1.0",
		StartStep:   "get_top_nodes",
		Variables: map[string]interface{}{
			"cpu_threshold": 80,
			"mem_threshold": 85,
		},
		Steps: map[string]*Step{
			"get_top_nodes": {
				ID:   "get_top_nodes",
				Name: "获取节点资源",
				Condition: &Condition{Type: ConditionAlways},
				Actions: []Action{
					{Type: ActionToolCall, ToolName: "k8s_top", ToolParams: map[string]interface{}{"resource_type": "node", "all_namespaces": true}},
				},
				NextStep: "check_resources",
			},
			"check_resources": {
				ID:   "check_resources",
				Name: "检查资源",
				Condition: &Condition{Type: ConditionAlways},
				Actions: []Action{
					{Type: ActionLog, Message: "节点资源检查完成"},
					{Type: ActionExit},
				},
				NextStep: "",
			},
		},
	}
}
