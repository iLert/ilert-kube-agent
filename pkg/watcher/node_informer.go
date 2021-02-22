package watcher

import (
	"fmt"

	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/iLert/ilert-go"
	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
)

var nodeInformerStopper chan struct{}

func startNodeInformer(kubeClient *kubernetes.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := factory.Core().V1().Nodes().Informer()
	nodeInformerStopper = make(chan struct{})
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			node := newObj.(*api.Node)
			nodeKey := getNodeKey(node)

			incidentRef := incident.GetIncidentRef(agentKubeClient, nodeKey, "kube-system")
			log.Debug().Interface("node", node).Msg("Update Node")

			if node.Status.Phase == api.NodeTerminated && incidentRef == nil {
				incidentID := incident.CreateEvent(
					cfg.APIKey,
					nodeKey,
					fmt.Sprintf("Node %s terminated", node.GetName()),
					getNodeDetails(kubeClient, node),
					ilert.EventTypes.Alert,
					cfg.NodeAlarmIncidentPriority)
				incident.CreateIncidentRef(agentKubeClient, node.GetName(), "kube-system", incidentID)
			}
		},
	})

	log.Info().Msg("Starting node informer")
	nodeInformer.Run(nodeInformerStopper)
}

func stopNodeInformer() {
	if nodeInformerStopper != nil {
		log.Info().Msg("Stopping node informer")
		close(nodeInformerStopper)
	}
}
