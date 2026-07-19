// Package bases embeds generated CRD manifests for offline validation.
package bases

import _ "embed"

// ArgoCDConfigurationCRD is the generated ArgoCDConfiguration CustomResourceDefinition.
//
//go:embed argoproj.io_argocdconfigurations.yaml
var ArgoCDConfigurationCRD []byte
