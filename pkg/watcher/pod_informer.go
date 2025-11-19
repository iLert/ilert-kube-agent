package watcher

import (
	"time"

	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/memory"
)

var (
	podInformerStopper chan struct{}
	podInformer        cache.SharedInformer
)

func startPodInformer(cfg *config.Config) {
	if sharedFactory == nil {
		sharedFactory = informers.NewSharedInformerFactory(cfg.KubeClient, 15*time.Minute)
	}

	podInformer = sharedFactory.Core().V1().Pods().Informer()
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

	defer memory.RecoverPanic("pod-informer")
	podInformer.Run(podInformerStopper)
}

func stopPodInformer() {
	if podInformerStopper != nil {
		log.Info().Msg("Stopping pod informer")
		close(podInformerStopper)
		podInformerStopper = nil
	}
}

func GetPodInformer() cache.SharedInformer {
	return podInformer
}
