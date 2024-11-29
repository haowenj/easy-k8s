package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"time"
)

type NodeLogic struct {
	NodeInformer cache.SharedIndexInformer
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

func (n *NodeLogic) GetNodeList(ctx *gin.Context) {
	var rows []*NodeListData

	for _, obj := range n.NodeInformer.GetStore().List() {
		node := obj.(*v1.Node)
		data := &NodeListData{}
		data.Name = node.Name
		data.Age = fmt.Sprintf("%d", time.Now().Unix()-node.CreationTimestamp.Unix())
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
				} else if condition.Status == v1.ConditionFalse {
					data.Status = "NotReady"
				} else {
					data.Status = "Unknown"
				}
			}
		}
		rows = append(rows, data)
	}
	ctx.JSON(http.StatusOK, rows)
}
