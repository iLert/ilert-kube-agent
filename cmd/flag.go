package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	shared "github.com/iLert/ilert-kube-agent"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/logger"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
)

var (
	help    bool
	version bool
	cfgFile string
)

func parseAndValidateFlags() *config.Config {

	flag.BoolVar(&help, "help", false, "Print this help.")
	flag.BoolVar(&version, "version", false, "Print version.")
	flag.StringVar(&cfgFile, "config", "", "Config file")

	flag.String("settings.kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.String("settings.master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.String("settings.namespace", "kube-system", "Namespace in which agent run.")
	flag.String("settings.log.level", "info", "Log level (debug, info, warn, error, fatal).")
	flag.Bool("settings.log.json", false, "Enable json format log")
	flag.String("settings.electionID", "ilert-kube-agent", "The lease lock resource name")
	flag.Int("settings.port", 9092, "The metrics server port")
	flag.String("settings.apiKey", "", "(REQUIRED) The iLert alert source api key")
	flag.String("settings.checkInterval", "15s", "The evaluation check interval e.g. resources check")

	flag.Bool("alarms.pods.enabled", true, "Enable pod alarms")
	flag.Bool("alarms.pods.terminate.enabled", true, "Enable pod terminate alarms")
	flag.String("alarms.pods.terminate.priority", "HIGH", "The pod terminate alarm incident priority")
	flag.Bool("alarms.pods.waiting.enabled", true, "Enable pod waiting alarms")
	flag.String("alarms.pods.waiting.priority", "LOW", "The pod waiting alarm incident priority")
	flag.Bool("alarms.pods.restarts.enabled", true, "Enable pod restarts alarms")
	flag.String("alarms.pods.restarts.priority", "LOW", "The pod waiting alarm incident priority")
	flag.Int("alarms.pods.restarts.threshold", 10, "Pod restart threshold to alarm")
	flag.Bool("alarms.pods.resources.enabled", true, "Enable pod resources alarms")
	flag.String("alarms.pods.resources.priority", "LOW", "The pod resources alarm incident priority")
	flag.Int("alarms.pods.resources.threshold", 90, "The pod resources percentage threshold from 1 to 100")

	flag.Bool("alarms.nodes.enabled", true, "Enable node alarms")
	flag.Bool("alarms.nodes.terminate.enabled", true, "Enable node terminate alarms")
	flag.String("alarms.nodes.terminate.priority", "HIGH", "The node terminate alarm incident priority")
	flag.Bool("alarms.nodes.resources.enabled", true, "Enable node resources alarms")
	flag.String("alarms.nodes.resources.priority", "LOW", "The node resources alarm incident priority")
	flag.Int("alarms.nodes.resources.threshold", 90, "The node resources percentage threshold from 1 to 100")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	viper.RegisterAlias("settings.api-key", "settings.apiKey")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.SetEnvPrefix("ilert")
	viper.AutomaticEnv()

	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to read config")
	}

	if help {
		pflag.Usage()
		os.Exit(0)
	}

	if version {
		fmt.Println(shared.Version)
		os.Exit(0)
	}

	if cfgFile != "" {
		log.Debug().Str("file", cfgFile).Msg("Reading config file")
		viper.SetConfigFile(cfgFile)
		err := viper.ReadInConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("Unable to read config")
		}
	}

	cfg := &config.Config{}
	err = viper.Unmarshal(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to decode config")
	}

	if cfg.Links.Pods == nil {
		cfg.Links.Pods = make([]config.ConfigLinksSetting, 0)
	}
	if cfg.Links.Nodes == nil {
		cfg.Links.Nodes = make([]config.ConfigLinksSetting, 0)
	}

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], "ILERT_LINKS_PODS_") {
			link := strings.ReplaceAll(pair[0], "ILERT_LINKS_PODS_", "")
			cfg.Links.Pods = append(cfg.Links.Pods, config.ConfigLinksSetting{
				Name: strings.Title(strings.ToLower(strings.ReplaceAll(link, "_", " "))),
				Href: pair[1],
			})
		}

		if strings.HasPrefix(pair[0], "ILERT_LINKS_NODES_") {
			cfg.Links.Nodes = append(cfg.Links.Nodes, config.ConfigLinksSetting{
				Name: strings.Title(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(pair[0], "ILERT_LINKS_NODES_", ""), "_", " "))),
				Href: pair[1],
			})
		}
	}

	logger.Init(cfg.Settings.Log)

	ilertAPIKeyEnv := utils.GetEnv("ILERT_API_KEY", "")
	if ilertAPIKeyEnv != "" {
		cfg.Settings.APIKey = ilertAPIKeyEnv
	}

	if cfg.Settings.ElectionID == "" {
		log.Fatal().Msg("Election ID is required.")
	}

	if cfg.Settings.Namespace == "" {
		log.Fatal().Msg("Namespace is required.")
	}

	if cfg.Settings.APIKey == "" {
		log.Fatal().Msg("iLert api key is required. Use --settings.apiKey flag or ILERT_API_KEY env var")
	}

	if cfg.Settings.Log.Level != "debug" && cfg.Settings.Log.Level != "info" && cfg.Settings.Log.Level != "warn" && cfg.Settings.Log.Level != "error" && cfg.Settings.Log.Level != "fatal" {
		log.Fatal().Msg("Invalid --settings.log.level flag value or config.")
	}

	checkPriorityConfig(cfg.Alarms.Pods.Terminate.Priority, "--alarms.pods.terminate.priority")
	checkPriorityConfig(cfg.Alarms.Pods.Waiting.Priority, "--alarms.pods.waiting.priority")
	checkPriorityConfig(cfg.Alarms.Pods.Restarts.Priority, "--alarms.pods.restarts.priority")
	checkPriorityConfig(cfg.Alarms.Pods.Resources.Priority, "--alarms.pods.resources.priority")
	checkPriorityConfig(cfg.Alarms.Nodes.Terminate.Priority, "--alarms.nodes.terminate.priority")
	checkPriorityConfig(cfg.Alarms.Nodes.Resources.Priority, "--alarms.nodes.resources.priority")

	checkThresholdConfig(cfg.Alarms.Pods.Resources.Threshold, 1, 100, "--alarms.pods.resources.threshold")
	checkThresholdConfig(cfg.Alarms.Pods.Restarts.Threshold, 1, 1000000, "--alarms.pods.restarts.threshold")
	checkThresholdConfig(cfg.Alarms.Pods.Resources.Threshold, 1, 100, "--alarms.nodes.resources.threshold")

	return cfg
}

func checkPriorityConfig(priority string, flag string) {
	if priority != "HIGH" && priority != "LOW" {
		log.Fatal().Msg(fmt.Sprintf("Invalid %s flag value.", flag))
	}
}

func checkThresholdConfig(threshold int32, min int32, max int32, flag string) {
	if threshold < min || threshold > max {
		log.Fatal().Msg(fmt.Sprintf("Invalid %s flag value (min=%d max=%d).", flag, min, max))
	}
}
