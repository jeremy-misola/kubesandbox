package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes client connections
type Client struct {
	dynamicClient dynamic.Interface
	clientset     *kubernetes.Clientset
	config        *rest.Config
}

// NewClient creates a new Kubernetes client
func NewClient() (*Client, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kube config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		dynamicClient: dynamicClient,
		clientset:     clientset,
		config:        config,
	}, nil
}

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		slog.Info("Using in-cluster Kubernetes config")
		return config, nil
	}

	// Fall back to kubeconfig file
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}

	slog.Info("Using kubeconfig file", "path", kubeconfig)
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	return config, nil
}

// GetGVR returns the GroupVersionResource for KubeSandboxSession
func GetGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "platform.kubesandbox.com",
		Version:  "v1alpha1",
		Resource: "kubesandboxsessions",
	}
}

// GetClientset returns the Kubernetes clientset
func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

// GetDynamicClient returns the dynamic client
func (c *Client) GetDynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// Scheme returns the runtime scheme
func Scheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	return scheme
}

// Ping checks if the cluster is accessible
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.clientset.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes: %w", err)
	}
	return nil
}
