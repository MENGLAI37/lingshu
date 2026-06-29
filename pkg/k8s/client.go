package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type ClientManager struct {
	mu            sync.RWMutex
	configs       map[string]*rest.Config
	clientsets    map[string]*kubernetes.Clientset
	dynamicClients map[string]*dynamic.DynamicClient
	currentContext string
	kubeconfigPath string
}

type ClusterInfo struct {
	Name     string
	Server   string
	Insecure bool
}

func NewClientManager(kubeconfigPath string) (*ClientManager, error) {
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
			kubeconfigPath = envKubeconfig
		}
	}

	cm := &ClientManager{
		configs:        make(map[string]*rest.Config),
		clientsets:     make(map[string]*kubernetes.Clientset),
		dynamicClients: make(map[string]*dynamic.DynamicClient),
		kubeconfigPath: kubeconfigPath,
	}

	if err := cm.loadContexts(); err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig contexts: %w", err)
	}

	return cm, nil
}

func (cm *ClientManager) loadContexts() error {
	config, err := clientcmd.LoadFromFile(cm.kubeconfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cm.loadInClusterConfig()
		}
		return fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	if config.CurrentContext != "" {
		cm.currentContext = config.CurrentContext
	}

	for ctxName := range config.Contexts {
		if err := cm.addContext(config, ctxName); err != nil {
			return fmt.Errorf("failed to add context %s: %w", ctxName, err)
		}
	}

	return nil
}

func (cm *ClientManager) loadInClusterConfig() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to load in-cluster config: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.configs["in-cluster"] = config
	cm.currentContext = "in-cluster"

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create in-cluster clientset: %w", err)
	}
	cm.clientsets["in-cluster"] = clientset

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create in-cluster dynamic client: %w", err)
	}
	cm.dynamicClients["in-cluster"] = dynamicClient

	return nil
}

func (cm *ClientManager) addContext(config *api.Config, ctxName string) error {
	clientConfig := clientcmd.NewNonInteractiveClientConfig(
		*config,
		ctxName,
		&clientcmd.ConfigOverrides{},
		nil,
	)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create rest config for context %s: %w", ctxName, err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.configs[ctxName] = restConfig

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset for context %s: %w", ctxName, err)
	}
	cm.clientsets[ctxName] = clientset

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client for context %s: %w", ctxName, err)
	}
	cm.dynamicClients[ctxName] = dynamicClient

	return nil
}

func (cm *ClientManager) GetClientSet(ctx context.Context, contextName string) (*kubernetes.Clientset, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	targetCtx := contextName
	if targetCtx == "" {
		targetCtx = cm.currentContext
	}

	clientset, ok := cm.clientsets[targetCtx]
	if !ok {
		return nil, fmt.Errorf("context %s not found", targetCtx)
	}

	return clientset, nil
}

func (cm *ClientManager) GetDynamicClient(ctx context.Context, contextName string) (*dynamic.DynamicClient, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	targetCtx := contextName
	if targetCtx == "" {
		targetCtx = cm.currentContext
	}

	dynamicClient, ok := cm.dynamicClients[targetCtx]
	if !ok {
		return nil, fmt.Errorf("context %s not found", targetCtx)
	}

	return dynamicClient, nil
}

func (cm *ClientManager) GetRestConfig(ctx context.Context, contextName string) (*rest.Config, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	targetCtx := contextName
	if targetCtx == "" {
		targetCtx = cm.currentContext
	}

	config, ok := cm.configs[targetCtx]
	if !ok {
		return nil, fmt.Errorf("context %s not found", targetCtx)
	}

	return config, nil
}

func (cm *ClientManager) SetCurrentContext(contextName string) error {
	cm.mu.RLock()
	_, ok := cm.configs[contextName]
	cm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("context %s not found", contextName)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.currentContext = contextName
	return nil
}

func (cm *ClientManager) GetCurrentContext() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.currentContext
}

func (cm *ClientManager) ListContexts() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	contexts := make([]string, 0, len(cm.configs))
	for ctx := range cm.configs {
		contexts = append(contexts, ctx)
	}
	return contexts
}

func (cm *ClientManager) GetClusterInfo(ctx context.Context, contextName string) (*ClusterInfo, error) {
	config, err := cm.GetRestConfig(ctx, contextName)
	if err != nil {
		return nil, err
	}

	return &ClusterInfo{
		Name:     contextName,
		Server:   config.Host,
		Insecure: config.TLSClientConfig.Insecure,
	}, nil
}

func (cm *ClientManager) Reload() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.configs = make(map[string]*rest.Config)
	cm.clientsets = make(map[string]*kubernetes.Clientset)
	cm.dynamicClients = make(map[string]*dynamic.DynamicClient)
	cm.currentContext = ""

	return cm.loadContexts()
}
