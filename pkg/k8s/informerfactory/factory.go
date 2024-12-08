package informerfactory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type newSharedInformer func() cache.SharedIndexInformer

type InformerFactory struct {
	log           logr.Logger
	informers     map[string]cache.SharedIndexInformer
	clientSet     *kubernetes.Clientset
	lock          sync.Mutex
	k8sConfig     *rest.Config
	defaultResync time.Duration
}

func NewInformerFactory(log logr.Logger, k8sConfig *rest.Config) (*InformerFactory, error) {
	factory := &InformerFactory{
		log:           log.WithName("InformerFactory"),
		k8sConfig:     k8sConfig,
		defaultResync: time.Hour,
		informers:     make(map[string]cache.SharedIndexInformer),
	}
	clientSet, err := kubernetes.NewForConfig(factory.k8sConfig)
	if err != nil {
		log.Error(err, "Failed to create informer factory")
		return nil, err
	}
	factory.clientSet = clientSet
	return factory, nil
}

// Start 运行所有的Informer
func (f *InformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for name, informer := range f.informers {
		f.log.Info("STARTING informer", "name", name)
		go informer.Run(stopCh)
	}
}

// WaitForCacheSync 同步所有Informer的缓存数据
func (f *InformerFactory) WaitForCacheSync(stopCh <-chan struct{}) {
	var syncs []cache.InformerSynced

	f.lock.Lock()
	for name, informer := range f.informers {
		f.log.Info("Waiting for cache sync of informer", "name", name)
		syncs = append(syncs, informer.HasSynced)
	}
	f.lock.Unlock()

	cache.WaitForCacheSync(stopCh, syncs...)
}

func (f *InformerFactory) Node() cache.SharedIndexInformer {
	return f.getInformer("nodeInformer", func() cache.SharedIndexInformer {
		lw := f.newListWatchFromClient(f.clientSet.CoreV1().RESTClient(), "nodes", k8sv1.NamespaceAll, fields.Everything(), labels.Everything())
		return cache.NewSharedIndexInformer(lw, &k8sv1.Node{}, f.defaultResync, cache.Indexers{})
	})
}

func (f *InformerFactory) Pod() cache.SharedIndexInformer {
	return f.getInformer("podInformer", func() cache.SharedIndexInformer {
		lw := f.newListWatchFromClient(f.clientSet.CoreV1().RESTClient(), "pods", k8sv1.NamespaceAll, fields.Everything(), labels.Everything())
		return cache.NewSharedIndexInformer(lw, &k8sv1.Pod{}, f.defaultResync, cache.Indexers{
			"nodeNameIdx": func(obj any) ([]string, error) {
				pod, ok := obj.(*k8sv1.Pod)
				if !ok {
					return nil, fmt.Errorf("unexpected type %T", obj)
				}
				return []string{pod.Spec.NodeName}, nil
			},
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		})
	})
}

func (f *InformerFactory) getInformer(key string, newFunc newSharedInformer) cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informer, exists := f.informers[key]
	if exists {
		return informer
	}
	informer = newFunc()
	f.informers[key] = informer

	return informer
}

func (f *InformerFactory) newListWatchFromClient(c cache.Getter, resource string, namespace string, fieldSelector fields.Selector, labelSelector labels.Selector) *cache.ListWatch {
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		options.FieldSelector = fieldSelector.String()
		options.LabelSelector = labelSelector.String()
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			Do(context.Background()).
			Get()
	}
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		options.FieldSelector = fieldSelector.String()
		options.LabelSelector = labelSelector.String()
		options.Watch = true
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			Watch(context.Background())
	}
	return &cache.ListWatch{ListFunc: listFunc, WatchFunc: watchFunc}
}
