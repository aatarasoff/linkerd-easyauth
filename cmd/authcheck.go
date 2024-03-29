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
	"reflect"
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
							break
						}

						if serverAuthorization.Spec.Server.Name == server.GetName() {
							founded = true
							break
						}
					}

					if !founded {
						for _, policy := range resources.AuthorizationPolicies {
							// namespaced policies applies on each server
							if policy.Spec.TargetRef.Kind == "Namespace" && policy.GetNamespace() == server.Namespace {
								founded = true
								break
							}

							if policy.Spec.TargetRef.Kind == k8s.ServerKind && string(policy.Spec.TargetRef.Name) == server.GetName() {
								founded = true
								break
							}

							if policy.Spec.TargetRef.Kind == k8s.HTTPRouteKind {
								for _, httpRoute := range resources.HTTPRoutes {
									for _, targetRef := range httpRoute.Spec.ParentRefs {
										if *targetRef.Kind == k8s.ServerKind && string(targetRef.Name) == server.GetName() && string(policy.Spec.TargetRef.Name) == httpRoute.GetName() {
											founded = true
											break
										}
									}
								}
							}
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
							if policy.Spec.TargetRef.Kind == k8s.ServerKind && (string(policy.Spec.TargetRef.Name) == server.GetName()) {
								founded = true
								break
							}
						}

						for _, httpRoute := range resources.HTTPRoutes {
							if policy.Spec.TargetRef.Kind == k8s.HTTPRouteKind {
								for _, server := range resources.Servers {
									for _, targetRef := range httpRoute.Spec.ParentRefs {
										if *targetRef.Kind == k8s.ServerKind && string(targetRef.Name) == server.GetName() && string(policy.Spec.TargetRef.Name) == httpRoute.GetName() {
											founded = true
											break
										}
									}
								}
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
		*healthcheck.NewChecker("linkerd-easyauth no obsolete HTTPRoutes").
			Warning().
			WithCheck(func(ctx context.Context) error {
				httpRoutesWithObsoleteTargetRef := []string{}

				for _, httpRoute := range resources.HTTPRoutes {
					for _, targetRef := range httpRoute.Spec.ParentRefs {
						founded := false

						if *targetRef.Kind == k8s.ServerKind {
							for _, server := range resources.Servers {
								if string(targetRef.Name) == server.GetName() {
									founded = true
									break
								}
							}
						}

						if !founded {
							httpRoutesWithObsoleteTargetRef = append(
								httpRoutesWithObsoleteTargetRef,
								fmt.Sprintf("TargetRef %s in HTTPPolicy %s is obsolete (eg. doesn't apply to any Server)",
									string(targetRef.Name),
									httpRoute.GetName(),
								),
							)
						}
					}
				}

				if len(httpRoutesWithObsoleteTargetRef) == 0 {
					return nil
				}
				return fmt.Errorf("Some HTTPRoutes have obsolete targetRef:\n\t%s", strings.Join(httpRoutesWithObsoleteTargetRef, "\n\t"))
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

	checkers = append(checkers,
		*healthcheck.NewChecker("linkerd-easyauth no obsolete authentications").
			Warning().
			WithCheck(func(ctx context.Context) error {
				var authentications []metav1.Object
				var obsoleteAuthentications []string

				for _, authn := range resources.MeshTLSAuthentications {
					authentications = append(authentications, authn)
				}

				for _, authn := range resources.NetworkAuthentications {
					authentications = append(authentications, authn)
				}

				for _, authn := range authentications {
					founded := false

					for _, policy := range resources.AuthorizationPolicies {
						for _, targetRef := range policy.Spec.RequiredAuthenticationRefs {
							if string(targetRef.Name) == authn.GetName() {
								founded = true
								break
							}
						}
					}

					if !founded {
						obsoleteAuthentications = append(
							obsoleteAuthentications,
							fmt.Sprintf("%s %s is obsolete",
								strings.Split(reflect.TypeOf(authn).String(), ".")[1],
								authn.GetName(),
							),
						)
					}
				}

				if len(obsoleteAuthentications) == 0 {
					return nil
				}
				return fmt.Errorf("Some authentications are obsolete (eg. doesn't apply to any policy):\n\t%s", strings.Join(obsoleteAuthentications, "\n\t"))
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
