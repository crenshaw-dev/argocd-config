package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"

	argov1alpha1 "github.com/crenshaw-dev/argocd-config/api/v1alpha1"
	argov1beta1 "github.com/crenshaw-dev/argocd-config/api/v1beta1"
	"github.com/crenshaw-dev/argocd-config/pkg/convert"
	"github.com/crenshaw-dev/argocd-config/pkg/kube"
	"github.com/crenshaw-dev/argocd-config/pkg/mapping"
	"github.com/crenshaw-dev/argocd-config/pkg/validate"
)

// Set at link time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// GlobalOpts are shared across conversion commands.
type GlobalOpts struct {
	Strict     bool
	Report     string
	NoValidate bool
	Verbose    bool
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// ExitCode returns a non-zero exit code when err is an exitError.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exitError); ok {
		return ee.code
	}
	return 1
}

func addGlobalFlags(cmd *cobra.Command, g *GlobalOpts) {
	cmd.PersistentFlags().BoolVar(&g.Strict, "strict", false, "Treat warnings as errors (non-zero exit)")
	cmd.PersistentFlags().StringVar(&g.Report, "report", "text", "Diagnostics report format: text or json")
	cmd.PersistentFlags().BoolVar(&g.NoValidate, "no-validate", false, "Skip post-conversion validation")
	cmd.PersistentFlags().BoolVar(&g.Verbose, "verbose", false, "Verbose logging to stderr")
}

func NewRootCommand() *cobra.Command {
	g := &GlobalOpts{}
	root := &cobra.Command{
		Use:   "argocd-config",
		Short: "Convert Argo CD ConfigMaps <-> ArgoCDConfiguration CRD and between apiVersions",
	}
	addGlobalFlags(root, g)
	root.AddCommand(newFromConfigMapsCommand(g))
	root.AddCommand(newToConfigMapsCommand(g))
	root.AddCommand(newConvertCommand(g))
	root.AddCommand(newValidateCommand(g))
	root.AddCommand(newVersionCommand())
	return root
}

func newFromConfigMapsCommand(g *GlobalOpts) *cobra.Command {
	var (
		cmFile      string
		cmdFile     string
		rbacFile    string
		fromCluster bool
		kubeconfig  string
		kubeContext string
		name        string
		namespace   string
		outFile     string
		permissive  bool
	)
	cmd := &cobra.Command{
		Use:   "from-configmaps",
		Short: "Convert argocd-cm / argocd-cmd-params-cm / argocd-rbac-cm into an ArgoCDConfiguration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cms, err := loadConfigMapsInput(cmd.Context(), fromCluster, kubeconfig, kubeContext, namespace, cmFile, cmdFile, rbacFile)
			if err != nil {
				return err
			}
			if g.Verbose && fromCluster {
				fmt.Fprintf(os.Stderr, "loaded ConfigMaps from cluster namespace %q\n", namespace)
			}

			cfg, diag, err := mapping.FromConfigMaps(cms, name, namespace)
			if err != nil {
				return finish(nil, err, g)
			}
			if g.Verbose {
				fmt.Fprintf(os.Stderr, "converted ConfigMaps to ArgoCDConfiguration %q in namespace %q\n", cfg.Name, cfg.Namespace)
			}

			if !g.NoValidate {
				diag.Merge(validate.Validate(cfg))
			}

			if !permissive {
				selfCheckRoundTrip(cms, cfg, namespace, diag)
			}

			if err := writeYAML(outFile, cfg); err != nil {
				return err
			}
			return finish(diag, nil, g)
		},
	}
	cmd.Flags().StringVar(&cmFile, "cm", "", "Path to argocd-cm YAML")
	cmd.Flags().StringVar(&cmdFile, "cmd-params", "", "Path to argocd-cmd-params-cm YAML")
	cmd.Flags().StringVar(&rbacFile, "rbac", "", "Path to argocd-rbac-cm YAML")
	cmd.Flags().BoolVar(&fromCluster, "from-cluster", false, "Load standard-named ConfigMaps from the cluster instead of disk")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (default: KUBECONFIG / ~/.kube/config)")
	cmd.Flags().StringVar(&kubeContext, "context", "", "Kubeconfig context to use with --from-cluster")
	cmd.Flags().StringVar(&name, "name", mapping.DefaultConfigurationName, "Name of the ArgoCDConfiguration")
	cmd.Flags().StringVar(&namespace, "namespace", "argocd", "Namespace of the ArgoCDConfiguration (and ConfigMaps when --from-cluster)")
	cmd.Flags().StringVarP(&outFile, "output", "o", "-", "Output file (- for stdout)")
	cmd.Flags().BoolVar(&permissive, "permissive", false, "Skip round-trip self-check (escape hatch; inspect output manually)")
	return cmd
}

