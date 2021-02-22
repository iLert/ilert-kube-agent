package watcher

import (
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/iLert/ilert-go"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
)

var podCheckerCron *cron.Cron

func startPodChecker(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, cfg *config.Config) {
	podCheckerCron = cron.New()
	podCheckerCron.AddFunc(fmt.Sprintf("@every %ds", cfg.ResourcesCheckerInterval), func() {
		checkPods(kubeClient, metricsClient, cfg)
	})

	log.Info().Msg("Starting pods checker")
	podCheckerCron.Start()
}

func stopPodMetricsChecker() {
	if podCheckerCron != nil {
		log.Info().Msg("Stopping pods checker")
		podCheckerCron.Stop()
	}
}

func checkPods(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, cfg *config.Config) {
	log.Info().Msg("Checking pods...")

	pods, err := kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
	}

	for _, pod := range pods.Items {
		if cfg.EnableResourcesAlarms {
			podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(pod.GetNamespace()).Get(pod.GetName(), metav1.GetOptions{})
			if err != nil {
				log.Debug().Err(err).Msg("Failed to get pod metrics")
				continue
			}
			podContainers := pod.Spec.Containers
			for _, container := range podContainers {
				metricsContainer := getContainerByName(container.Name, podMetrics.Containers)
				if metricsContainer == nil {
					log.Warn().
						Str("pod", pod.GetName()).
						Str("namespace", pod.GetNamespace()).
						Str("container", container.Name).
						Msg("Could not find container for metrics data")
					continue
				}
				var cpuUsage, memoryUsage int64
				cpuUsage, ok := metricsContainer.Usage.Cpu().AsInt64()
				if !ok {
					cpuUsage = 0
				}
				memoryUsage, ok = metricsContainer.Usage.Memory().AsInt64()
				if !ok {
					memoryUsage = 0
				}
				if cpuUsage > 0 && container.Resources.Limits.Cpu() != nil {
					cpuLimit, ok := container.Resources.Limits.Cpu().AsInt64()
					if ok && cpuLimit > 0 {
						log.Debug().
							Str("pod", pod.GetName()).
							Str("namespace", pod.GetNamespace()).
							Str("container", container.Name).
							Int64("limit", cpuLimit).
							Int64("usage", cpuUsage).
							Msg("Checking CPU limit")
						if cpuUsage >= (int64(cfg.ResourcesThreshold) * (cpuLimit / 100)) {
							incident.CreateEvent(
								cfg.APIKey,
								getPodKey(&pod),
								fmt.Sprintf("Pod %s/%s CPU limit reached > %d%", pod.GetNamespace(), pod.GetName(), cfg.ResourcesThreshold),
								getPodDetailsWithUsageLimit(kubeClient, &pod, fmt.Sprintf("%d CPU", cpuUsage), fmt.Sprintf("%d CPU", cpuLimit)),
								ilert.EventTypes.Alert,
								cfg.ResourcesAlarmIncidentPriority)
						}
					}
				}
				if memoryUsage > 0 && container.Resources.Limits.Memory() != nil {
					memoryLimit, ok := container.Resources.Limits.Memory().AsInt64()
					if ok && memoryLimit > 0 {
						log.Debug().
							Str("pod", pod.GetName()).
							Str("namespace", pod.GetNamespace()).
							Str("container", container.Name).
							Int64("limit", memoryLimit).
							Int64("usage", memoryUsage).
							Msg("Checking memory limit")
						if memoryUsage >= (int64(cfg.ResourcesThreshold) * (memoryLimit / 100)) {
							incident.CreateEvent(
								cfg.APIKey,
								getPodKey(&pod),
								fmt.Sprintf("Pod %s/%s memory limit reached > %d%", pod.GetNamespace(), pod.GetName(), cfg.ResourcesThreshold),
								getPodDetailsWithUsageLimit(kubeClient, &pod, fmt.Sprintf("%d", memoryUsage), fmt.Sprintf("%d", memoryLimit)),
								ilert.EventTypes.Alert,
								cfg.ResourcesAlarmIncidentPriority)
						}
					}
				}
			}
		}
	}
}

func getContainerByName(name string, containers []v1beta1.ContainerMetrics) *v1beta1.ContainerMetrics {
	var container v1beta1.ContainerMetrics
	for _, c := range containers {
		if c.Name == name {
			container = c
			break
		}
	}
	return &container
}
