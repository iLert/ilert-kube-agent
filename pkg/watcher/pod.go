package watcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/cbroglie/mustache"
	"github.com/dustin/go-humanize"
	"github.com/iLert/ilert-go/v3"
	"github.com/iLert/ilert-kube-agent/pkg/alert"
	"github.com/iLert/ilert-kube-agent/pkg/commander"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getPodKey(pod *api.Pod) string {
	return fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())
}
func getPodDetailsWithUsageLimit(kubeClient *kubernetes.Clientset, pod *api.Pod, usage string, limit string) string {
	details := fmt.Sprintf("Name: %s\nNamespace: %s",
		pod.GetName(),
		pod.GetNamespace())

	if usage != "" {
		details += fmt.Sprintf("\nUsage: %s", usage)
	}
	if limit != "" {
		details += fmt.Sprintf("\nLimit: %s", limit)
	}
	return details
}

func getPodDetailsWithStatus(kubeClient *kubernetes.Clientset, pod *api.Pod, containerStatus *api.ContainerStatus) string {
	details := fmt.Sprintf("Name: %s\nNamespace: %s",
		pod.GetName(),
		pod.GetNamespace())

	if containerStatus != nil && containerStatus.State.Terminated != nil {
		details += fmt.Sprintf("\nReason: %s\nExit code: %d\nStarted at: %s\nFinished at: %s",
			containerStatus.State.Terminated.Reason,
			containerStatus.State.Terminated.ExitCode,
			containerStatus.State.Terminated.StartedAt,
			containerStatus.State.Terminated.FinishedAt,
		)
	}

	if containerStatus != nil && containerStatus.State.Waiting != nil {
		details += fmt.Sprintf("\nReason: %s\nMessage: %s",
			containerStatus.State.Waiting.Reason,
			containerStatus.State.Waiting.Message,
		)
	}

	podLogs := getPodLogs(kubeClient, pod, containerStatus.Name)
	if podLogs != "" {
		details += fmt.Sprintf("\nLogs:\n%s", podLogs)
	}

	return details
}

func getPodLogs(kubeClient *kubernetes.Clientset, pod *api.Pod, container string) string {
	podLogOpts := api.PodLogOptions{
		TailLines: utils.Int64(50),
		Container: container,
	}

	req := kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return ""
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return ""
	}

	return buf.String()

}

func getPodMustacheValues(pod *api.Pod) map[string]string {
	return map[string]string{
		"pod_name":      pod.GetName(),
		"pod_namespace": pod.GetNamespace(),
	}
}

func getPodLinks(cfg *config.Config, node *api.Pod) []ilert.AlertLink {
	mustacheValues := getPodMustacheValues(node)

	links := make([]ilert.AlertLink, 0)
	for _, link := range cfg.Links.Pods {
		url, err := mustache.Render(link.Href, mustacheValues)
		if err == nil && url != "" {
			links = append(links, ilert.AlertLink{
				Href: url,
				Text: link.Name,
			})
		}
	}
	return links
}

func analyzePodStatus(pod *api.Pod, cfg *config.Config) {
	podKey := getPodKey(pod)
	alertRef := alert.GetAlertRef(cfg.AgentKubeClient, pod.GetName(), pod.GetNamespace())

	labels := getEventLabelsFromPod(pod, cfg.KubeClient)

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil &&
			utils.StringContains(containerTerminatedReasons, containerStatus.State.Terminated.Reason) &&
			cfg.Alarms.Pods.Terminate.Enabled && alertRef == nil {
			summary := fmt.Sprintf("Pod %s/%s terminated - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Terminated.Reason)
			details := getPodDetailsWithStatus(cfg.KubeClient, pod, &containerStatus)
			links := getPodLinks(cfg, pod)
			alert.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Terminate.Priority, labels)
			break
		}

		if containerStatus.State.Waiting != nil &&
			utils.StringContains(containerWaitingReasons, containerStatus.State.Waiting.Reason) &&
			cfg.Alarms.Pods.Waiting.Enabled && alertRef == nil {
			summary := fmt.Sprintf("Pod %s/%s waiting - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Waiting.Reason)
			details := getPodDetailsWithStatus(cfg.KubeClient, pod, &containerStatus)
			links := getPodLinks(cfg, pod)
			alert.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Waiting.Priority, labels)
			break
		}

		if cfg.Alarms.Pods.Restarts.Enabled && containerStatus.RestartCount >= cfg.Alarms.Pods.Restarts.Threshold && alertRef == nil {
			summary := fmt.Sprintf("Pod %s/%s restarts threshold reached: %d", pod.GetNamespace(), pod.GetName(), containerStatus.RestartCount)
			details := getPodDetailsWithStatus(cfg.KubeClient, pod, &containerStatus)
			links := getPodLinks(cfg, pod)
			alert.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Restarts.Priority, labels)
			break
		}
	}
}

