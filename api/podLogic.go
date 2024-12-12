package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"easy-k8s/pkg/comm"
)

type PodLogic struct {
	Log           logr.Logger
	DynamicClient dynamic.Interface
	NodeInformer  cache.SharedIndexInformer
	PodInformer   cache.SharedIndexInformer
}

type PodVolumeData struct {
	Name          string `json:"name,omitempty"`
	ResourceName  string `json:"resourceName,omitempty"`
	Option        *bool  `json:"option,omitempty"`
	HostPath      string `json:"hostPath,omitempty"`
	HostPathType  string `json:"hostPathType,omitempty"`
	ReadOnly      *bool  `json:"readOnly,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	MountPath     string `json:"mountPath,omitempty"`
	VolumeType    string `json:"volumeType,omitempty"`
}

type PodAssociatedResourcesRsp struct {
	PodName    string                    `json:"podName"`
	VolumeData map[string]*PodVolumeData `json:"volumeData"`
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

func (p *PodLogic) PodAssociatedResources(ctx *gin.Context) {
	ns := ctx.Param("ns")
	name := ctx.Param("name")
	pod, err := p.getPod(fmt.Sprintf("%s/%s", ns, name))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}
	data := &PodAssociatedResourcesRsp{PodName: name}
	volumeData := p.volumeData(pod.Spec.Volumes)
	mountInfo := p.mouthInfo(pod.Spec.Containers)
	for cname, mount := range mountInfo {
		for mname, path := range mount {
			if _, ok := volumeData[mname]; ok {
				volumeData[mname].ContainerName = cname
				volumeData[mname].MountPath = path
			}
		}
	}
	data.VolumeData = volumeData
	ctx.JSON(http.StatusOK, gin.H{"data": data})
}

func (p *PodLogic) mouthInfo(containers []v1.Container) map[string]map[string]string {
	data := make(map[string]map[string]string)
	for _, container := range containers {
		data[container.Name] = make(map[string]string)
		for _, mount := range container.VolumeMounts {
			data[container.Name][mount.Name] = mount.MountPath
		}
	}
	return data
}

func (p *PodLogic) volumeData(volumes []v1.Volume) map[string]*PodVolumeData {
	data := make(map[string]*PodVolumeData, 0)
	for _, volume := range volumes {
		if volume.ConfigMap != nil {
			optional := volume.ConfigMap.Optional != nil && *volume.ConfigMap.Optional
			data[volume.Name] = &PodVolumeData{
				ResourceName: volume.ConfigMap.Name,
				Option:       &optional,
				VolumeType:   comm.VolumeConfigMap,
			}
			continue
		}
		if volume.Secret != nil {
			optional := volume.Secret.Optional != nil && *volume.Secret.Optional
			data[volume.Name] = &PodVolumeData{
				ResourceName: volume.Secret.SecretName,
				Option:       &optional,
				VolumeType:   comm.VolumeSecret,
			}
			continue
		}
		if volume.HostPath != nil {
			data[volume.Name] = &PodVolumeData{
				HostPath:     volume.HostPath.Path,
				HostPathType: string(*volume.HostPath.Type),
				VolumeType:   comm.VolumeHostPath,
			}
			continue
		}
		if volume.PersistentVolumeClaim != nil {
			data[volume.Name] = &PodVolumeData{
				ResourceName: volume.PersistentVolumeClaim.ClaimName,
				ReadOnly:     &volume.PersistentVolumeClaim.ReadOnly,
				VolumeType:   comm.VolumePersistentVolumeClaim,
			}
			continue
		}
	}
	return data
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
