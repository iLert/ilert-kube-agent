package watcher

import (
	"errors"
	"fmt"

	"github.com/iLert/ilert-go"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getClusterKey(cfg *config.Config) string {
	return fmt.Sprintf("%s/%s", cfg.Settings.Namespace, cfg.Settings.ElectionID)
}

func getConfigDetails(cfg *config.Config) string {
	details := fmt.Sprintf("Master: %s\nKubeConfig: %s\nElectionID: %s\nNamespace: %s\nInsecure: %v",
		cfg.Settings.Master,
		cfg.Settings.KubeConfig,
		cfg.Settings.ElectionID,
		cfg.Settings.Namespace,
		cfg.Settings.Insecure)

	return details
}

func analyzeClusterStatus(cfg *config.Config) error {
	clusterKey := getClusterKey(cfg)

	labels := map[string]string{
		"cluster": clusterKey,
	}

	// Init check
	incidentKeyInit := fmt.Sprintf("%s-init", clusterKey)
	incidentRefInit := incident.GetIncidentRef(cfg.AgentKubeClient, incidentKeyInit, cfg.Settings.Namespace)

	if cfg.KubeClient == nil || cfg.AgentKubeClient == nil || cfg.MetricsClient == nil {
		summary := fmt.Sprintf("Cluster connection is not established: %s", clusterKey)
		if incidentRefInit == nil && cfg.Alarms.Cluster.Enabled {
			details := getConfigDetails(cfg)
			incident.CreateEvent(cfg, nil, incidentKeyInit, summary, details, ilert.EventTypes.Alert, ilert.IncidentPriorities.High, labels)
		}
		return errors.New(summary)
	}

	if incidentRefInit != nil {
		summary := fmt.Sprintf("Cluster connection is established: %s", clusterKey)
		incident.CreateEvent(cfg, nil, incidentKeyInit, summary, "", ilert.EventTypes.Resolve, "", labels)
	}

	// Client check
	incidentKeyClient := fmt.Sprintf("%s-client", clusterKey)
	incidentRefClient := incident.GetIncidentRef(cfg.AgentKubeClient, incidentKeyClient, cfg.Settings.Namespace)

	_, err := cfg.KubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get nodes from apiserver")
		if incidentRefClient == nil && cfg.Alarms.Cluster.Enabled {
			summary := fmt.Sprintf("Failed to get nodes from apiserver %s", clusterKey)
			details := getConfigDetails(cfg)
			details += fmt.Sprintf("\n\nError: \n%v", err.Error())
			incident.CreateEvent(cfg, nil, incidentKeyClient, summary, details, ilert.EventTypes.Alert, ilert.IncidentPriorities.High, labels)
		}
		return err
	}

	if incidentRefClient != nil {
		summary := fmt.Sprintf("Cluster client is ok: %s", clusterKey)
		incident.CreateEvent(cfg, nil, incidentKeyClient, summary, "", ilert.EventTypes.Resolve, "", labels)
	}

	// CLuster health check
	incidentKeyHealth := fmt.Sprintf("%s-health", clusterKey)
	incidentRefHealth := incident.GetIncidentRef(cfg.AgentKubeClient, incidentKeyClient, cfg.Settings.Namespace)
	path := "/healthz"
	content, err := cfg.KubeClient.Discovery().RESTClient().Get().AbsPath(path).DoRaw()
	if err != nil {
		log.Error().Err(err).Msg("Failed to health info from apiserver")
		return err
	}

	contentStr := string(content)
	if contentStr != "ok" {
		summary := fmt.Sprintf("Cluster is not healthy: %s", clusterKey)
		if incidentRefHealth == nil && cfg.Alarms.Cluster.Enabled {
			details := getConfigDetails(cfg)
			incident.CreateEvent(cfg, nil, incidentKeyHealth, summary, details, ilert.EventTypes.Alert, ilert.IncidentPriorities.High, labels)
		}
		return errors.New(summary)
	}

	if incidentRefHealth != nil {
		summary := fmt.Sprintf("Cluster is healthy: %s", clusterKey)
		incident.CreateEvent(cfg, nil, incidentKeyHealth, summary, "", ilert.EventTypes.Resolve, "", labels)
	}

	return nil
}
