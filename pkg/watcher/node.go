package watcher

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cbroglie/mustache"
	"github.com/dustin/go-humanize"
	"github.com/iLert/ilert-go/v3"
	"github.com/iLert/ilert-kube-agent/pkg/alert"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getNodeKey(node *api.Node) string {
	return fmt.Sprintf("%s", node.GetName())
}

func getNodeDetails(kubeClient *kubernetes.Clientset, node *api.Node) string {
	details := fmt.Sprintf("Name: %s\nArchitecture: %s\nOS image: %s\nOperating system: %s\nKernel version: %s\nContainer runtime version: %s\nKubelet version: %s",
		node.GetName(),
		node.Status.NodeInfo.Architecture,
		node.Status.NodeInfo.OSImage,
		node.Status.NodeInfo.OperatingSystem,
		node.Status.NodeInfo.KernelVersion,
		node.Status.NodeInfo.ContainerRuntimeVersion,
		node.Status.NodeInfo.KubeletVersion)

	return details
}

func getNodeDetailsWithUsageLimit(kubeClient *kubernetes.Clientset, node *api.Node, usage string, limit string) string {
	details := getNodeDetails(kubeClient, node)

	if usage != "" {
		details += fmt.Sprintf("\nUsage: %s", usage)
	}
	if limit != "" {
		details += fmt.Sprintf("\nLimit: %s", limit)
	}
	return details
}

func getNodeMustacheValues(node *api.Node) map[string]string {
	return map[string]string{
		"node_name": node.GetName(),
	}
}

func getNodeLinks(cfg *config.Config, node *api.Node) []ilert.AlertLink {
	mustacheValues := getNodeMustacheValues(node)

	links := make([]ilert.AlertLink, 0)
	for _, link := range cfg.Links.Nodes {
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

func analyzeNodeStatus(node *api.Node, cfg *config.Config) {
	nodeKey := getNodeKey(node)

	labels := map[string]string{
		"namespace":       node.GetNamespace(),
		"nodeName":        node.GetName(),
		"resourceVersion": node.GetResourceVersion(),
	}

	if node.Status.Phase == api.NodeTerminated && cfg.Alarms.Nodes.Terminate.Enabled {
		summary := fmt.Sprintf("Node %s terminated", node.GetName())
		details := getNodeDetails(cfg.KubeClient, node)
		links := getNodeLinks(cfg, node)
		alert.CreateEvent(cfg, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Nodes.Terminate.Priority, labels, links, nil)
	}
}

func analyzeNodeResources(node *api.Node, cfg *config.Config) error {
	if !cfg.Alarms.Nodes.Resources.Enabled {
		return nil
	}

	labels := map[string]string{
		"namespace":       node.GetNamespace(),
		"nodeName":        node.GetName(),
		"resourceVersion": node.GetResourceVersion(),
	}
	nodeKey := getNodeKey(node)

	nodeMetrics, err := cfg.MetricsClient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), node.GetName(), metav1.GetOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("Failed to get node metrics")
		return err
	}

	healthy := true
	var memoryUsage int64
	var cpuUsage, cpuLimit float64
	cpuUsageDec := nodeMetrics.Usage.Cpu().AsDec().String()
	cpuUsage, err = strconv.ParseFloat(cpuUsageDec, 64)
	if err != nil {
		cpuUsage = 0
	}
	memoryUsage, ok := nodeMetrics.Usage.Memory().AsInt64()
	if !ok {
		memoryUsage = 0
	}

	if cfg.Alarms.Nodes.Resources.CPU.Enabled {
		cpuLimitDec := node.Status.Capacity.Cpu().AsDec().String()
		cpuLimit, err = strconv.ParseFloat(cpuLimitDec, 64)
		if err != nil {
			cpuLimit = 0
		}
		if ok && cpuLimit > 0 && cpuUsage > 0 {
			log.Debug().
				Str("node", node.GetName()).
				Float64("limit", cpuLimit).
				Float64("usage", cpuUsage).
				Msg("Checking CPU limit")
			if cpuUsage >= (float64(cfg.Alarms.Nodes.Resources.CPU.Threshold) * (cpuLimit / 100)) {
				healthy = false
				summary := fmt.Sprintf("Node %s CPU limit reached > %d%%", node.GetName(), cfg.Alarms.Nodes.Resources.CPU.Threshold)
				details := getNodeDetailsWithUsageLimit(cfg.KubeClient, node, fmt.Sprintf("%.3f CPU", cpuUsage), fmt.Sprintf("%.3f CPU", cpuLimit))
				links := getNodeLinks(cfg, node)
				alert.CreateEvent(cfg, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Nodes.Resources.CPU.Priority, labels, links, nil)
			}
		}
	}

	if cfg.Alarms.Nodes.Resources.Memory.Enabled {
		memoryLimit, ok := node.Status.Capacity.Memory().AsInt64()
		if ok && memoryLimit > 0 && memoryUsage > 0 {
			log.Debug().
				Str("node", node.GetName()).
				Int64("limit", memoryLimit).
				Int64("usage", memoryUsage).
				Msg("Checking memory limit")
			if memoryUsage >= (int64(cfg.Alarms.Nodes.Resources.Memory.Threshold) * (memoryLimit / 100)) {
				healthy = false
				summary := fmt.Sprintf("Node %s memory limit reached > %d%%", node.GetName(), cfg.Alarms.Nodes.Resources.Memory.Threshold)
				details := getNodeDetailsWithUsageLimit(cfg.KubeClient, node, humanize.Bytes(uint64(memoryUsage)), humanize.Bytes(uint64(memoryLimit)))
				links := getNodeLinks(cfg, node)
				alert.CreateEvent(cfg, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Nodes.Resources.Memory.Priority, labels, links, nil)
			}
		}
	}

	if healthy {
		alert.CreateEvent(cfg, nodeKey, fmt.Sprintf("Node %s recovered", node.GetName()), "", ilert.EventTypes.Resolve, "", labels, nil, nil)
	}
	return nil
}
