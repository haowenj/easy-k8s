package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
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
		var useGpuCount int
		var gpuProduct string
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[comm.LabelNVIDIA]; ok {
				useGpu = true
				req := container.Resources.Requests[comm.LabelNVIDIA]
				useGpuCount += int(req.Value())

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
			}
		}
		row := &PodData{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Ip:          pod.Status.PodIP,
			NodeName:    pod.Spec.NodeName,
			Age:         calculateAge(pod.CreationTimestamp.Time),
			UseGpu:      useGpu,
			UseGpuCount: useGpuCount,
			GpuProduct:  gpuProduct,
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady {
				if condition.Status == v1.ConditionTrue {
					row.Status = "Ready"
				} else {
					row.Status = "NotReady"
				}
				break
			}

		}
		data = append(data, row)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": data})
}

func (p *PodLogic) PodResourceInfo(ctx *gin.Context) {
	namespace := ctx.Param("ns")
	name := ctx.Param("name")
	if namespace == "" || name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "namespace or name is null"})
		return
	}

	pod, err := p.getPod(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	res := &PodResource{Name: pod.Name, Containers: make([]*Container, 0)}
	for _, container := range pod.Spec.Containers {
		containerRes := &Container{Name: container.Name, Request: make(map[string]string), Limits: make(map[string]string)}
		for key, val := range container.Resources.Requests {
			containerRes.Request[key.String()] = val.String()
		}
		for key, val := range container.Resources.Limits {
			containerRes.Limits[key.String()] = val.String()
		}
		res.Containers = append(res.Containers, containerRes)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": res})
}

func (p *PodLogic) PodLabels(ctx *gin.Context) {
	namespace := ctx.Param("ns")
	name := ctx.Param("name")
	if namespace == "" || name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "namespace or name is null"})
		return
	}
	pod, err := p.getPod(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	res := []string{}
	for key, val := range pod.GetLabels() {
		res = append(res, fmt.Sprintf("%s=%s", key, val))
	}
	ctx.JSON(http.StatusOK, gin.H{"data": res})
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
