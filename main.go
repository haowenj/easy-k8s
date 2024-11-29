package main

import (
	"context"
	"flag"
	"net/http"
	"path/filepath"

	"easy-k8s/api"
	"easy-k8s/pkg/k8s/client"
	"easy-k8s/pkg/k8s/informerfactory"
	"easy-k8s/pkg/log"

	"k8s.io/client-go/util/homedir"
)

var (
	kubeconfig *string
	logger     = log.NewStdoutLogger()
	ctx        = context.Background()
)

func init() {
	var defaultKubeConfigPath string
	if home := homedir.HomeDir(); home != "" {
		defaultKubeConfigPath = filepath.Join(home, ".kube", "config")
	}

	kubeconfig = flag.String("kubeconfig", defaultKubeConfigPath, "absolute path to the kubeconfig file")

	flag.Parse()
}

func main() {
	k8sConfig, err := client.NewBaseConfig(kubeconfig)
	if err != nil {
		logger.Error(err, "create k8s config failed")
		return
	}

	factory, err := informerfactory.NewInformerFactory(logger, k8sConfig)
	if err != nil {
		logger.Error(err, "create informer failed")
		return
	}

	apiSvc := &api.ApiServer{}
	apiSvc.RunInformerFactory(factory, ctx)

	err = http.ListenAndServe(":9898", apiSvc.Engine())
	if err != nil {
		logger.Error(err, "Started web server")
		return
	}
}
