package informerfactory

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type InformerFactory struct {
	log           logr.Logger
	informers     map[string]cache.SharedIndexInformer
	clientSet     *kubernetes.Clientset
	lock          sync.Mutex
	k8sConfig     *rest.Config
	defaultResync time.Duration
}
