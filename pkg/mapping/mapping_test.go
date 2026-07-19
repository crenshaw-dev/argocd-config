package mapping_test

import (
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
)

func TestRoundTripSample(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "sample-cms")
	cms := mapping.ConfigMaps{
		CM:        mustReadCM(t, filepath.Join(dir, "argocd-cm.yaml")),
		CmdParams: mustReadCM(t, filepath.Join(dir, "argocd-cmd-params-cm.yaml")),
		RBAC:      mustReadCM(t, filepath.Join(dir, "argocd-rbac-cm.yaml")),
	}

	cfg, diag, err := mapping.FromConfigMaps(cms, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("FromConfigMaps: %v", err)
	}
	if diag == nil {
		t.Fatal("expected non-nil diagnostics")
	}

	// Spot-check hard cases
	if cfg.Spec.Server == nil || len(cfg.Spec.Server.URLs) != 1 || cfg.Spec.Server.URLs[0] != "https://argocd.example.com" {
		t.Fatalf("url: got %#v", cfg.Spec.Server)
	}
	if cfg.Spec.Server.OIDC == nil || cfg.Spec.Server.OIDC.ClientSecretRef == nil ||
		cfg.Spec.Server.OIDC.ClientSecretRef.Name != "argocd-secret" ||
		cfg.Spec.Server.OIDC.ClientSecretRef.Key != "oidc.clientSecret" {
		t.Fatalf("oidc clientSecretRef not mapped from $string: %#v", cfg.Spec.Server.OIDC)
	}
	if cfg.Spec.Server.Dex == nil || len(cfg.Spec.Server.Dex.Connectors) != 1 || cfg.Spec.Server.Dex.Connectors[0].Type != "github" {
		t.Fatalf("dex connectors: %#v", cfg.Spec.Server.Dex)
	}
	if cfg.Spec.Server.RBAC == nil || cfg.Spec.Server.RBAC.PolicyCSV == "" || cfg.Spec.Server.RBAC.Default != "role:readonly" {
		t.Fatalf("rbac: %#v", cfg.Spec.Server.RBAC)
	}
	if len(cfg.Spec.Server.RBAC.PolicyOverlays) != 1 || cfg.Spec.Server.RBAC.PolicyOverlays[0].Name != "extra" {
		t.Fatalf("rbac overlays: %#v", cfg.Spec.Server.RBAC.PolicyOverlays)
	}
	if cfg.Spec.Controller == nil || cfg.Spec.Controller.Resource == nil || len(cfg.Spec.Controller.Resource.Customizations) == 0 {
		t.Fatalf("resource customizations missing: %#v", cfg.Spec.Controller)
	}
	var foundActions bool
	for _, c := range cfg.Spec.Controller.Resource.Customizations {
		if c.Group == "apps" && c.Kind == "Deployment" && c.Actions != nil {
			foundActions = true
			if c.Actions.DiscoveryLua == "" {
				t.Fatalf("actions discoveryLua empty: %#v", c.Actions)
			}
			if len(c.Actions.Definitions) != 1 || c.Actions.Definitions[0].Name != "restart" || c.Actions.Definitions[0].ActionLua == "" {
				t.Fatalf("actions definitions: %#v", c.Actions.Definitions)
			}
		}
	}
	if !foundActions {
		t.Fatalf("apps/Deployment actions customization not mapped")
	}
	if len(cfg.Spec.ApplicationNamespaceGlobs) != 2 {
		t.Fatalf("application.namespaces: %#v", cfg.Spec.ApplicationNamespaceGlobs)
	}
	if cfg.Spec.RepoServer == nil || cfg.Spec.RepoServer.Address != "argocd-repo-server:8081" {
		t.Fatalf("repo.server address: %#v", cfg.Spec.RepoServer)
	}
	if cfg.Spec.RepoServer == nil || cfg.Spec.RepoServer.Plugin == nil || len(cfg.Spec.RepoServer.Plugin.TarExclusionGlobs) != 2 {
		t.Fatalf("plugin tar exclusions (; separator): %#v", cfg.Spec.RepoServer)
	}
	if cfg.Spec.Controller == nil || cfg.Spec.Controller.Processors == nil || cfg.Spec.Controller.Processors.Status == nil || *cfg.Spec.Controller.Processors.Status != 20 {
		t.Fatalf("controller status processors: %#v", cfg.Spec.Controller)
	}
	if cfg.Spec.Controller.Sync == nil || cfg.Spec.Controller.Sync.Wave == nil || cfg.Spec.Controller.Sync.Wave.Delay == nil || cfg.Spec.Controller.Sync.Wave.Delay.Duration.Seconds() != 2 {
		t.Fatalf("sync.wave.delay: %#v", cfg.Spec.Controller.Sync)
	}
	if cfg.Spec.Controller.SourceHydrator == nil || cfg.Spec.Controller.SourceHydrator.Enabled == nil || *cfg.Spec.Controller.SourceHydrator.Enabled {
		t.Fatalf("hydrator.enabled: %#v", cfg.Spec.Controller.SourceHydrator)
	}
	if cfg.Spec.Cluster == nil || cfg.Spec.Cluster.InClusterEnabled == nil || !*cfg.Spec.Cluster.InClusterEnabled {
		t.Fatalf("cluster.inClusterEnabled: %#v", cfg.Spec.Cluster)
	}
	if cfg.Spec.Server.Users == nil || cfg.Spec.Server.Users.PasswordRegex != "^.{8,32}$" {
		t.Fatalf("passwordRegex: %#v", cfg.Spec.Server.Users)
	}
	if cfg.Spec.Controller.Sync == nil || cfg.Spec.Controller.Sync.Impersonation == nil || cfg.Spec.Controller.Sync.Impersonation.Mode != "required" {
		t.Fatalf("impersonation.mode: %#v", cfg.Spec.Controller.Sync)
	}
	if cfg.Spec.InstallationID != "sample-install" {
		t.Fatalf("installationID: %#v", cfg.Spec.InstallationID)
	}
	if cfg.Spec.Controller.Reconciliation == nil || cfg.Spec.Controller.Reconciliation.Jitter == nil || cfg.Spec.Controller.Reconciliation.Jitter.Duration.Seconds() != 60 {
		t.Fatalf("reconciliation jitter: %#v", cfg.Spec.Controller.Reconciliation)
	}
	if cfg.Spec.RepoServer.Jsonnet == nil || cfg.Spec.RepoServer.Jsonnet.Enabled == nil || !*cfg.Spec.RepoServer.Jsonnet.Enabled {
		t.Fatalf("jsonnet.enable: %#v", cfg.Spec.RepoServer.Jsonnet)
	}
	if cfg.Spec.Server.BaseHref != "/argo-cd" || cfg.Spec.Server.Compression != "gzip" {
		t.Fatalf("server cmd-params: %#v", cfg.Spec.Server)
	}
	if cfg.Spec.ApplicationSet == nil || cfg.Spec.ApplicationSet.ProgressiveSyncs == nil || cfg.Spec.ApplicationSet.ProgressiveSyncs.Enabled == nil || !*cfg.Spec.ApplicationSet.ProgressiveSyncs.Enabled {
		t.Fatalf("appsset progressive syncs: %#v", cfg.Spec.ApplicationSet)
	}
	if cfg.Spec.ApplicationSet.K8sClient == nil || cfg.Spec.ApplicationSet.K8sClient.QPS != "50" ||
		cfg.Spec.ApplicationSet.K8sClient.Burst == nil || *cfg.Spec.ApplicationSet.K8sClient.Burst != 100 {
		t.Fatalf("appsset k8s client: %#v", cfg.Spec.ApplicationSet.K8sClient)
	}
	if cfg.Spec.ApplicationSet.RepoServerClient == nil ||
		cfg.Spec.ApplicationSet.RepoServerClient.CACertPath != "/etc/argocd/appsset/ca.crt" ||
		cfg.Spec.ApplicationSet.RepoServerClient.ClientCertPath != "/etc/argocd/appsset/tls.crt" ||
		cfg.Spec.ApplicationSet.RepoServerClient.ClientCertKeyPath != "/etc/argocd/appsset/tls.key" {
		t.Fatalf("appsset repo-server certs: %#v", cfg.Spec.ApplicationSet.RepoServerClient)
	}
	if cfg.Spec.DexServer == nil || cfg.Spec.DexServer.TLSEnabled == nil || *cfg.Spec.DexServer.TLSEnabled {
		t.Fatalf("dexserver: %#v", cfg.Spec.DexServer)
	}
	if cfg.Spec.Notifications == nil || cfg.Spec.Notifications.ProcessorsCount == nil || *cfg.Spec.Notifications.ProcessorsCount != 5 {
		t.Fatalf("notifications: %#v", cfg.Spec.Notifications)
	}
	if cfg.Spec.Notifications.RepoServerClient == nil ||
		cfg.Spec.Notifications.RepoServerClient.CACertPath != "/etc/argocd/notifications/ca.crt" ||
		cfg.Spec.Notifications.RepoServerClient.ClientCertPath != "/etc/argocd/notifications/tls.crt" ||
		cfg.Spec.Notifications.RepoServerClient.ClientCertKeyPath != "/etc/argocd/notifications/tls.key" {
		t.Fatalf("notifications repo-server certs: %#v", cfg.Spec.Notifications.RepoServerClient)
	}

	out, outDiag, err := mapping.ToConfigMaps(cfg, "argocd")
	if err != nil {
		t.Fatalf("ToConfigMaps: %v", err)
	}
	if outDiag == nil {
		t.Fatal("expected non-nil diagnostics from ToConfigMaps")
	}

	// Round-trip again and compare key semantic fields
	cfg2, _, err := mapping.FromConfigMaps(out, mapping.DefaultConfigurationName, "argocd")
	if err != nil {
		t.Fatalf("second FromConfigMaps: %v", err)
	}
	if len(cfg2.Spec.Server.URLs) != len(cfg.Spec.Server.URLs) || (len(cfg.Spec.Server.URLs) > 0 && cfg2.Spec.Server.URLs[0] != cfg.Spec.Server.URLs[0]) {
		t.Fatalf("url drift: %#v vs %#v", cfg2.Spec.Server.URLs, cfg.Spec.Server.URLs)
	}
	if cfg2.Spec.Server.OIDC.ClientSecretRef == nil || cfg2.Spec.Server.OIDC.ClientSecretRef.Key != "oidc.clientSecret" {
		t.Fatalf("secret ref lost on round-trip: %#v", cfg2.Spec.Server.OIDC.ClientSecretRef)
	}
	if cfg2.Spec.Server.RBAC.PolicyCSV != cfg.Spec.Server.RBAC.PolicyCSV {
		t.Fatalf("rbac policy.csv drift")
	}
	if cfg2.Spec.Server.Dex.Connectors[0].Type != "github" {
		t.Fatalf("dex connector lost")
	}
}

func mustReadCM(t *testing.T, path string) *corev1.ConfigMap {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	cm := &corev1.ConfigMap{}
	if err := yaml.Unmarshal(b, cm); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	return cm
}
