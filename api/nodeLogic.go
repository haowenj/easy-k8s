package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"easy-k8s/pkg/comm"
	eresource "easy-k8s/pkg/k8s/resource"
)

type NodeLogic struct {
	Log           logr.Logger
	DynamicClient dynamic.Interface
	NodeInformer  cache.SharedIndexInformer
	PodInformer   cache.SharedIndexInformer
}

var displayFileds = map[string]struct{}{
	"status":           {},
	"roles":            {},
	"age":              {},
	"version":          {},
	"internalIP":       {},
	"externalIP":       {},
	"osImage":          {},
	"kernelVersion":    {},
	"containerRuntime": {},
	"gpuProduct":       {},
}

type NodeListData struct {
	Name             string `json:"name"`
	Status           string `json:"status,omitempty"`
	Roles            string `json:"roles,omitempty"`
	Age              string `json:"age,omitempty"`
	Version          string `json:"version,omitempty"`
	InternalIP       string `json:"internalIP,omitempty"`
	ExternalIP       string `json:"externalIP,omitempty"`
	OsImage          string `json:"osImage,omitempty"`
	KernelVersion    string `json:"kernelVersion,omitempty"`
	ContainerRuntime string `json:"containerRuntime,omitempty"`
	GpuProduct       string `json:"gpuProduct,omitempty"`
}

type NodeListReq struct {
	NodeRole string `json:"nodeRole" form:"nodeRole"`
	HasGpu   bool   `json:"hasGpu" form:"hasGpu"`
	Name     string `json:"name" form:"name"`
}

type NodeLabelPatchReq struct {
	Labels []*struct {
		Op     string `json:"op"`
		Encode bool   `json:"encode"`
		Key    string `json:"key"`
		Value  string `json:"value"`
	} `json:"labels"`
}

func NewNodeLogic(log logr.Logger, dynamicClient dynamic.Interface, nodeInformer, podInformer cache.SharedIndexInformer) *NodeLogic {
	return &NodeLogic{
		Log:           log.WithName("NodeLogic"),
		DynamicClient: dynamicClient,
		NodeInformer:  nodeInformer,
		PodInformer:   podInformer,
	}
}

func (n *NodeLogic) GetDisplayFileds(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": displayFileds})
}

