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

	if cfg.KubeClient == nil || cfg.MetricsClient == nil {
		summary := fmt.Sprintf("Cluster connection is not established: %s", clusterKey)
		if cfg.Alarms.Cluster.Enabled {
			details := getConfigDetails(cfg)
			alert.CreateEvent(cfg, alertKeyInit, summary, details, ilert.EventTypes.Alert, ilert.AlertPriorities.High, labels, nil, nil, nil)
		}
		return errors.New(summary)
	}

	// Client check
	alertKeyClient := fmt.Sprintf("%s-client", clusterKey)

	_, err := cfg.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get nodes from apiserver")
		if cfg.Alarms.Cluster.Enabled {
			summary := fmt.Sprintf("Failed to get nodes from apiserver %s", clusterKey)
			details := getConfigDetails(cfg)
			details += fmt.Sprintf("\n\nError: \n%v", err.Error())
			alert.CreateEvent(cfg, alertKeyClient, summary, details, ilert.EventTypes.Alert, ilert.AlertPriorities.High, labels, nil, nil, nil)
		}
		return err
	}

	// CLuster health check
	alertKeyHealth := fmt.Sprintf("%s-health", clusterKey)
	path := "/healthz"
	content, err := cfg.KubeClient.Discovery().RESTClient().Get().AbsPath(path).DoRaw(context.TODO())
	if err != nil {
		log.Error().Err(err).Msg("Failed to health info from apiserver")
		return err
	}

	contentStr := string(content)
	if contentStr != "ok" {
		summary := fmt.Sprintf("Cluster is not healthy: %s", clusterKey)
		if cfg.Alarms.Cluster.Enabled {
			details := getConfigDetails(cfg)
			alert.CreateEvent(cfg, alertKeyHealth, summary, details, ilert.EventTypes.Alert, ilert.AlertPriorities.High, labels, nil, nil, nil)
		}
		return errors.New(summary)
	}

	return nil
}