// loadConfigMapsInput loads ConfigMaps from the cluster or from disk.
// --from-cluster is mutually exclusive with --cm / --cmd-params / --rbac.
func loadConfigMapsInput(ctx context.Context, fromCluster bool, kubeconfig, kubeContext, namespace, cmFile, cmdFile, rbacFile string) (mapping.ConfigMaps, error) {
	hasDisk := cmFile != "" || cmdFile != "" || rbacFile != ""
	if fromCluster && hasDisk {
		return mapping.ConfigMaps{}, fmt.Errorf("--from-cluster cannot be combined with --cm, --cmd-params, or --rbac")
	}
	if !fromCluster && !hasDisk {
		return mapping.ConfigMaps{}, fmt.Errorf("no ConfigMaps provided (use --from-cluster, or --cm/--cmd-params/--rbac)")
	}
	if fromCluster {
		cs, err := kube.NewClientset(kube.ClientOptions{Kubeconfig: kubeconfig, Context: kubeContext})
		if err != nil {
			return mapping.ConfigMaps{}, err
		}
		return kube.LoadConfigMaps(ctx, kube.ConfigMapsInNamespace(cs, namespace), namespace)
	}
	return loadConfigMaps(cmFile, cmdFile, rbacFile)
}

func newToConfigMapsCommand(g *GlobalOpts) *cobra.Command {
	var (
		inFile     string
		outDir     string
		namespace  string
		sourceCM   string
		sourceCmd  string
		sourceRBAC string
	)
	cmd := &cobra.Command{
		Use:   "to-configmaps",
		Short: "Convert an ArgoCDConfiguration into ConfigMap YAML files",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := readConfiguration(inFile)
			if err != nil {
				return err
			}
			if namespace == "" {
				namespace = cfg.Namespace
			}
			if namespace == "" {
				namespace = "argocd"
			}

			if !g.NoValidate {
				diag := validate.Validate(cfg)
				if diag.HasErrors() || (g.Strict && diag.HasWarnings()) {
					if err := writeDiagnostics(os.Stderr, diag, g.Report); err != nil {
						return err
					}
					return finish(diag, nil, g)
				}
			}

			source := mapping.ConfigMaps{}
			if sourceCM != "" || sourceCmd != "" || sourceRBAC != "" {
				source, err = loadConfigMaps(sourceCM, sourceCmd, sourceRBAC)
				if err != nil {
					return err
				}
				if g.Verbose {
					fmt.Fprintf(os.Stderr, "loaded source ConfigMaps for metadata preservation\n")
				}
			}

			cms, diag, err := mapping.ToConfigMapsWithSource(cfg, namespace, source)
			if err != nil {
				return finish(diag, err, g)
			}

			if outDir == "" || outDir == "-" {
				if err := writeMultiYAML(os.Stdout, cms.CM, cms.CmdParams, cms.RBAC); err != nil {
					return finish(diag, err, g)
				}
				return finish(diag, nil, g)
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			if err := writeYAML(filepath.Join(outDir, "argocd-cm.yaml"), cms.CM); err != nil {
				return finish(diag, err, g)
			}
			if err := writeYAML(filepath.Join(outDir, "argocd-cmd-params-cm.yaml"), cms.CmdParams); err != nil {
				return finish(diag, err, g)
			}
			if err := writeYAML(filepath.Join(outDir, "argocd-rbac-cm.yaml"), cms.RBAC); err != nil {
				return finish(diag, err, g)
			}
			return finish(diag, nil, g)
		},
	}
	cmd.Flags().StringVarP(&inFile, "file", "f", "-", "Input ArgoCDConfiguration YAML (- for stdin)")
	cmd.Flags().StringVarP(&outDir, "output", "o", "-", "Output directory (or - for stdout multi-doc)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace for emitted ConfigMaps (default: from CR metadata)")
	cmd.Flags().StringVar(&sourceCM, "source-cm", "", "Original argocd-cm YAML for metadata preservation")
	cmd.Flags().StringVar(&sourceCmd, "source-cmd-params", "", "Original argocd-cmd-params-cm YAML for metadata preservation")
	cmd.Flags().StringVar(&sourceRBAC, "source-rbac", "", "Original argocd-rbac-cm YAML for metadata preservation")
	return cmd
}

