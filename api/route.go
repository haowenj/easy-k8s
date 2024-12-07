package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"easy-k8s/pkg/k8s/informerfactory"
)

type ApiServer struct {
	Log           logr.Logger
	DynamicClient dynamic.Interface
	nodeInformer  cache.SharedIndexInformer
	podInformer   cache.SharedIndexInformer
}

func (s *ApiServer) Engine() *gin.Engine {
	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "has been successfully run"})
	})

	node := NewNodeLogic(s.Log, s.DynamicClient, s.nodeInformer, s.podInformer)
	engine.GET("/getConf", node.GetDisplayFileds)
	engine.POST("/setConf", node.SetDisplayFileds)
	engine.GET("/nodeList", node.GetNodeList)
	engine.GET("/nodeLabels/:node", node.NodeLabels)
	engine.POST("/nodeLabels/:node", node.NodeLabelPatch)
	engine.GET("/nodeResource/:node", node.NodeResource)
	engine.GET("/nodePodList/:node", node.NodePodList)

	pod := NewPodLogic(s.Log, s.DynamicClient, s.nodeInformer, s.podInformer)
	engine.GET("/podResourceInfo/:ns/:name", pod.PodResourceInfo)
	engine.GET("/podLabels/:ns/:name", pod.PodLabels)
	return engine
}

func (s *ApiServer) RunInformerFactory(factory *informerfactory.InformerFactory, ctx context.Context) {
	s.nodeInformer = factory.Node()
	s.podInformer = factory.Pod()

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
}
