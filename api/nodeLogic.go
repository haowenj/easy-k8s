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
	ctx.JSON(http.StatusOK, rows)
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
