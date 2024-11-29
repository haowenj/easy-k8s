package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/cache"

	"easy-k8s/pkg/k8s/informerfactory"
)

type ApiServer struct {
	nodeInformer cache.SharedIndexInformer
}

func (s *ApiServer) Engine() *gin.Engine {
	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "has been successfully run"})
	})

	node := NodeLogic{NodeInformer: s.nodeInformer}
	engine.GET("/nodeList", node.GetNodeList)
	return engine
}

func (s *ApiServer) RunInformerFactory(factory *informerfactory.InformerFactory, ctx context.Context) {
	s.nodeInformer = factory.Node()

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
}
