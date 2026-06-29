package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientManager(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")

	_, err = os.Stat(kubeconfigPath)
	if os.IsNotExist(err) {
		t.Skip("kubeconfig file not found, skipping test")
	}

	manager, err := NewClientManager(kubeconfigPath)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
}

func TestClientManager_WithEmptyKubeconfig(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	_, err = os.Stat(kubeconfigPath)
	if os.IsNotExist(err) {
		t.Skip("kubeconfig file not found, skipping test")
	}

	manager, err := NewClientManager("")
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotEmpty(t, manager.GetCurrentContext())
}

func TestClientManager_ListContexts(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	_, err = os.Stat(kubeconfigPath)
	if os.IsNotExist(err) {
		t.Skip("kubeconfig file not found, skipping test")
	}

	manager, err := NewClientManager(kubeconfigPath)
	require.NoError(t, err)

	contexts := manager.ListContexts()
	assert.NotNil(t, contexts)
	assert.True(t, len(contexts) > 0)
}

func TestClientManager_GetCurrentContext(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	_, err = os.Stat(kubeconfigPath)
	if os.IsNotExist(err) {
		t.Skip("kubeconfig file not found, skipping test")
	}

	manager, err := NewClientManager(kubeconfigPath)
	require.NoError(t, err)

	currentCtx := manager.GetCurrentContext()
	assert.NotEmpty(t, currentCtx)
}
