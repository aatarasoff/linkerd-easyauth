package cmd

import (
	"context"
	"fmt"
	"github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta1"
	pkgcmd "github.com/linkerd/linkerd2/pkg/cmd"
	"github.com/linkerd/linkerd2/pkg/healthcheck"
	"github.com/linkerd/linkerd2/pkg/k8s"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"strings"
	"time"
)

const (
	linkerdEasyAuthExtensionCheck healthcheck.CategoryID = "linkerd-easyauth"
)

type authCheckOptions struct {
	namespace     string
	allNamespaces bool
}

func newCmdAuthCheck() *cobra.Command {
	var options authCheckOptions

	cmd := &cobra.Command{
		Use:   "authcheck [flags]",
		Short: "Lists which pods use easyauth configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.namespace == "" {
				options.namespace = pkgcmd.GetDefaultNamespace(kubeconfigPath, kubeContext)
			}
			if options.allNamespaces {
				options.namespace = v1.NamespaceAll
			}

			resources, err := FetchK8sResources(cmd.Context(), options.namespace)
			if err != nil {
				return err
			}

			hc := healthcheck.NewHealthChecker([]healthcheck.CategoryID{}, &healthcheck.Options{
				ControlPlaneNamespace: controlPlaneNamespace,
				KubeConfig:            kubeconfigPath,
				KubeContext:           kubeContext,
				Impersonate:           impersonate,
				ImpersonateGroup:      impersonateGroup,
				APIAddr:               apiAddr,
				RetryDeadline:         time.Now().Add(600),
				DataPlaneNamespace:    options.namespace,
			})

			hc.AppendCategories(easyAuthCategory(resources))

			success, warning := healthcheck.RunChecks(stdout, stderr, hc, healthcheck.TableOutput)
			healthcheck.PrintChecksResult(stdout, healthcheck.TableOutput, success, warning)

			if !success {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&options.namespace, "namespace", "n", options.namespace, "The namespace to list pods in")
	cmd.Flags().BoolVarP(&options.allNamespaces, "all-namespaces", "A", options.allNamespaces, "If present, list pods across all namespaces")

	pkgcmd.ConfigureNamespaceFlagCompletion(
		cmd, []string{"namespace"},
		kubeconfigPath, impersonate, impersonateGroup, kubeContext)

	return cmd
}

