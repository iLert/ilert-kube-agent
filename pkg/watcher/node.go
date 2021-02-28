package watcher

import (
	"fmt"

	"github.com/cbroglie/mustache"
	"github.com/iLert/ilert-go"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	api "k8s.io/api/core/v1"
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
