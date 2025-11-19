package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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

	return details
}

func getPodLogs(kubeClient *kubernetes.Clientset, pod *api.Pod, container string) []ilert.EventLog {
	podLogOpts := api.PodLogOptions{
		TailLines: utils.Int64(50),
		Container: container,
	}

	req := kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return nil
	}
	defer podLogs.Close()

	var eventLogs []ilert.EventLog
	scanner := bufio.NewScanner(podLogs)

	levelPattern := regexp.MustCompile(`(?i)(ERROR|WARN|WARNING|INFO|DEBUG|FATAL|PANIC|ERR|WRN|INF|SEVERE|TRACE|CRITICAL|CRIT|EMERGENCY|EMERG|CONFIG|FINE|FINER|FINEST)`)
	timestampPattern := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:\d{2})?)`)
	unixTimestampPattern := regexp.MustCompile(`(\d{10,13})`)
	shortDatePattern := regexp.MustCompile(`(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})`)
	monthDayPattern := regexp.MustCompile(`(\w{3} \d{1,2} \d{2}:\d{2}:\d{2})`)

	const maxLogsSize = 24 * 1024 // 24KB

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		timestamp := time.Now().Format(time.RFC3339)

		if matches := timestampPattern.FindStringSubmatch(line); len(matches) > 1 {
			tsStr := matches[1]
			if parsedTime, err := time.Parse(time.RFC3339, tsStr); err == nil {
				timestamp = parsedTime.Format(time.RFC3339)
			} else if parsedTime, err := time.Parse("2006-01-02 15:04:05", tsStr); err == nil {
				timestamp = parsedTime.Format(time.RFC3339)
			} else if parsedTime, err := time.Parse("2006-01-02 15:04:05,000", tsStr); err == nil {
				timestamp = parsedTime.Format(time.RFC3339)
			} else if parsedTime, err := time.Parse("2006-01-02 15:04:05.000", tsStr); err == nil {
				timestamp = parsedTime.Format(time.RFC3339)
			}
		} else if matches := unixTimestampPattern.FindStringSubmatch(line); len(matches) > 1 {
			if unixTs, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				if unixTs > 1e10 {
					unixTs = unixTs / 1000
				}
				if parsedTime := time.Unix(unixTs, 0); !parsedTime.IsZero() {
					timestamp = parsedTime.Format(time.RFC3339)
				}
			}
		} else if matches := shortDatePattern.FindStringSubmatch(line); len(matches) > 1 {
			if parsedTime, err := time.Parse("01/02/2006 15:04:05", matches[1]); err == nil {
				timestamp = parsedTime.Format(time.RFC3339)
			}
		} else if matches := monthDayPattern.FindStringSubmatch(line); len(matches) > 1 {
			if parsedTime, err := time.Parse("Jan 2 15:04:05", matches[1]); err == nil {
				now := time.Now()
				parsedTime = time.Date(now.Year(), parsedTime.Month(), parsedTime.Day(),
					parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, now.Location())
				timestamp = parsedTime.Format(time.RFC3339)
			}
		}

		level := "INFO"
		if matches := levelPattern.FindStringSubmatch(line); len(matches) > 1 {
			detectedLevel := strings.ToUpper(matches[1])
			switch detectedLevel {
			case "ERROR", "ERR", "FATAL", "PANIC", "SEVERE", "CRITICAL", "CRIT", "EMERGENCY", "EMERG":
				level = "ERROR"
			case "WARN", "WARNING", "WRN":
				level = "WARN"
			case "DEBUG", "TRACE", "FINE", "FINER", "FINEST":
				level = "DEBUG"
			case "INFO", "INF", "CONFIG":
				level = "INFO"
			}
		}

		eventLog := ilert.EventLog{
			Timestamp: timestamp,
			Level:     level,
			Body:      line,
		}

		testLogs := append(eventLogs, eventLog)
		if jsonData, err := json.Marshal(testLogs); err == nil {
			if len(jsonData) > maxLogsSize {
				break
			}
		}

		eventLogs = append(eventLogs, eventLog)
	}

	if err := scanner.Err(); err != nil {
		log.Warn().Err(err).Msg("Error reading pod logs")
		return nil
	}

	return eventLogs
}

func getPodCustomDetails(containerStatus *api.ContainerStatus) map[string]interface{} {
	if containerStatus == nil {
		return nil
	}
	customDetails := map[string]interface{}{}
	if containerStatus != nil && containerStatus.State.Waiting != nil {
		customDetails["reason"] = containerStatus.State.Waiting.Reason
		customDetails["message"] = containerStatus.State.Waiting.Message
	}
	if containerStatus != nil && containerStatus.State.Terminated != nil {
		customDetails["reason"] = containerStatus.State.Terminated.Reason
		customDetails["signal"] = containerStatus.State.Terminated.Signal
		customDetails["exit_code"] = containerStatus.State.Terminated.ExitCode
		customDetails["started_at"] = containerStatus.State.Terminated.StartedAt
		customDetails["finished_at"] = containerStatus.State.Terminated.FinishedAt
	}
	if containerStatus.StopSignal != nil {
		customDetails["stop_signal"] = containerStatus.StopSignal
	}
	return customDetails
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

	labels := getEventLabelsFromPod(pod, cfg.KubeClient)

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil &&
			utils.StringContains(containerTerminatedReasons, containerStatus.State.Terminated.Reason) &&
			cfg.Alarms.Pods.Terminate.Enabled {
			if utils.StringContains(cfg.Alarms.Pods.Terminate.ExcludedReasons, containerStatus.State.Terminated.Reason) {
				log.Debug().
					Str("pod", pod.GetName()).
					Str("namespace", pod.GetNamespace()).
					Str("reason", containerStatus.State.Terminated.Reason).
					Msg("Skipping alert for excluded termination reason")
				continue
			}
			summary := fmt.Sprintf("Pod %s/%s terminated - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Terminated.Reason)
			details := getPodDetailsWithStatus(cfg.KubeClient, pod, &containerStatus)
			links := getPodLinks(cfg, pod)
			podLogs := getPodLogs(cfg.KubeClient, pod, containerStatus.Name)
			customDetails := getPodCustomDetails(&containerStatus)
			alert.CreateEvent(cfg, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Terminate.Priority, labels, links, podLogs, customDetails)
			break
		}

		if containerStatus.State.Waiting != nil &&
			utils.StringContains(containerWaitingReasons, containerStatus.State.Waiting.Reason) &&
			cfg.Alarms.Pods.Waiting.Enabled {
			summary := fmt.Sprintf("Pod %s/%s waiting - %s", pod.GetNamespace(), pod.GetName(), containerStatus.State.Waiting.Reason)
			details := getPodDetailsWithStatus(cfg.KubeClient, pod, &containerStatus)
			links := getPodLinks(cfg, pod)
			podLogs := getPodLogs(cfg.KubeClient, pod, containerStatus.Name)
			customDetails := getPodCustomDetails(&containerStatus)
			alert.CreateEvent(cfg, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Waiting.Priority, labels, links, podLogs, customDetails)
			break
		}

		if cfg.Alarms.Pods.Restarts.Enabled && containerStatus.RestartCount >= cfg.Alarms.Pods.Restarts.Threshold {
			summary := fmt.Sprintf("Pod %s/%s restarts threshold reached: %d", pod.GetNamespace(), pod.GetName(), containerStatus.RestartCount)
			details := getPodDetailsWithStatus(cfg.KubeClient, pod, &containerStatus)
			links := getPodLinks(cfg, pod)
			podLogs := getPodLogs(cfg.KubeClient, pod, containerStatus.Name)
			customDetails := getPodCustomDetails(&containerStatus)
			alert.CreateEvent(cfg, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Restarts.Priority, labels, links, podLogs, customDetails)
			break
		}
	}
}

func analyzePodResources(pod *api.Pod, cfg *config.Config) error {
	if !cfg.Alarms.Pods.Resources.Enabled {
		return nil
	}

	podKey := getPodKey(pod)

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
					summary := fmt.Sprintf("Pod %s/%s CPU limit reached > %d%%", pod.GetNamespace(), pod.GetName(), cfg.Alarms.Pods.Resources.CPU.Threshold)
					details := getPodDetailsWithUsageLimit(cfg.KubeClient, pod, fmt.Sprintf("%.3f CPU", cpuUsage), fmt.Sprintf("%.3f CPU", cpuLimit))
					links := getPodLinks(cfg, pod)
					alert.CreateEvent(cfg, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Resources.CPU.Priority, labels, links, nil, nil)
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
					summary := fmt.Sprintf("Pod %s/%s memory limit reached > %d%%", pod.GetNamespace(), pod.GetName(), cfg.Alarms.Pods.Resources.Memory.Threshold)
					details := getPodDetailsWithUsageLimit(cfg.KubeClient, pod, humanize.Bytes(uint64(memoryUsage)), humanize.Bytes(uint64(memoryLimit)))
					links := getPodLinks(cfg, pod)
					alert.CreateEvent(cfg, podKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Pods.Resources.Memory.Priority, labels, links, nil, nil)
				}
			}
		}
	}
	if healthy {
		alert.CreateEvent(cfg, podKey, fmt.Sprintf("Pod %s/%s recovered", pod.GetNamespace(), pod.GetName()), "", ilert.EventTypes.Resolve, "", labels, nil, nil, nil)
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
		case commander.WorkloadTypeDaemonSet:
			labels[string(commander.WorkloadTypeDaemonSet)] = workload.Name
		}
	}

	return labels
}