func (n *NodeLogic) SetDisplayFileds(ctx *gin.Context) {
	var fields []string
	if err := ctx.BindJSON(&fields); err != nil {
		n.Log.Error(err, "bind json err")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}
	displayFileds = map[string]struct{}{}
	for _, field := range fields {
		displayFileds[field] = struct{}{}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": displayFileds})
}

func (n *NodeLogic) GetNodeList(ctx *gin.Context) {
	var rows []*NodeListData

	var req NodeListReq
	if err := ctx.BindQuery(&req); err != nil {
		n.Log.Error(err, "bind query err")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, obj := range n.NodeInformer.GetStore().List() {
		node := obj.(*v1.Node)

		var isGpuNode bool
		gpuProduct := "-"
		for key := range node.Labels {
			if strings.HasPrefix(key, "osgalaxy.io-gpu-nvidia.com") {
				isGpuNode = true
				gpuProduct = strings.Split(key, "/")[1]
			}
		}

		if len(req.NodeRole) != 0 {
			if role, ok := node.Labels["osgalaxy.io/role"]; !ok || role != req.NodeRole {
				continue
			}
		}
		if len(req.Name) != 0 {
			if !strings.Contains(node.Name, req.Name) {
				continue
			}
		}
		if req.HasGpu {
			if !isGpuNode {
				continue
			}
		}
		data := &NodeListData{}
		data.Name = node.Name
		if _, ok := displayFileds["age"]; ok {
			data.Age = translateTimestampSince(node.CreationTimestamp)
		}
		if _, ok := displayFileds["gpuProduct"]; ok {
			data.GpuProduct = gpuProduct
		}
		if _, ok := displayFileds["version"]; ok {
			data.Version = node.Status.NodeInfo.KubeletVersion
		}
		if _, ok := displayFileds["kernelVersion"]; ok {
			data.KernelVersion = node.Status.NodeInfo.KubeletVersion
		}
		if _, ok := displayFileds["osImage"]; ok {
			data.OsImage = node.Status.NodeInfo.OSImage
		}
		if _, ok := displayFileds["containerRuntime"]; ok {
			data.ContainerRuntime = node.Status.NodeInfo.ContainerRuntimeVersion
		}
		if _, ok := displayFileds["internalIP"]; ok {
			for _, address := range node.Status.Addresses {
				if address.Type == v1.NodeInternalIP {
					data.InternalIP = address.Address
				}
			}
		}
		if _, ok := displayFileds["status"]; ok {
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady {
					if condition.Status == v1.ConditionTrue {
						data.Status = "Ready"
					} else if condition.Status == v1.ConditionUnknown {
						data.Status = "NotReady"
					} else {
						data.Status = "Unknown"
					}
				}
			}
		}
		if _, ok := displayFileds["roles"]; ok {
			roles := []string{}
			for k, v := range node.Labels {
				switch {
				case strings.HasPrefix(k, comm.LabelNodeRolePrefix):
					if role := strings.TrimPrefix(k, comm.LabelNodeRolePrefix); len(role) > 0 {
						roles = append(roles, role)
					}

				case k == comm.NodeLabelRole && v != "":
					roles = append(roles, v)
				}
			}
			data.Roles = strings.Join(roles, ",")
		}
		rows = append(rows, data)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": rows})
}

func (n *NodeLogic) NodeLabels(ctx *gin.Context) {
	name := ctx.Param("node")
	tagType := ctx.Query("tagType")
	if len(name) == 0 || len(tagType) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "request parameter error"})
		return
	}

	node, err := n.getNodeByName(name)
	if err != nil {
		if errors.Is(err, comm.NodeNotFoundErr) {
			ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
			return
		}
		n.Log.Error(err, "get node err")
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	var labels []string
	for k, v := range node.GetLabels() {
		if _, ok := comm.DecodeLables[k]; ok {
			v, _ = comm.Base64UrlDecode(v)
		}
		if strings.HasPrefix(k, comm.LabelCustomPrefix) && tagType == "custom" {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
		if !strings.HasPrefix(k, comm.LabelCustomPrefix) && tagType == "sys" {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"data": labels})
}

func (n *NodeLogic) NodeLabelPatch(ctx *gin.Context) {
	name := ctx.Param("node")
	if len(name) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "request parameter error"})
		return
	}

	var newLabels *NodeLabelPatchReq
	if err := ctx.BindJSON(&newLabels); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}

	node, err := n.getNodeByName(name)
	if err != nil {
		if errors.Is(err, comm.NodeNotFoundErr) {
			n.Log.Error(err, "get node err")
			ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	labels := node.GetLabels()
	for _, l := range newLabels.Labels {
		if l.Op == "remove" {
			delete(labels, l.Key)
			continue
		}
		if l.Encode {
			l.Value = comm.Base64UrlEncode(l.Value)
		}
		labels[l.Key] = l.Value
	}

	patchData := []comm.PatchOperation{{Op: "replace", Path: "/metadata/labels", Value: labels}}
	//这里有另一种写法，路径中的~1是对字符/的编码，因为在JSON Pointer中，/是一个特殊字符，用于分隔路径的各个部分。因此，osgalaxy.io/supplier-status在JSON Patch的路径中被编码为osgalaxy.io~1supplier-status
	//patchData := `[{"op": "remove", "path": "/metadata/labels/osgalaxy.io~1supplier-status"}, {"op": "remove", "path": "/metadata/labels/osgalaxy.io~1sale-type"}]`

	playLoadBytes, err := json.Marshal(patchData)
	if err != nil {
		n.Log.Error(err, "json marshal err")
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	if _, err = n.DynamicClient.Resource(comm.NodeGVR).Patch(ctx, name, types.JSONPatchType, playLoadBytes, metav1.PatchOptions{}); err != nil {
		n.Log.Error(err, "patch err")
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (n *NodeLogic) NodeResource(ctx *gin.Context) {
	name := ctx.Param("node")
	if len(name) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "request parameter error"})
		return
	}
	node, err := n.getNodeByName(name)
	if err != nil {
		if errors.Is(err, comm.NodeNotFoundErr) {
			ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	total := make(map[string]string)
	var product string
	cpuAllocatable := node.Status.Allocatable[v1.ResourceCPU]
	memAllocatable := node.Status.Allocatable[v1.ResourceMemory]
	total["cpu"] = cpuAllocatable.String()
	total["memory"] = memAllocatable.String()
	if _, ok := node.Status.Allocatable[comm.LabelNVIDIA]; ok {
		for key := range node.GetLabels() {
			if strings.HasPrefix(key, "osgalaxy.io-gpu-nvidia.com") {
				product = strings.Split(key, "/")[1]
			}
		}
		nvAllocate := node.Status.Allocatable[comm.LabelNVIDIA]
		total[product] = nvAllocate.String()
	}
	used := make(map[string]string)
	objs, err := n.PodInformer.GetIndexer().ByIndex("nodeNameIdx", node.GetName())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	reqs, _ := n.getPodsTotalRequestsAndLimits(objs)
	cpuReq := reqs[v1.ResourceCPU]
	memReq := reqs[v1.ResourceMemory]
	used["cpu"] = cpuReq.String()
	used["memory"] = memReq.String()
	if _, ok := reqs[comm.LabelNVIDIA]; ok {
		nvReq := reqs[comm.LabelNVIDIA]
		used[product] = nvReq.String()
	}
	ctx.JSON(http.StatusOK, gin.H{"data": map[string]map[string]string{
		"total": total,
		"used":  used,
	}})
}

func (n *NodeLogic) NodePodList(ctx *gin.Context) {
	name := ctx.Param("node")
	if len(name) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "request parameter error"})
		return
	}

	onlyGpu := ctx.Query("onlyGpu")
	node, err := n.getNodeByName(name)
	if err != nil {
		if errors.Is(err, comm.NodeNotFoundErr) {
			ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	objs, err := n.PodInformer.GetIndexer().ByIndex("nodeNameIdx", node.GetName())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
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
				for key := range node.GetLabels() {
					if strings.HasPrefix(key, "osgalaxy.io-gpu-nvidia.com") {
						gpuProduct = strings.Split(key, "/")[1]
					}
				}
				nvAllocate := node.Status.Allocatable[comm.LabelNVIDIA]
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

		if !useGpu && onlyGpu == "true" {
			continue
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

func (n *NodeLogic) getNodeByName(name string) (*v1.Node, error) {
	obj, exists, err := n.NodeInformer.GetStore().GetByKey(name)
	if err != nil {
		n.Log.Error(err, "getNode error")
		return nil, err
	}
	if !exists {
		n.Log.Error(comm.NodeNotFoundErr, "getNode error")
		return nil, comm.NodeNotFoundErr
	}

	return obj.(*v1.Node), nil
}

func (n *NodeLogic) getPodsTotalRequestsAndLimits(podList []any) (reqs map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) {
	reqs, limits = map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	for _, obj := range podList {
		pod := obj.(*v1.Pod)
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		podReqs, podLimits := eresource.PodRequestsAndLimits(pod)
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = podReqValue.DeepCopy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = podLimitValue.DeepCopy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	return
}
