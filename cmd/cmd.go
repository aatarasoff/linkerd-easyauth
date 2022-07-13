package cmd

import (
	"github.com/fatih/color"
	pkgcmd "github.com/linkerd/linkerd2/pkg/cmd"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultLinkerdNamespace = "linkerd"
)

var (
	stdout = color.Output
	stderr = color.Error

	apiAddr               string
	controlPlaneNamespace string
	kubeconfigPath        string
	kubeContext           string
	impersonate           string
	impersonateGroup      []string
	verbose               bool
)

func NewEasyAuthCmd() *cobra.Command {
	easyAuthCmd := &cobra.Command{
		Use:   "easyauth",
		Short: "easyauth manages the easyauth extension of Linkerd service mesh",
		Long:  `easyauth manages the easyauth extension of Linkerd service mesh.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				log.SetLevel(log.DebugLevel)
			} else {
				log.SetLevel(log.PanicLevel)
			}

			return nil
		},
	}

	easyAuthCmd.AddCommand(newCmdList())
	easyAuthCmd.AddCommand(newCmdAuthCheck())
	easyAuthCmd.AddCommand(newCmdAuthz())

	easyAuthCmd.PersistentFlags().StringVarP(&controlPlaneNamespace, "linkerd-namespace", "L", defaultLinkerdNamespace, "Namespace in which Linkerd is installed")
	easyAuthCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests")
	easyAuthCmd.PersistentFlags().StringVar(&kubeContext, "context", "", "Name of the kubeconfig context to use")
	easyAuthCmd.PersistentFlags().StringVar(&impersonate, "as", "", "Username to impersonate for Kubernetes operations")
	easyAuthCmd.PersistentFlags().StringArrayVar(&impersonateGroup, "as-group", []string{}, "Group to impersonate for Kubernetes operations")
	easyAuthCmd.PersistentFlags().StringVar(&apiAddr, "api-addr", "", "Override kubeconfig and communicate directly with the control plane at host:port (mostly for testing)")
	easyAuthCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Turn on debug logging")

	// resource-aware completion flag configurations
	pkgcmd.ConfigureNamespaceFlagCompletion(
		easyAuthCmd, []string{"linkerd-namespace"},
		kubeconfigPath, impersonate, impersonateGroup, kubeContext)

	pkgcmd.ConfigureKubeContextFlagCompletion(easyAuthCmd, kubeconfigPath)
	return easyAuthCmd
}
