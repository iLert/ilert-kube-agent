package watcher

import (
	"fmt"

	"github.com/iLert/ilert-go"
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
)

var podInformerStopper chan struct{}

func startPodInformer(kubeClient *kubernetes.Clientset, cfg *config.Config) {
	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := factory.Core().V1().Pods().Informer()
	podInformerStopper = make(chan struct{})
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			pod := newObj.(*api.Pod)
			log.Info().Interface("pod", pod.Name).Msg("Update Pod")

			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Terminated != nil &&
					utils.StringContains(containerTerminatedReasons, containerStatus.State.Terminated.Reason) &&
					cfg.EnablePodTerminateAlarms {
					incident.CreateEvent(
						cfg.APIKey,
						getPodKey(pod),
						fmt.Sprintf("Pod %s/%s terminated - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Terminated.Reason),
						getPodDetailsWithStatus(kubeClient, pod, &containerStatus),
						ilert.EventTypes.Alert,
						cfg.PodAlarmIncidentPriority)
					break
				}

				if containerStatus.State.Waiting != nil &&
					utils.StringContains(containerWaitingReasons, containerStatus.State.Waiting.Reason) &&
					cfg.EnablePodWaitingAlarms {
					incident.CreateEvent(
						cfg.APIKey,
						getPodKey(pod),
						fmt.Sprintf("Pod %s/%s waiting - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Waiting.Reason),
						getPodDetailsWithStatus(kubeClient, pod, &containerStatus),
						ilert.EventTypes.Alert,
						cfg.PodAlarmIncidentPriority)
					break
				}

				if cfg.EnablePodRestartsAlarms && containerStatus.RestartCount >= cfg.PodRestartThreshold {
					incident.CreateEvent(
						cfg.APIKey,
						getPodKey(pod),
						fmt.Sprintf("Pod %s/%s restarts threshold reached: %d", pod.GetNamespace(), pod.GetName(), containerStatus.RestartCount),
						getPodDetailsWithStatus(kubeClient, pod, &containerStatus),
						ilert.EventTypes.Alert,
						cfg.PodRestartsAlarmIncidentPriority)
				}
			}
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
