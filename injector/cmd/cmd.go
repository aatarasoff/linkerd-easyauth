package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/linkerd/linkerd2/controller/k8s"
	"github.com/linkerd/linkerd2/controller/webhook"
	"github.com/linkerd/linkerd2/pkg/flags"
	"linkerd-easyauth/injector/mutator"
	"os"
)

const componentName = "linkerd-easyauth-injector"

func main() {
	cmd := flag.NewFlagSet("injector", flag.ExitOnError)
	metricsAddr := cmd.String("metrics-addr", fmt.Sprintf(":%d", 9995),
		"address to serve scrapable metrics on")
	addr := cmd.String("addr", ":8443", "address to serve on")
	kubeconfig := cmd.String("kubeconfig", "", "path to kubeconfig")
	enablePprof := cmd.Bool("enable-pprof", false, "Enable pprof endpoints on the admin server")

	flags.ConfigureAndParse(cmd, os.Args[1:])

	webhook.Launch(
		context.Background(),
		[]k8s.APIResource{k8s.NS},
		mutator.Mutate(),
		componentName,
		*metricsAddr,
		*addr,
		*kubeconfig,
		*enablePprof,
	)
}
