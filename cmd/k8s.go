package cmd

import (
	"context"
	"fmt"
	policy "github.com/linkerd/linkerd2/controller/gen/apis/policy/v1alpha1"
	server "github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta1"
	saz "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"
	l5dcrdinformer "github.com/linkerd/linkerd2/controller/gen/client/informers/externalversions"
	pkgK8s "github.com/linkerd/linkerd2/controller/k8s"
	"github.com/linkerd/linkerd2/pkg/k8s"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"os"
	"time"
)

type K8sResources struct {
	Pods                   *v1.PodList
	Services               *v1.ServiceList
	Servers                []*server.Server
	ServerAuthorizations   []*saz.ServerAuthorization
	AuthorizationPolicies  []*policy.AuthorizationPolicy
	HTTPRoutes             []*policy.HTTPRoute
	MeshTLSAuthentications []*policy.MeshTLSAuthentication
	NetworkAuthentications []*policy.NetworkAuthentication
}

func FetchK8sResources(ctx context.Context, namespace string) (*K8sResources, error) {
	k8sAPI, err := k8s.NewAPI(kubeconfigPath, kubeContext, impersonate, impersonateGroup, 0)
	if err != nil {
		return nil, err
	}

	lr5dAPI := initServerAPI(kubeconfigPath)

	pods, err := k8sAPI.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services, err := k8sAPI.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	servers, err := lr5dAPI.Server().V1beta1().Servers().Lister().Servers(namespace).List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	serverAuthorizations, err := lr5dAPI.Serverauthorization().V1beta1().ServerAuthorizations().Lister().ServerAuthorizations(namespace).List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	authorizationPolicies, err := lr5dAPI.Policy().V1alpha1().AuthorizationPolicies().Lister().AuthorizationPolicies(namespace).List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	httpRoutes, err := lr5dAPI.Policy().V1alpha1().HTTPRoutes().Lister().HTTPRoutes(namespace).List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	meshTLSAuthentications, err := lr5dAPI.Policy().V1alpha1().MeshTLSAuthentications().Lister().MeshTLSAuthentications(namespace).List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	newtworkAuthentications, err := lr5dAPI.Policy().V1alpha1().NetworkAuthentications().Lister().NetworkAuthentications(namespace).List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	return &K8sResources{
		Pods:                   pods,
		Services:               services,
		Servers:                servers,
		ServerAuthorizations:   serverAuthorizations,
		AuthorizationPolicies:  authorizationPolicies,
		HTTPRoutes:             httpRoutes,
		MeshTLSAuthentications: meshTLSAuthentications,
		NetworkAuthentications: newtworkAuthentications,
	}, nil
}

func initServerAPI(kubeconfigPath string) l5dcrdinformer.SharedInformerFactory {
	config, err := k8s.GetConfig(kubeconfigPath, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lr5dClient, err := pkgK8s.NewL5DCRDClient(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lr5dAPI := l5dcrdinformer.NewSharedInformerFactory(lr5dClient, 10*time.Minute)

	stopCh := make(chan struct{})
	go lr5dAPI.Server().V1beta1().Servers().Informer().Run(stopCh)
	go lr5dAPI.Serverauthorization().V1beta1().ServerAuthorizations().Informer().Run(stopCh)
	go lr5dAPI.Policy().V1alpha1().AuthorizationPolicies().Informer().Run(stopCh)
	go lr5dAPI.Policy().V1alpha1().MeshTLSAuthentications().Informer().Run(stopCh)
	go lr5dAPI.Policy().V1alpha1().NetworkAuthentications().Informer().Run(stopCh)
	go lr5dAPI.Policy().V1alpha1().HTTPRoutes().Informer().Run(stopCh)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !cache.WaitForCacheSync(ctx.Done(), lr5dAPI.Server().V1beta1().Servers().Informer().HasSynced, lr5dAPI.Serverauthorization().V1beta1().ServerAuthorizations().Informer().HasSynced) {
		fmt.Fprintln(os.Stderr, "failed to initialized client")
		os.Exit(1)
	}

	return lr5dAPI
}
