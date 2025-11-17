package config

import (
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
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
	// Create a sanitized copy of Settings with sensitive values masked
	sanitizedSettings := ConfigSettings{
		APIKey:               maskIfNotEmpty(cfg.Settings.APIKey),
		HttpAuthorizationKey: maskIfNotEmpty(cfg.Settings.HttpAuthorizationKey),
		KubeConfig:           cfg.Settings.KubeConfig,
		Master:               cfg.Settings.Master,
		Insecure:             cfg.Settings.Insecure,
		Namespace:            cfg.Settings.Namespace,
		Port:                 cfg.Settings.Port,
		Log:                  cfg.Settings.Log,
		ElectionID:           cfg.Settings.ElectionID,
		CheckInterval:        cfg.Settings.CheckInterval,
	}

	log.Info().Interface("config", struct {
		Settings ConfigSettings
		Alarms   ConfigAlarms
		Links    ConfigLinks
	}{
		Settings: sanitizedSettings,
		Alarms:   cfg.Alarms,
		Links:    cfg.Links,
	}).Msg("Starting with config")
}

// maskIfNotEmpty returns "(sensitive value)" if the string is not empty, otherwise returns the original string
func maskIfNotEmpty(s string) string {
	if s != "" {
		return "(sensitive value)"
	}
	return s
}