func newConvertCommand(g *GlobalOpts) *cobra.Command {
	var (
		inFile    string
		outFile   string
		toVersion string
	)
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert an ArgoCDConfiguration between apiVersions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := readConfiguration(inFile)
			if err != nil {
				return err
			}

			diag := &mapping.Diagnostics{}
			if !g.NoValidate {
				diag = validate.Validate(cfg)
				if diag.HasErrors() {
					return finish(diag, nil, g)
				}
			}

			converted, err := convert.ToVersion(cfg, toVersion)
			if err != nil {
				return err
			}

			if !g.NoValidate {
				switch out := converted.(type) {
				case *argov1alpha1.ArgoCDConfiguration:
					diag.Merge(validate.Validate(out))
				case *argov1beta1.ArgoCDConfiguration:
					if out.Name != mapping.DefaultConfigurationName {
						diag.Error("", "metadata.name",
							fmt.Sprintf("name must be %q, got %q", mapping.DefaultConfigurationName, out.Name))
					}
					validateHTTPURLDiag(diag, "spec.url", out.Spec.URL)
				default:
					return fmt.Errorf("expected ArgoCDConfiguration, got %T", converted)
				}
			}

			if err := writeYAML(outFile, converted); err != nil {
				return err
			}
			return finish(diag, nil, g)
		},
	}
	cmd.Flags().StringVarP(&inFile, "file", "f", "-", "Input ArgoCDConfiguration YAML")
	cmd.Flags().StringVarP(&outFile, "output", "o", "-", "Output file")
	cmd.Flags().StringVar(&toVersion, "to-version", argov1alpha1.GroupVersion.String(), "Target apiVersion")
	return cmd
}

func newValidateCommand(g *GlobalOpts) *cobra.Command {
	var inFile string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate an ArgoCDConfiguration resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := readConfiguration(inFile)
			if err != nil {
				return err
			}
			diag := validate.Validate(cfg)
			if g.Verbose && diag.Len() == 0 {
				fmt.Fprintf(os.Stderr, "validation passed for %q\n", cfg.Name)
			}
			return finish(diag, nil, g)
		},
	}
	cmd.Flags().StringVarP(&inFile, "file", "f", "-", "Input ArgoCDConfiguration YAML (- for stdin)")
	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "argocd-config %s (commit: %s, built: %s)\n", Version, Commit, Date)
		},
	}
}

func finish(diag *mapping.Diagnostics, err error, g *GlobalOpts) error {
	if err != nil {
		if diag != nil && diag.Len() > 0 {
			_ = writeDiagnostics(os.Stderr, diag, g.Report)
		}
		return err
	}
	if diag != nil && diag.Len() > 0 {
		if err := writeDiagnostics(os.Stderr, diag, g.Report); err != nil {
			return err
		}
	}
	if diag != nil && diag.HasErrors() {
		return &exitError{code: 1, msg: "completed with errors"}
	}
	if g.Strict && diag != nil && diag.HasWarnings() {
		return &exitError{code: 1, msg: "completed with warnings (--strict)"}
	}
	return nil
}

func writeDiagnostics(w io.Writer, diag *mapping.Diagnostics, report string) error {
	if diag == nil || diag.Len() == 0 {
		return nil
	}
	switch strings.ToLower(report) {
	case "json":
		return diag.WriteJSON(w)
	case "text", "":
		return diag.WriteHuman(w)
	default:
		return fmt.Errorf("unknown report format %q (use text or json)", report)
	}
}

func loadConfigMaps(cmFile, cmdFile, rbacFile string) (mapping.ConfigMaps, error) {
	cms := mapping.ConfigMaps{}
	var err error
	if cmFile != "" {
		cms.CM, err = readCMFile(cmFile)
		if err != nil {
			return cms, err
		}
	}
	if cmdFile != "" {
		cms.CmdParams, err = readCMFile(cmdFile)
		if err != nil {
			return cms, err
		}
	}
	if rbacFile != "" {
		cms.RBAC, err = readCMFile(rbacFile)
		if err != nil {
			return cms, err
		}
	}
	return cms, nil
}