func analyzePodResources(pod *api.Pod, cfg *config.Config) error {
	if !cfg.Alarms.Pods.Resources.Enabled {
		return nil
	}

	podKey := getPodKey(pod)
	alertRef := alert.GetAlertRef(cfg.AgentKubeClient, pod.GetName(), pod.GetNamespace())

	podMetrics, err := cfg.MetricsClient.MetricsV1beta1().PodMetricses(pod.GetNamespace()).Get(context.TODO(), pod.GetName(), metav1.GetOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("Failed to get pod metrics")
		return err
	}

	labels := getEventLabelsFromPod(pod, cfg.KubeClient)

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
			return errors.New("Could not find container for metrics data")
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

		if cfg.Alarms.Pods.Resources.CPU.Enabled && cpuUsage > 0 && container.Resources.Limits.Cpu() != nil {
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
				if cpuUsage >= (float64(cfg.Alarms.Pods.Resources.CPU.Threshold) * (cpuLimit / 100)) {
					healthy = false
					if alertRef == nil {
						summary := fmt.Sprintf("Pod %s/%s CPU limit reached > %d%%", pod.GetNamespace(), pod.GetName(), cfg.Alarms.Pods.Resources.CPU.Threshold)
						details := getPodDetailsWithUsageLimit(cfg.KubeClient, pod, fmt.Sprintf("%.3f CPU", cpuUsage), fmt.Sprintf("%.3f CPU", cpuLimit))
						links := getPodLinks(cfg, pod)
						alert.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Resources.CPU.Priority, labels)
					}
				}
			}
		}

		if cfg.Alarms.Pods.Resources.Memory.Enabled && memoryUsage > 0 && container.Resources.Limits.Memory() != nil {
			memoryLimit, ok := container.Resources.Limits.Memory().AsInt64()
			if ok && memoryLimit > 0 {
				log.Debug().
					Str("pod", pod.GetName()).
					Str("namespace", pod.GetNamespace()).
					Str("container", container.Name).
					Int64("limit", memoryLimit).
					Int64("usage", memoryUsage).
					Msg("Checking memory limit")
				if memoryUsage >= (int64(cfg.Alarms.Pods.Resources.Memory.Threshold) * (memoryLimit / 100)) {
					healthy = false
					if alertRef == nil {
						summary := fmt.Sprintf("Pod %s/%s memory limit reached > %d%%", pod.GetNamespace(), pod.GetName(), cfg.Alarms.Pods.Resources.Memory.Threshold)
						details := getPodDetailsWithUsageLimit(cfg.KubeClient, pod, humanize.Bytes(uint64(memoryUsage)), humanize.Bytes(uint64(memoryLimit)))
						links := getPodLinks(cfg, pod)
						alert.CreateEvent(cfg, links, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Resources.Memory.Priority, labels)
					}
				}
			}
		}
	}
	if healthy && alertRef != nil && alertRef.Spec.ID > 0 && alertRef.Spec.Type == "resources" {
		alert.CreateEvent(cfg, nil, podKey, fmt.Sprintf("Pod %s/%s recovered", pod.GetNamespace(), pod.GetName()), "", ilert.EventTypes.Resolve, "", labels)
	}

	return nil
}

func getEventLabelsFromPod(pod *api.Pod, clientset *kubernetes.Clientset) map[string]string {
	podNamespace := pod.GetNamespace()
	podName := pod.GetName()

	labels := map[string]string{
		"namespace":       podNamespace,
		"podName":         podName,
		"resourceVersion": pod.GetResourceVersion(),
		"node":            pod.Spec.NodeName,
		"app":             getLabel(pod, "app"),
		"stage":           getLabel(pod, "stage"),
		"version":         getLabel(pod, "version"),
	}

	workload, err, _ := commander.FindWorkloadByPodName(clientset, podNamespace, podName)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to find workload by pod name")
	} else {
		labels["workloadType"] = string(workload.Type)
		switch workload.Type {
		case commander.WorkloadTypeDeployment:
			labels[string(commander.WorkloadTypeDeployment)] = workload.Name
		case commander.WorkloadTypeStatefulSet:
			labels[string(commander.WorkloadTypeStatefulSet)] = workload.Name
		}
	}

	return labels
}
