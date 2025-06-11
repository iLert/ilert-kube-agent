package watcher

import (
	"fmt"
	"strconv"

	"github.com/cbroglie/mustache"
	"github.com/dustin/go-humanize"
	"github.com/iLert/ilert-go"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
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
		"node_name":    node.GetName(),
		"cluster_name": node.GetClusterName(),
	}
}

func getNodeLinks(cfg *config.Config, node *api.Node) []ilert.IncidentLink {
	mustacheValues := getNodeMustacheValues(node)

	links := make([]ilert.IncidentLink, 0)
	for _, link := range cfg.Links.Nodes {
		url, err := mustache.Render(link.Href, mustacheValues)
		if err == nil && url != "" {
			links = append(links, ilert.IncidentLink{
				Href: url,
				Text: link.Name,
			})
		}
	}
	return links
}

func analyzeNodeStatus(node *api.Node, cfg *config.Config) {
	nodeKey := getNodeKey(node)
	incidentRef := incident.GetIncidentRef(cfg.AgentKubeClient, nodeKey, cfg.Settings.Namespace)

	labels := map[string]string{
		"namespace":       node.GetNamespace(),
		"nodeName":        node.GetName(),
		"resourceVersion": node.GetResourceVersion(),
		"clusterName":     node.GetClusterName(),
	}

	if node.Status.Phase == api.NodeTerminated && cfg.Alarms.Nodes.Terminate.Enabled && incidentRef == nil {
		summary := fmt.Sprintf("Node %s terminated", node.GetName())
		details := getNodeDetails(cfg.KubeClient, node)
		links := getNodeLinks(cfg, node)
		incident.CreateEvent(cfg, links, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Nodes.Terminate.Priority, labels)
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
		"clusterName":     node.GetClusterName(),
	}
	nodeKey := getNodeKey(node)
	incidentRef := incident.GetIncidentRef(cfg.AgentKubeClient, node.GetName(), cfg.Settings.Namespace)

	nodeMetrics, err := cfg.MetricsClient.MetricsV1beta1().NodeMetricses().Get(node.GetName(), metav1.GetOptions{})
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
				if incidentRef == nil {
					summary := fmt.Sprintf("Node %s CPU limit reached > %d%%", node.GetName(), cfg.Alarms.Nodes.Resources.CPU.Threshold)
					details := getNodeDetailsWithUsageLimit(cfg.KubeClient, node, fmt.Sprintf("%.3f CPU", cpuUsage), fmt.Sprintf("%.3f CPU", cpuLimit))
					links := getNodeLinks(cfg, node)
					incident.CreateEvent(cfg, links, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Nodes.Resources.CPU.Priority, labels)
				}
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
				if incidentRef == nil {
					summary := fmt.Sprintf("Node %s memory limit reached > %d%%", node.GetName(), cfg.Alarms.Nodes.Resources.Memory.Threshold)
					details := getNodeDetailsWithUsageLimit(cfg.KubeClient, node, humanize.Bytes(uint64(memoryUsage)), humanize.Bytes(uint64(memoryLimit)))
					links := getNodeLinks(cfg, node)
					incident.CreateEvent(cfg, links, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.Alarms.Nodes.Resources.Memory.Priority, labels)
				}
			}
		}
	}

	if healthy && incidentRef != nil && incidentRef.Spec.ID > 0 && incidentRef.Spec.Type == "resources" {
		incident.CreateEvent(cfg, nil, nodeKey, fmt.Sprintf("Node %s recovered", node.GetName()), "", ilert.EventTypes.Resolve, "", labels)
	}
	return nil
}
