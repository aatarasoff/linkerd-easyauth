package cmd

import (
	"fmt"
	"github.com/linkerd/linkerd2/cli/table"
	pkgcmd "github.com/linkerd/linkerd2/pkg/cmd"
	"github.com/linkerd/linkerd2/pkg/k8s"
	"github.com/spf13/cobra"
	common "linkerd-easyauth/pkg"
	"os"
)

type authzOptions struct {
	namespace string
}

func newCmdAuthz() *cobra.Command {
	var options authzOptions

	cmd := &cobra.Command{
		Use:   "authz [flags]",
		Short: "List server authorizations for a resource (fast implementation)",
		Long:  "List server authorizations for a resource (fast implementation).",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.namespace == "" {
				options.namespace = pkgcmd.GetDefaultNamespace(kubeconfigPath, kubeContext)
			}

			var resource string
			if len(args) == 1 {
				resource = args[0]
			} else if len(args) == 2 {
				resource = args[0] + "/" + args[1]
			}

			rows := make([]table.Row, 0)

			prefetched, err := FetchK8sResources(cmd.Context(), options.namespace)
			if err != nil {
				return err
			}

			k8sAPI, err := k8s.NewAPI(kubeconfigPath, kubeContext, impersonate, impersonateGroup, 0)

			authzs, err := common.AuthorizationsForResource(cmd.Context(), k8sAPI, prefetched.AuthorizationPolicies, prefetched.HTTPRoutes, prefetched.ServerAuthorizations, prefetched.Servers, options.namespace, resource)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get serverauthorization resources: %s\n", err)
				os.Exit(1)
			}

			for _, authz := range authzs {
				route := "*"
				if authz.Route != "" {
					route = authz.Route
				}
				rows = append(rows, table.Row{route, authz.Server, authz.ServerAuthorization, authz.AuthorizationPolicy})
			}

			cols := []table.Column{
				{Header: "ROUTE", Width: 10, Flexible: true},
				{Header: "SERVER", Width: 10, Flexible: true},
				{Header: "SERVER_AUTHORIZATION", Width: 21, Flexible: true},
				{Header: "AUTHORIZATION_POLICY", Width: 21, Flexible: true},
			}

			table := table.NewTable(cols, rows)
			table.Render(os.Stdout)

			return nil
		},
	}

	cmd.Flags().StringVarP(&options.namespace, "namespace", "n", options.namespace, "The namespace to list pods in")

	pkgcmd.ConfigureNamespaceFlagCompletion(
		cmd, []string{"namespace"},
		kubeconfigPath, impersonate, impersonateGroup, kubeContext)

	return cmd
}
