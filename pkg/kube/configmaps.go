// Package kube loads Argo CD ConfigMaps from a live Kubernetes cluster.
package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

// ConfigMapGetter is the subset of the CoreV1 ConfigMaps API used to load
// Argo CD configuration ConfigMaps. Satisfied by client-go and its fake.
type ConfigMapGetter interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error)
}

// LoadConfigMaps fetches the standard-named Argo CD ConfigMaps from namespace.
// Missing ConfigMaps are omitted (same as omitting --cm/--cmd-params/--rbac on disk).
// At least one ConfigMap must exist or an error is returned.
func LoadConfigMaps(ctx context.Context, cms ConfigMapGetter, namespace string) (mapping.ConfigMaps, error) {
	out := mapping.ConfigMaps{}
	var err error

	out.CM, err = getOptional(ctx, cms, mapping.ArgoCDCMName)
	if err != nil {
		return out, err
	}
	out.CmdParams, err = getOptional(ctx, cms, mapping.ArgoCDCmdParamsCMName)
	if err != nil {
		return out, err
	}
	out.RBAC, err = getOptional(ctx, cms, mapping.ArgoCDRBACCMName)
	if err != nil {
		return out, err
	}

	if out.CM == nil && out.CmdParams == nil && out.RBAC == nil {
		return out, fmt.Errorf("no Argo CD ConfigMaps found in namespace %q (looked for %s, %s, %s)",
			namespace, mapping.ArgoCDCMName, mapping.ArgoCDCmdParamsCMName, mapping.ArgoCDRBACCMName)
	}
	return out, nil
}

func getOptional(ctx context.Context, cms ConfigMapGetter, name string) (*corev1.ConfigMap, error) {
	cm, err := cms.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ConfigMap %q: %w", name, err)
	}
	return cm, nil
}

// ClientOptions configures how RestConfig builds a kubeconfig-backed client.
type ClientOptions struct {
	Kubeconfig string // path; empty uses the standard loading rules
	Context    string // kubeconfig context override; empty keeps current-context
}

// RestConfig builds a rest.Config from kubeconfig loading rules.
func RestConfig(opts ClientOptions) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		overrides.CurrentContext = opts.Context
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return cfg, nil
}

// NewClientset builds a kubernetes clientset from ClientOptions.
func NewClientset(opts ClientOptions) (kubernetes.Interface, error) {
	cfg, err := RestConfig(opts)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return cs, nil
}

// ConfigMapsInNamespace returns a ConfigMapGetter scoped to namespace.
func ConfigMapsInNamespace(cs kubernetes.Interface, namespace string) ConfigMapGetter {
	return cs.CoreV1().ConfigMaps(namespace)
}
