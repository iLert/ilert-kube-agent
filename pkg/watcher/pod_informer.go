package watcher

import (
	"fmt"

	"github.com/iLert/ilert-go"
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
)

var podInformerStopper chan struct{}

func startPodInformer(kubeClient *kubernetes.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := factory.Core().V1().Pods().Informer()
	podInformerStopper = make(chan struct{})
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			pod := newObj.(*api.Pod)
			log.Debug().Interface("pod", pod.GetName()).Msg("Update Pod")
			podKey := getPodKey(pod)
			incidentRef := incident.GetIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace())

			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Terminated != nil &&
					utils.StringContains(containerTerminatedReasons, containerStatus.State.Terminated.Reason) &&
					cfg.Alarms.Pods.Terminate.Enabled && incidentRef == nil {
					summary := fmt.Sprintf("Pod %s/%s terminated - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Terminated.Reason)
					details := getPodDetailsWithStatus(kubeClient, pod, &containerStatus)
					links := getPodLinks(cfg, pod)
					incidentID := incident.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Terminate.Priority)
					incident.CreateIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace(), incidentID, summary, details)
					break
				}

				if containerStatus.State.Waiting != nil &&
					utils.StringContains(containerWaitingReasons, containerStatus.State.Waiting.Reason) &&
					cfg.Alarms.Pods.Waiting.Enabled && incidentRef == nil {
					summary := fmt.Sprintf("Pod %s/%s waiting - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Waiting.Reason)
					details := getPodDetailsWithStatus(kubeClient, pod, &containerStatus)
					links := getPodLinks(cfg, pod)
					incidentID := incident.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Waiting.Priority)
					incident.CreateIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace(), incidentID, summary, details)
					break
				}

				if cfg.Alarms.Pods.Restarts.Enabled && containerStatus.RestartCount >= cfg.Alarms.Pods.Restarts.Threshold && incidentRef == nil {
					summary := fmt.Sprintf("Pod %s/%s restarts threshold reached: %d", pod.GetNamespace(), pod.GetName(), containerStatus.RestartCount)
					details := getPodDetailsWithStatus(kubeClient, pod, &containerStatus)
					links := getPodLinks(cfg, pod)
					incidentID := incident.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Restarts.Priority)
					incident.CreateIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace(), incidentID, summary, details)
					break
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*api.Pod)
			log.Debug().Interface("pod", pod.Name).Msg("Delete Pod")
			incident.DeleteIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace())
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
