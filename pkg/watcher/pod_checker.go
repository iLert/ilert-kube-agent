package watcher

import (
	"fmt"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/iLert/ilert-go"
	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
)

var podCheckerCron *cron.Cron

func startPodChecker(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	podCheckerCron = cron.New()
	podCheckerCron.AddFunc(cfg.Settings.CheckInterval, func() {
		checkPods(kubeClient, metricsClient, agentKubeClient, cfg)
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

func checkPods(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	pods, err := kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
	}

	if cfg.Alarms.Pods.Resources.Enabled {
		for _, pod := range pods.Items {
			podKey := getPodKey(&pod)
			incidentRef := incident.GetIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace())

			podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(pod.GetNamespace()).Get(pod.GetName(), metav1.GetOptions{})
			if err != nil {
				log.Debug().Err(err).Msg("Failed to get pod metrics")
				continue
			}

			healthy := true
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
				var memoryUsage int64
				var cpuUsage, cpuLimit float64
				cpuUsageDec := metricsContainer.Usage.Cpu().AsDec().String()
				cpuUsage, err = strconv.ParseFloat(cpuUsageDec, 64)
				if err != nil {
					cpuUsage = 0
				}
				memoryUsage, ok := metricsContainer.Usage.Memory().AsInt64()
				if !ok {
					memoryUsage = 0
				}
				if cpuUsage > 0 && container.Resources.Limits.Cpu() != nil {
					cpuLimitDec := container.Resources.Limits.Cpu().AsDec().String()
					cpuLimit, err = strconv.ParseFloat(cpuLimitDec, 64)
					if err != nil {
						cpuLimit = 0
					}
					if cpuLimit > 0 {
						log.Debug().
							Str("pod", pod.GetName()).
							Str("namespace", pod.GetNamespace()).
							Str("container", container.Name).
							Float64("limit", cpuLimit).
							Float64("usage", cpuUsage).
							Msg("Checking CPU limit")
						if cpuUsage >= (float64(cfg.Alarms.Pods.Resources.Threshold) * (cpuLimit / 100)) {
							healthy = false
							if incidentRef == nil {
								summary := fmt.Sprintf("Pod %s/%s CPU limit reached > %d%%", pod.GetNamespace(), pod.GetName(), cfg.Alarms.Pods.Resources.Threshold)
								details := getPodDetailsWithUsageLimit(kubeClient, &pod, fmt.Sprintf("%.3f CPU", cpuUsage), fmt.Sprintf("%.3f CPU", cpuLimit))
								incidentID := incident.CreateEvent(cfg, getPodMustacheValues(&pod), podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Resources.Priority)
								incident.CreateIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace(), incidentID, summary, details)
							}
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
						if memoryUsage >= (int64(cfg.Alarms.Pods.Resources.Threshold) * (memoryLimit / 100)) {
							healthy = false
							if incidentRef == nil {
								summary := fmt.Sprintf("Pod %s/%s memory limit reached > %d%%", pod.GetNamespace(), pod.GetName(), cfg.Alarms.Pods.Resources.Threshold)
								details := getPodDetailsWithUsageLimit(kubeClient, &pod, humanize.Bytes(uint64(memoryUsage)), humanize.Bytes(uint64(memoryLimit)))
								incidentID := incident.CreateEvent(cfg, getPodMustacheValues(&pod), podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Resources.Priority)
								incident.CreateIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace(), incidentID, summary, details)
							}
						}
					}
				}
			}
			if healthy && incidentRef != nil && incidentRef.Spec.ID > 0 {
				incident.CreateEvent(cfg, getPodMustacheValues(&pod), podKey, fmt.Sprintf("Pod %s/%s recovered", pod.GetNamespace(), pod.GetName()), "", ilert.EventTypes.Resolve, cfg.Alarms.Pods.Resources.Priority)
				incident.DeleteIncidentRef(agentKubeClient, pod.GetName(), pod.GetNamespace())
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
