package api

import (
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"strings"

	"easy-k8s/pkg/comm"
)

type PodLogic struct {
	Log           logr.Logger
	DynamicClient dynamic.Interface
	NodeInformer  cache.SharedIndexInformer
	PodInformer   cache.SharedIndexInformer
}

type PodResource struct {
	Name       string       `json:"name"`
	Containers []*Container `json:"containers"`
}

type Container struct {
	Name    string            `json:"name"`
	Request map[string]string `json:"request"`
	Limits  map[string]string `json:"limits"`
}

func NewPodLogic(log logr.Logger, dynamicClient dynamic.Interface, nodeInformer, podInformer cache.SharedIndexInformer) *PodLogic {
	return &PodLogic{
		Log:           log.WithName("PodLogic"),
		DynamicClient: dynamicClient,
		NodeInformer:  nodeInformer,
		PodInformer:   podInformer,
	}
}

func (p *PodLogic) PodListByNs(ctx *gin.Context) {
	ns := ctx.Param("ns")
	if ns == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "namespace is empty"})
		return
	}

	objs, err := p.PodInformer.GetIndexer().ByIndex(cache.NamespaceIndex, ns)
	if err != nil {
		p.Log.Error(err, "get pod list by namespace")
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}

	var data []*PodData
	for _, obj := range objs {
		pod := obj.(*v1.Pod)

		var useGpu bool
		var useGpuCount, gpuProduct string
		var gpuQuantity *resource.Quantity
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[comm.LabelNVIDIA]; ok {
				useGpu = true
				node, _, err := p.NodeInformer.GetStore().GetByKey(pod.Spec.NodeName)
				if err != nil {
					p.Log.Error(err, "get node by key")
					continue
				}
				n := node.(*v1.Node)
				for key := range n.GetLabels() {
					if strings.HasPrefix(key, "osgalaxy.io-gpu-nvidia.com") {
						gpuProduct = strings.Split(key, "/")[1]
					}
				}
				nvAllocate := n.Status.Allocatable[comm.LabelNVIDIA]
				if gpuQuantity == nil {
					gpuQuantity = &nvAllocate
				} else {
					gpuQuantity.Add(nvAllocate)
				}
			}
		}
		if gpuQuantity != nil {
			useGpuCount = gpuQuantity.String()
		}
		row := &PodData{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Ip:          pod.Status.PodIP,
			NodeName:    pod.Spec.NodeName,
			Age:         translateTimestampSince(pod.CreationTimestamp),
			UseGpu:      useGpu,
			UseGpuCount: useGpuCount,
			GpuProduct:  gpuProduct,
			Status:      string(pod.Status.Phase),
		}
		data = append(data, row)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": data})
}

func (p *PodLogic) PodRelatedResource(ctx *gin.Context) {

}

func (p *PodLogic) getPod(key string) (*v1.Pod, error) {
	obj, exists, err := p.PodInformer.GetStore().GetByKey(key)
	if err != nil {
		p.Log.Error(err, "getPod error")
		return nil, err
	}
	if !exists {
		p.Log.Error(comm.PodNotFoundErr, "getPod error")
		return nil, comm.PodNotFoundErr
	}

	return obj.(*v1.Pod), nil
}