func easyAuthCategory(resources *K8sResources) *healthcheck.Category {
	checkers := []healthcheck.Checker{}

	checkers = append(checkers,
		*healthcheck.NewChecker("linkerd-easyauth no Server without authorization policies").
			Warning().
			WithCheck(func(ctx context.Context) error {
				serversWOServerAuthorizations := []string{}

				for _, server := range resources.Servers {
					founded := false

					for _, serverAuthorization := range resources.ServerAuthorizations {
						selector, err := metav1.LabelSelectorAsSelector(serverAuthorization.Spec.Server.Selector)
						if err != nil {
							return err
						}

						if selector.Matches(labels.Set(server.GetLabels())) {
							founded = true
						}

						if serverAuthorization.Spec.Server.Name == server.GetName() {
							founded = true
						}
					}

					for _, policy := range resources.AuthorizationPolicies {
						// namespaced policies applies on each server
						if policy.Spec.TargetRef.Kind == "Namespace" {
							founded = true
						}

						if string(policy.Spec.TargetRef.Name) == server.GetName() {
							founded = true
						}
					}

					if !founded {
						serversWOServerAuthorizations = append(serversWOServerAuthorizations, fmt.Sprintf("Server %s has no authorization policies", server.GetName()))
					}
				}

				if len(serversWOServerAuthorizations) == 0 {
					return nil
				}
				return fmt.Errorf("Some servers have no authorization policies:\n\t%s", strings.Join(serversWOServerAuthorizations, "\n\t"))
			}))

	checkers = append(checkers,
		*healthcheck.NewChecker("linkerd-easyauth no authorization policies without Server").
			Warning().
			WithCheck(func(ctx context.Context) error {
				serverAuthorizationsWOServer := []string{}

				for _, serverAuthorization := range resources.ServerAuthorizations {
					founded := false
					for _, server := range resources.Servers {
						selector, err := metav1.LabelSelectorAsSelector(serverAuthorization.Spec.Server.Selector)
						if err != nil {
							return err
						}

						if selector.Matches(labels.Set(server.GetLabels())) {
							founded = true
						}

						if serverAuthorization.Spec.Server.Name == server.GetName() {
							founded = true
						}
					}

					if !founded {
						serverAuthorizationsWOServer = append(serverAuthorizationsWOServer, fmt.Sprintf("ServerAuthorizarions %s does not apply to any Server", serverAuthorization.GetName()))
					}
				}

				for _, policy := range resources.AuthorizationPolicies {
					founded := false

					if policy.Spec.TargetRef.Kind == "Namespace" {
						// at least one Server should exist
						founded = len(resources.Servers) > 0
					} else {
						for _, server := range resources.Servers {
							if policy.Spec.TargetRef.Kind == "Server" && (string(policy.Spec.TargetRef.Name) == server.GetName()) {
								founded = true
							}
						}
					}

					if !founded {
						serverAuthorizationsWOServer = append(serverAuthorizationsWOServer, fmt.Sprintf("Authorization Policy %s does not apply to any Server", policy.GetName()))
					}
				}

				if len(serverAuthorizationsWOServer) == 0 {
					return nil
				}
				return fmt.Errorf("Obsolete ServerAuthorizations:\n\t%s", strings.Join(serverAuthorizationsWOServer, "\n\t"))
			}))

	checkers = append(checkers,
		*healthcheck.NewChecker("linkerd-easyauth no ports without Server").
			Warning().
			WithCheck(func(ctx context.Context) error {
				portsWOServers := []string{}

				for _, pod := range resources.Pods.Items {
					if k8s.IsMeshed(&pod, controlPlaneNamespace) {
						foundedPorts, err := checkPodsPortsForServer(resources, pod)
						if err != nil {
							return err
						}
						portsWOServers = append(portsWOServers, foundedPorts...)
					}
				}

				if len(portsWOServers) == 0 {
					return nil
				}
				return fmt.Errorf("Some pods have ports that are not covered by Server:\n\t%s", strings.Join(portsWOServers, "\n\t"))
			}))

	return healthcheck.NewCategory(linkerdEasyAuthExtensionCheck, checkers, true)
}

func checkPodsPortsForServer(resources *K8sResources, pod v1.Pod) ([]string, error) {
	portsWOServers := []string{}
	foundedPorts := map[int32]bool{}
	for _, service := range resources.Services.Items {
		if len(service.Spec.Selector) > 0 && labels.SelectorFromSet(service.Spec.Selector).Matches(labels.Set(pod.Labels)) {
			for _, svcPort := range service.Spec.Ports {
				for _, container := range pod.Spec.Containers {
					for _, podPort := range container.Ports {
						var matchedPort v1.ContainerPort
						if svcPort.TargetPort.IntValue() > 0 {
							if int(podPort.ContainerPort) == svcPort.TargetPort.IntValue() {
								matchedPort = podPort
							}
						} else {
							if podPort.Name == svcPort.TargetPort.String() {
								matchedPort = podPort
							}
						}

						if matchedPort.ContainerPort > 0 {
							founded, err := findServerForPort(resources.Servers, pod, matchedPort)
							if err != nil {
								return nil, err
							}

							if !founded {
								if !foundedPorts[matchedPort.ContainerPort] {
									foundedPorts[matchedPort.ContainerPort] = true
									portsWOServers = append(portsWOServers, fmt.Sprintf("%s -> %s:%d has no Server", pod.Name, container.Name, matchedPort.ContainerPort))
								}
							}
						}
					}
				}
			}
		}
	}
	return portsWOServers, nil
}

func findServerForPort(servers []*v1beta1.Server, pod v1.Pod, matchedPort v1.ContainerPort) (bool, error) {
	founded := false

	for _, server := range servers {
		selector, err := metav1.LabelSelectorAsSelector(server.Spec.PodSelector)
		if err != nil {
			return false, err
		}

		if selector.Matches(labels.Set(pod.Labels)) {
			if server.Spec.Port.IntValue() > 0 {
				if int(matchedPort.ContainerPort) == server.Spec.Port.IntValue() {
					founded = true
				}
			} else {
				if matchedPort.Name == server.Spec.Port.String() {
					founded = true
				}
			}
		}
	}
	return founded, nil
}
