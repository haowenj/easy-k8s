package test

import (
	"easy-k8s/pkg/k8s/client"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/homedir"
	"log"
	"path/filepath"
	"testing"
)

var defaultKubeConfigPath string

func init() {
	if home := homedir.HomeDir(); home != "" {
		defaultKubeConfigPath = filepath.Join(home, ".kube", "config")
	}
}

func TestMapper(t *testing.T) {
	k8sConfig, err := client.NewBaseConfig(&defaultKubeConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(k8sConfig)
	if err != nil {
		log.Fatal(err)
	}

	gr, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		log.Fatal(err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	fullySpecifiedGVR, groupResource := schema.ParseResourceArg("iptables-eips.kubeovn.io")
	gvk := schema.GroupVersionKind{}
	if fullySpecifiedGVR != nil {
		gvk, _ = (mapper).KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		gvk, _ = (mapper).KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		mapping, err := (mapper).RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(mapping.Resource.Group)
		fmt.Println(mapping.Resource.Version)
		fmt.Println(mapping.Resource.Resource)
	}
}
