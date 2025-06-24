package watcher

import (
	"context"
	"errors"
	"fmt"

	"github.com/iLert/ilert-go/v3"
	"github.com/iLert/ilert-kube-agent/pkg/alert"
	"github.com/iLert/ilert-kube-agent/pkg/config"
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
	alertKeyInit := fmt.Sprintf("%s-init", clusterKey)
	alertRefInit := alert.GetAlertRef(cfg.AgentKubeClient, alertKeyInit, cfg.Settings.Namespace)

	if cfg.KubeClient == nil || cfg.AgentKubeClient == nil || cfg.MetricsClient == nil {
		summary := fmt.Sprintf("Cluster connection is not established: %s", clusterKey)
		if alertRefInit == nil && cfg.Alarms.Cluster.Enabled {
			details := getConfigDetails(cfg)
			alert.CreateEvent(cfg, nil, alertKeyInit, summary, details, ilert.EventTypes.Alert, ilert.AlertPriorities.High, labels)
		}
		return errors.New(summary)
	}

	if alertRefInit != nil {
		summary := fmt.Sprintf("Cluster connection is established: %s", clusterKey)
		alert.CreateEvent(cfg, nil, alertKeyInit, summary, "", ilert.EventTypes.Resolve, "", labels)
	}

	// Client check
	alertKeyClient := fmt.Sprintf("%s-client", clusterKey)
	alertRefClient := alert.GetAlertRef(cfg.AgentKubeClient, alertKeyClient, cfg.Settings.Namespace)

	_, err := cfg.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get nodes from apiserver")
		if alertRefClient == nil && cfg.Alarms.Cluster.Enabled {
			summary := fmt.Sprintf("Failed to get nodes from apiserver %s", clusterKey)
			details := getConfigDetails(cfg)
			details += fmt.Sprintf("\n\nError: \n%v", err.Error())
			alert.CreateEvent(cfg, nil, alertKeyClient, summary, details, ilert.EventTypes.Alert, ilert.AlertPriorities.High, labels)
		}
		return err
	}

	if alertRefClient != nil {
		summary := fmt.Sprintf("Cluster client is ok: %s", clusterKey)
		alert.CreateEvent(cfg, nil, alertKeyClient, summary, "", ilert.EventTypes.Resolve, "", labels)
	}

	// CLuster health check
	alertKeyHealth := fmt.Sprintf("%s-health", clusterKey)
	alertRefHealth := alert.GetAlertRef(cfg.AgentKubeClient, alertKeyClient, cfg.Settings.Namespace)
	path := "/healthz"
	content, err := cfg.KubeClient.Discovery().RESTClient().Get().AbsPath(path).DoRaw(context.TODO())
	if err != nil {
		log.Error().Err(err).Msg("Failed to health info from apiserver")
		return err
	}

	contentStr := string(content)
	if contentStr != "ok" {
		summary := fmt.Sprintf("Cluster is not healthy: %s", clusterKey)
		if alertRefHealth == nil && cfg.Alarms.Cluster.Enabled {
			details := getConfigDetails(cfg)
			alert.CreateEvent(cfg, nil, alertKeyHealth, summary, details, ilert.EventTypes.Alert, ilert.AlertPriorities.High, labels)
		}
		return errors.New(summary)
	}

	if alertRefHealth != nil {
		summary := fmt.Sprintf("Cluster is healthy: %s", clusterKey)
		alert.CreateEvent(cfg, nil, alertKeyHealth, summary, "", ilert.EventTypes.Resolve, "", labels)
	}

	return nil
}
