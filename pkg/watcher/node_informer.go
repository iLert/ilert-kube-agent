package watcher

import (
	"time"

	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/iLert/ilert-kube-agent/pkg/config"
)

var (
	nodeInformerStopper chan struct{}
	nodeInformer        cache.SharedInformer
)

func startNodeInformer(cfg *config.Config) {
	if sharedFactory == nil {
		sharedFactory = informers.NewSharedInformerFactory(cfg.KubeClient, 15*time.Minute)
	}

	nodeInformer = sharedFactory.Core().V1().Nodes().Informer()
	nodeInformerStopper = make(chan struct{})
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			node := newObj.(*api.Node)
			log.Debug().Interface("node_name", node.GetName()).Msg("Update Node")
			analyzeNodeStatus(node, cfg)
		},
	})

	log.Info().Msg("Starting node informer")
	nodeInformer.Run(nodeInformerStopper)
}

func stopNodeInformer() {
	if nodeInformerStopper != nil {
		log.Info().Msg("Stopping node informer")
		close(nodeInformerStopper)
		nodeInformerStopper = nil
	}
}

func GetNodeInformer() cache.SharedInformer {
	return nodeInformer
}
