package mapping

import (
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func FuzzFromConfigMaps(f *testing.F) {
	// Seed from case corpus inputs and legacy sample-cms.
	roots := []string{
		filepath.Join("..", "..", "testdata", "cases"),
		filepath.Join("..", "..", "testdata", "sample-cms"),
	}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			base := d.Name()
			if base != "argocd-cm.yaml" && base != "argocd-cmd-params-cm.yaml" && base != "argocd-rbac-cm.yaml" {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			f.Add(b)
			return nil
		})
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		cm := &corev1.ConfigMap{}
		if err := yaml.Unmarshal(raw, cm); err != nil {
			return
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cms := ConfigMaps{CM: cm}
		_, _, _ = FromConfigMaps(cms, DefaultConfigurationName, "argocd")
	})
}
