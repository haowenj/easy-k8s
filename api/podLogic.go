package api

import (
	"easy-k8s/pkg/comm"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
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
