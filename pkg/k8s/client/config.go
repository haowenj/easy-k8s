package client

import (
	"errors"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewBaseConfig(kubeConfigPath *string) (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if errors.Is(err, rest.ErrNotInCluster) {
		return clientcmd.BuildConfigFromFlags("", *kubeConfigPath)
	}
	return config, err
}
