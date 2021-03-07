package config

import (
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
)

// SetKubeConfig override default kube config
func (cfg *Config) SetKubeConfig(config *rest.Config) {
	cfg.KubeConfig = config
}

func (cfg *Config) initializeClients() {
	if cfg.KubeConfig == nil {
		config, err := clientcmd.BuildConfigFromFlags(cfg.Settings.Master, cfg.Settings.KubeConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to build kubeconfig")
		} else {
			cfg.KubeConfig = config
		}

		if cfg.Settings.Insecure {
			config.Insecure = true
		}
	}

	if cfg.KubeClient == nil {
		kubeClient, err := kubernetes.NewForConfig(cfg.KubeConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create kube client")
		} else {
			cfg.KubeClient = kubeClient
		}
	}

	if cfg.AgentKubeClient == nil {
		agentKubeClient, err := agentclientset.NewForConfig(cfg.KubeConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create kube agent client")
		} else {
			cfg.AgentKubeClient = agentKubeClient
		}
	}

	if cfg.MetricsClient == nil {
		metricsClient, err := metrics.NewForConfig(cfg.KubeConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create metrics client")
		} else {
			cfg.MetricsClient = metricsClient
		}
	}
}

func (cfg *Config) Print() {
	log.Info().Interface("config", struct {
		Settings ConfigSettings
		Alarms   ConfigAlarms
		Links    ConfigLinks
	}{
		Settings: cfg.Settings,
		Alarms:   cfg.Alarms,
		Links:    cfg.Links,
	}).Msg("Starting with config")
}
