package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"easy-k8s/pkg/comm"
)

var nodeNotFoundErr = errors.New("node not found")

type NodeLogic struct {
	DynamicClient dynamic.Interface
	NodeInformer  cache.SharedIndexInformer
	PodInformer   cache.SharedIndexInformer
}

type NodeListData struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Roles            string `json:"roles"`
	Age              string `json:"age"`
	Version          string `json:"version"`
	InternalIP       string `json:"internalIP"`
	ExternalIP       string `json:"externalIP"`
	OsImage          string `json:"osImage"`
	KernelVersion    string `json:"kernelVersion"`
	ContainerRuntime string `json:"containerRuntime"`
}

type NodeLabelPatchReq struct {
	Labels []*struct {
		Op     string `json:"op"`
		Encode bool   `json:"encode"`
		Key    string `json:"key"`
		Value  string `json:"value"`
	} `json:"labels"`
}

func (n *NodeLogic) GetNodeList(ctx *gin.Context) {
	var rows []*NodeListData

	for _, obj := range n.NodeInformer.GetStore().List() {
		node := obj.(*v1.Node)
		data := &NodeListData{}
		data.Name = node.Name
		data.Age = calculateAge(node.CreationTimestamp.Time)
		data.Version = node.Status.NodeInfo.KubeletVersion
		data.KernelVersion = node.Status.NodeInfo.KubeletVersion
		data.OsImage = node.Status.NodeInfo.OSImage
		data.ContainerRuntime = node.Status.NodeInfo.ContainerRuntimeVersion
		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeInternalIP {
				data.InternalIP = address.Address
			}
		}
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
		if errors.Is(err, nodeNotFoundErr) {
			ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
			return
		}
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
		if errors.Is(err, nodeNotFoundErr) {
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

	patchData := comm.PatchOperation{Op: "replace", Path: "/metadata/labels", Value: labels}

	playLoadBytes, err := json.Marshal(patchData)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	if _, err = n.DynamicClient.Resource(comm.NodeGVR).Patch(ctx, name, types.MergePatchType, playLoadBytes, metav1.PatchOptions{}); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (n *NodeLogic) getNodeByName(name string) (*v1.Node, error) {
	obj, err := n.NodeInformer.GetIndexer().ByIndex("nodeNameIdx", name)
	if err != nil {
		return nil, err
	}
	if len(obj) == 0 {
		return nil, nodeNotFoundErr
	}
	return obj[0].(*v1.Node), nil
}

func (n *NodeLogic) NodeResource(ctx *gin.Context) {
	name := ctx.Param("node")
	if len(name) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "request parameter error"})
		return
	}
	node, err := n.getNodeByName(name)
	if err != nil {
		if errors.Is(err, nodeNotFoundErr) {
			ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	total := make(map[string]string)
	var product string
	for resourceName, quantity := range node.Status.Allocatable {
		if resourceName.String() == "cpu" {
			total["cpu"] = fmt.Sprintf("%dc", quantity.Value())
		}
		if resourceName.String() == "memory" {
			total["memory"] = fmt.Sprintf("%dg", quantity.Value()/(1024*1024*1024))
		}
		if resourceName.String() == "nvidia.com/gpu" {
			for key := range node.GetLabels() {
				if strings.HasPrefix(key, "osgalaxy.io-gpu-nvidia.com") {
					product = strings.Split(key, "/")[1]
				}
			}
			total[product] = fmt.Sprintf("%d", quantity.Value())
		}
	}

	used := make(map[string]string)
	objs, err := n.PodInformer.GetIndexer().ByIndex("nodeNameIdx", node.GetName())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	var cpu, mem, gpu int64
	for _, obj := range objs {
		pod := obj.(*v1.Pod)
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
				req := container.Resources.Requests["nvidia.com/gpu"]
				gpu += req.Value()
			}
			if _, ok := container.Resources.Requests["cpu"]; ok {
				req := container.Resources.Requests["cpu"]
				cpu += req.Value()
			}
			if _, ok := container.Resources.Requests["memory"]; ok {
				req := container.Resources.Requests["memory"]
				mem += req.Value()
			}
		}
	}
	used["cpu"] = fmt.Sprintf("%dc", cpu)
	used["memory"] = fmt.Sprintf("%dg", mem/(1024*1024*1024))
	if gpu > 0 {
		used[product] = fmt.Sprintf("%d", gpu)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": map[string]map[string]string{
		"total": total,
		"used":  used,
	}})
}

func calculateAge(creationTime time.Time) string {
	now := time.Now()
	duration := now.Sub(creationTime)

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if days > 0 {
		if days > 5 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%d秒", seconds)
	}
}
