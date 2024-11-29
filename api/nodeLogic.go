package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"strings"
	"time"
)

const (
	NodeLabelRole       = "kubernetes.io/role"
	LabelNodeRolePrefix = "node-role.kubernetes.io/"
	LabelCustomPrefix   = "osgalaxy.io"
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

type NodeTag struct {
	Tag map[string]string `json:"tag"`
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
			case strings.HasPrefix(k, LabelNodeRolePrefix):
				if role := strings.TrimPrefix(k, LabelNodeRolePrefix); len(role) > 0 {
					roles = append(roles, role)
				}

			case k == NodeLabelRole && v != "":
				roles = append(roles, v)
			}
		}
		data.Roles = strings.Join(roles, ",")
		rows = append(rows, data)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": rows})
}

func (n *NodeLogic) NodeTag(ctx *gin.Context) {
	name := ctx.Param("node")
	tagType := ctx.Query("tagType")
	if len(name) == 0 || len(tagType) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": "request parameter error"})
		return
	}

	obj, err := n.NodeInformer.GetIndexer().ByIndex("nodeNameIdx", name)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	if len(obj) == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"msg": "node not found"})
		return
	}

	node := obj[0].(*v1.Node)
	labels := map[string]string{}
	for k, v := range node.GetLabels() {
		if strings.HasPrefix(k, LabelCustomPrefix) && tagType == "custom" {
			labels[k] = v
		}
		if !strings.HasPrefix(k, LabelCustomPrefix) && tagType == "sys" {
			labels[k] = v
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"data": &NodeTag{Tag: labels}})
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
