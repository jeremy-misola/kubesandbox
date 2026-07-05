// Package kubernetes wraps the client-go dynamic client used to manage
// KubeSandboxSession claims. No generated CRD types are required.
package kubernetes

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// restConfig prefers in-cluster config (the production path, using the
// backend ServiceAccount) and falls back to the caller's kubeconfig for local
// development.
func restConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	fallback, ferr := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader, &clientcmd.ConfigOverrides{}).ClientConfig()
	if ferr != nil {
		return nil, fmt.Errorf("no in-cluster config (%v) and no kubeconfig (%w)", err, ferr)
	}
	return fallback, nil
}

// NewDynamicClient builds the dynamic client used for claims and markers.
func NewDynamicClient() (dynamic.Interface, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return client, nil
}

// NewClientset builds the typed clientset used for leader-election Leases.
func NewClientset() (clientgo.Interface, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	client, err := clientgo.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return client, nil
}
