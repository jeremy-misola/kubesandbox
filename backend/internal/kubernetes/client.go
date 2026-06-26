// Package kubernetes wraps the client-go dynamic client used to manage
// KubeSandboxSession claims. No generated CRD types are required (G1 decision).
package kubernetes

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewDynamicClient builds a dynamic client. It prefers in-cluster config (the
// production path, using the backend ServiceAccount) and falls back to the
// caller's kubeconfig for local development.
func NewDynamicClient() (dynamic.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		loader := clientcmd.NewDefaultClientConfigLoadingRules()
		fallback, ferr := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loader, &clientcmd.ConfigOverrides{}).ClientConfig()
		if ferr != nil {
			return nil, fmt.Errorf("no in-cluster config (%v) and no kubeconfig (%w)", err, ferr)
		}
		cfg = fallback
	}
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return client, nil
}
