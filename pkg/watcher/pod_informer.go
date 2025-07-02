package watcher

import (
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/iLert/ilert-kube-agent/pkg/config"
)

var podInformerStopper chan struct{}

func startPodInformer(cfg *config.Config) {
	factory := informers.NewSharedInformerFactory(cfg.KubeClient, 0)
	podInformer := factory.Core().V1().Pods().Informer()
	podInformerStopper = make(chan struct{})
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			pod := newObj.(*api.Pod)
			log.Debug().Interface("pod", pod.GetName()).Msg("Update Pod")
			analyzePodStatus(pod, cfg)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*api.Pod)
			log.Debug().Interface("pod", pod.Name).Msg("Delete Pod")
		},
	})

	log.Info().Msg("Starting pod informer")
	podInformer.Run(podInformerStopper)
}

func stopPodInformer() {
	if podInformerStopper != nil {
		log.Info().Msg("Stopping pod informer")
		close(podInformerStopper)
	}
}