func selfCheckRoundTrip(source mapping.ConfigMaps, cfg *argov1alpha1.ArgoCDConfiguration, namespace string, diag *mapping.Diagnostics) {
	round, roundDiag, err := mapping.ToConfigMapsWithSource(cfg, namespace, source)
	if roundDiag != nil {
		diag.Merge(roundDiag)
	}
	if err != nil {
		diag.Error(mapping.DirCRToCM, "self-check", fmt.Sprintf("round-trip failed: %v", err))
		return
	}
	diffConfigMapData(diag, mapping.ArgoCDCMName, source.CM, round.CM)
	diffConfigMapData(diag, mapping.ArgoCDCmdParamsCMName, source.CmdParams, round.CmdParams)
	diffConfigMapData(diag, mapping.ArgoCDRBACCMName, source.RBAC, round.RBAC)
}

func diffConfigMapData(diag *mapping.Diagnostics, cmName string, orig, round *corev1.ConfigMap) {
	if orig == nil && round == nil {
		return
	}
	if orig == nil {
		if round != nil && len(round.Data) > 0 {
			diag.Warn(mapping.DirCRToCM, cmName, "round-trip produced keys but source ConfigMap was absent")
		}
		return
	}
	if round == nil {
		if len(orig.Data) > 0 {
			diag.Warn(mapping.DirCRToCM, cmName, "round-trip lost ConfigMap that was present in source")
		}
		return
	}
	d := mapping.DiffConfigMapDataNormalized(orig.Data, round.Data)
	for _, k := range d.Missing {
		diag.Warn(mapping.DirCRToCM, k, fmt.Sprintf("key missing after round-trip in %s", cmName))
	}
	for _, k := range d.Extra {
		diag.Warn(mapping.DirCRToCM, k, fmt.Sprintf("unexpected key after round-trip in %s", cmName))
	}
	for _, k := range d.Changed {
		diag.Warn(mapping.DirCRToCM, k, fmt.Sprintf("value changed after round-trip in %s", cmName))
	}
}

func readCMFile(path string) (*corev1.ConfigMap, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{}
	if err := yaml.Unmarshal(b, cm); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	return cm, nil
}

func validateHTTPURLDiag(diag *mapping.Diagnostics, field, raw string) {
	if raw == "" {
		return
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		diag.Error("", field, fmt.Sprintf("URL must start with http:// or https://, got %q", raw))
	}
}

func readConfiguration(path string) (*argov1alpha1.ArgoCDConfiguration, error) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	scheme := convert.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	obj, _, err := codecs.UniversalDeserializer().Decode(b, nil, nil)
	if err != nil {
		cfg := &argov1alpha1.ArgoCDConfiguration{}
		if err2 := yaml.Unmarshal(b, cfg); err2 != nil {
			return nil, fmt.Errorf("decode: %v (yaml fallback: %w)", err, err2)
		}
		if cfg.APIVersion == "" {
			cfg.APIVersion = argov1alpha1.GroupVersion.String()
		}
		if cfg.Kind == "" {
			cfg.Kind = "ArgoCDConfiguration"
		}
		return cfg, nil
	}
	switch o := obj.(type) {
	case *argov1alpha1.ArgoCDConfiguration:
		return o, nil
	case *argov1beta1.ArgoCDConfiguration:
		hub := &argov1alpha1.ArgoCDConfiguration{}
		if err := o.ConvertTo(hub); err != nil {
			return nil, fmt.Errorf("convert v1beta1 to hub: %w", err)
		}
		return hub, nil
	default:
		return nil, fmt.Errorf("expected ArgoCDConfiguration, got %T", obj)
	}
}

func writeYAML(path string, obj any) error {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	if path == "-" {
		_, err = os.Stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeMultiYAML(w io.Writer, objs ...runtime.Object) error {
	first := true
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		b, err := yaml.Marshal(obj)
		if err != nil {
			return err
		}
		if !first {
			if _, err := io.WriteString(w, "---\n"); err != nil {
				return err
			}
		}
		first = false
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}
