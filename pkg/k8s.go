package common

import (
	"context"
	"fmt"
	"github.com/linkerd/linkerd2/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"os"

	serverv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta1"
	serverauthorizationv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func ServerAuthorizationsForResource(ctx context.Context, k8sAPI *k8s.KubernetesAPI, serverAuthorizations []*serverauthorizationv1beta1.ServerAuthorization, servers []*serverv1beta1.Server, namespace string, resource string) ([]k8s.ServerAndAuthorization, error) {
	pods, err := k8s.GetPodsFor(ctx, k8sAPI, namespace, resource)
	if err != nil {
		return nil, err
	}

	results := make([]k8s.ServerAndAuthorization, 0)

	for _, saz := range serverAuthorizations {
		var selectedServers []serverv1beta1.Server

		for _, srv := range servers {
			selector, err := metav1.LabelSelectorAsSelector(saz.Spec.Server.Selector)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create selector: %s\n", err)
				os.Exit(1)
			}

			if selector.Matches(labels.Set(srv.GetLabels())) || saz.Spec.Server.Name == srv.GetName() {
				selectedServers = append(selectedServers, *srv)
			}
		}

		for _, server := range selectedServers {
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
				results = append(results, k8s.ServerAndAuthorization{server.GetName(), saz.GetName()})
			}
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
