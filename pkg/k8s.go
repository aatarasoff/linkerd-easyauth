package common

import (
	"context"
	"fmt"
	policies "github.com/linkerd/linkerd2/controller/gen/apis/policy/v1alpha1"
	"github.com/linkerd/linkerd2/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"os"

	serverv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta1"
	serverauthorizationv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

type authCandidate struct {
	Server        serverv1beta1.Server
	Authorization k8s.Authorization
}

func AuthorizationsForResource(ctx context.Context, k8sAPI *k8s.KubernetesAPI, policies []*policies.AuthorizationPolicy, httpRoutes []*policies.HTTPRoute, serverAuthorizations []*serverauthorizationv1beta1.ServerAuthorization, servers []*serverv1beta1.Server, namespace string, resource string) ([]k8s.Authorization, error) {
	pods, err := k8s.GetPodsFor(ctx, k8sAPI, namespace, resource)
	if err != nil {
		return nil, err
	}

	results := make([]k8s.Authorization, 0)

	var candidates []authCandidate

	for _, saz := range serverAuthorizations {
		for _, srv := range servers {
			selector, err := metav1.LabelSelectorAsSelector(saz.Spec.Server.Selector)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create selector: %s\n", err)
				os.Exit(1)
			}

			if selector.Matches(labels.Set(srv.GetLabels())) || saz.Spec.Server.Name == srv.GetName() {
				authorization := k8s.Authorization{
					Server:              srv.GetName(),
					ServerAuthorization: saz.GetName(),
					AuthorizationPolicy: "",
				}
				candidates = append(candidates, authCandidate{Server: *srv, Authorization: authorization})
			}
		}
	}

	for _, policy := range policies {
		target := policy.Spec.TargetRef
		if target.Kind == "Namespace" || (target.Kind == k8s.ServerKind && target.Group == k8s.PolicyAPIGroup) {
			for _, srv := range servers {
				if target.Kind == "Namespace" || (string(target.Name) == srv.GetName()) {
					authorization := k8s.Authorization{
						Server:              srv.GetName(),
						ServerAuthorization: "",
						AuthorizationPolicy: policy.GetName(),
					}
					candidates = append(candidates, authCandidate{Server: *srv, Authorization: authorization})
				}
			}
		}

		if target.Kind == k8s.HTTPRouteKind {
			for _, httpRoute := range httpRoutes {
				for _, targetRef := range httpRoute.Spec.ParentRefs {
					if *targetRef.Kind == k8s.ServerKind {
						for _, srv := range servers {
							if *targetRef.Kind == k8s.ServerKind && string(targetRef.Name) == srv.GetName() && string(policy.Spec.TargetRef.Name) == httpRoute.GetName() {
								authorization := k8s.Authorization{
									Route:               httpRoute.Name,
									Server:              srv.GetName(),
									ServerAuthorization: "",
									AuthorizationPolicy: policy.GetName(),
								}
								candidates = append(candidates, authCandidate{Server: *srv, Authorization: authorization})
							}
						}
					}
				}
			}
		}
	}

	for _, candidate := range candidates {
		server := candidate.Server
		if server.Spec.PodSelector == nil {
			continue
		}

		selector, err := metav1.LabelSelectorAsSelector(server.Spec.PodSelector)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create selector: %s\n", err)
			os.Exit(1)
		}

		var selectedPods []corev1.Pod
		for _, pod := range pods {
			if selector.Matches(labels.Set(pod.Labels)) {
				selectedPods = append(selectedPods, pod)
			}
		}

		if serverIncludesPod(server, selectedPods) {
			results = append(results, candidate.Authorization)
		}
	}

	return results, nil
}

func serverIncludesPod(server serverv1beta1.Server, serverPods []corev1.Pod) bool {
	for _, pod := range serverPods {
		for _, container := range pod.Spec.Containers {
			for _, p := range container.Ports {
				if server.Spec.Port.IntVal == p.ContainerPort || server.Spec.Port.StrVal == p.Name {
					return true
				}
			}
		}
	}
	return false
}
