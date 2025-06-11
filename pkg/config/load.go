package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/iLert/ilert-kube-agent/pkg/utils"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var runOnce bool

func (cfg *Config) SetRunOnce(r bool) {
	runOnce = r
}

func (cfg *Config) GetRunOnce() bool {
	return runOnce
}

// SetConfigFile set config file path and read it into struct
func (cfg *Config) SetConfigFile(cfgFile string) {
	if cfgFile != "" {
		log.Debug().Str("file", cfgFile).Msg("Reading config file")
		viper.SetConfigFile(cfgFile)
		err := viper.ReadInConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("Unable to read config")
		}
	}
}

// Load reads config from file, envs or flags
func (cfg *Config) Load() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.SetEnvPrefix("ilert")
	viper.AutomaticEnv()

	err := viper.Unmarshal(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to decode config")
	}

	if cfg.Links.Pods == nil {
		cfg.Links.Pods = make([]ConfigLinksSetting, 0)
	}
	if cfg.Links.Nodes == nil {
		cfg.Links.Nodes = make([]ConfigLinksSetting, 0)
	}

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], "ILERT_LINKS_PODS_") {
			link := strings.ReplaceAll(pair[0], "ILERT_LINKS_PODS_", "")
			cfg.Links.Pods = append(cfg.Links.Pods, ConfigLinksSetting{
				Name: strings.Title(strings.ToLower(strings.ReplaceAll(link, "_", " "))),
				Href: pair[1],
			})
		}

		if strings.HasPrefix(pair[0], "ILERT_LINKS_NODES_") {
			cfg.Links.Nodes = append(cfg.Links.Nodes, ConfigLinksSetting{
				Name: strings.Title(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(pair[0], "ILERT_LINKS_NODES_", ""), "_", " "))),
				Href: pair[1],
			})
		}
	}

	ilertAPIKeyEnv := utils.GetEnv("ILERT_API_KEY", "")
	if ilertAPIKeyEnv != "" {
		cfg.Settings.APIKey = ilertAPIKeyEnv
	}

	namespaceEnv := utils.GetEnv("NAMESPACE", "")
	if namespaceEnv != "" {
		cfg.Settings.Namespace = namespaceEnv
	}

	logLevelEnv := utils.GetEnv("LOG_LEVEL", "")
	if logLevelEnv != "" {
		cfg.Settings.Log.Level = logLevelEnv
	}

	httpAuthorizationKeyEnv := utils.GetEnv("HTTP_AUTHORIZATION_KEY", "")
	if httpAuthorizationKeyEnv != "" {
		cfg.Settings.HttpAuthorizationKey = httpAuthorizationKeyEnv
	}

	cfg.Validate()

	// Base64 encoded kubeconfig
	encodedKubeConfig := utils.GetEnv("KUBECONFIG", "")
	if encodedKubeConfig != "" {
		kubeConfigBytes, err := base64.StdEncoding.DecodeString(encodedKubeConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to decode kubeconfig from base64")
		}

		kubeConfigPath := "/tmp/kubeconfig"
		f, err := os.Create(kubeConfigPath)
		if err != nil {
			log.Fatal().Err(err).Msg(fmt.Sprintf("Failed to create %s file", kubeConfigPath))
		}

		_, err = f.Write(kubeConfigBytes)
		if err != nil {
			f.Close()
			log.Fatal().Err(err).Msg(fmt.Sprintf("Failed to write %s file", kubeConfigPath))
		}

		f.Close()
		cfg.Settings.KubeConfig = kubeConfigPath
	}

	cfg.initializeClients()
}
