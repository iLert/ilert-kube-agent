package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	shared "github.com/iLert/ilert-kube-agent"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/logger"
)

var (
	help    bool
	version bool
	runOnce bool
	cfgFile string
)

func parseAndValidateFlags() *config.Config {

	flag.BoolVar(&help, "help", false, "Print this help.")
	flag.BoolVar(&version, "version", false, "Print version.")
	flag.BoolVar(&runOnce, "run-once", false, "Run checks only once and exit.")
	flag.StringVar(&cfgFile, "config", "", "Config file")

	flag.String("settings.kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.String("settings.master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.Bool("settings.insecure", false, "The Kubernetes API server should be accessed without verifying the TLS certificate. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.String("settings.namespace", "kube-system", "Namespace in which agent run.")
	flag.String("settings.log.level", "info", "Log level (debug, info, warn, error, fatal).")
	flag.Bool("settings.log.json", false, "Enable json format log")
	flag.String("settings.electionID", "ilert-kube-agent", "The lease lock resource name")
	flag.Int("settings.port", 9092, "The metrics server port")
	flag.String("settings.apiKey", "", "(REQUIRED) The iLert alert source api key")
	flag.String("settings.checkInterval", "15s", "The evaluation check interval e.g. resources check")

	flag.Bool("alarms.cluster.enabled", true, "Enable cluster alarms")
	flag.String("alarms.cluster.priority", "HIGH", "The cluster alarm incident priority")

	flag.Bool("alarms.pods.enabled", true, "Enable pod alarms")
	flag.Bool("alarms.pods.terminate.enabled", true, "Enable pod terminate alarms")
	flag.String("alarms.pods.terminate.priority", "HIGH", "The pod terminate alarm incident priority")
	flag.Bool("alarms.pods.waiting.enabled", true, "Enable pod waiting alarms")
	flag.String("alarms.pods.waiting.priority", "LOW", "The pod waiting alarm incident priority")
	flag.Bool("alarms.pods.restarts.enabled", true, "Enable pod restarts alarms")
	flag.String("alarms.pods.restarts.priority", "LOW", "The pod waiting alarm incident priority")
	flag.Int("alarms.pods.restarts.threshold", 10, "Pod restart threshold to alarm")
	flag.Bool("alarms.pods.resources.enabled", true, "Enable pod resources alarms")
	flag.Bool("alarms.pods.resources.cpu.enabled", true, "Enable pod CPU resources alarms")
	flag.String("alarms.pods.resources.cpu.priority", "LOW", "The pod CPU resources alarm incident priority")
	flag.Int("alarms.pods.resources.cpu.threshold", 90, "The pod CPU resources percentage threshold from 1 to 100")
	flag.Bool("alarms.pods.resources.memory.enabled", true, "Enable pod memory resources alarms")
	flag.String("alarms.pods.resources.memory.priority", "LOW", "The pod memory resources alarm incident priority")
	flag.Int("alarms.pods.resources.memory.threshold", 90, "The pod memory resources percentage threshold from 1 to 100")

	flag.Bool("alarms.nodes.enabled", true, "Enable node alarms")
	flag.Bool("alarms.nodes.terminate.enabled", true, "Enable node terminate alarms")
	flag.String("alarms.nodes.terminate.priority", "HIGH", "The node terminate alarm incident priority")
	flag.Bool("alarms.nodes.resources.enabled", true, "Enable node resources alarms")
	flag.Bool("alarms.nodes.resources.cpu.enabled", true, "Enable node CPU resources alarms")
	flag.String("alarms.nodes.resources.cpu.priority", "LOW", "The node CPU resources alarm incident priority")
	flag.Int("alarms.nodes.resources.cpu.threshold", 90, "The node CPU resources percentage threshold from 1 to 100")
	flag.Bool("alarms.nodes.resources.memory.enabled", true, "Enable node memory resources alarms")
	flag.String("alarms.nodes.resources.memory.priority", "LOW", "The node memory resources alarm incident priority")
	flag.Int("alarms.nodes.resources.memory.threshold", 90, "The node memory resources percentage threshold from 1 to 100")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	viper.RegisterAlias("settings.api-key", "settings.apiKey")

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

	cfg := &config.Config{}
	if cfgFile != "" {
		cfg.SetConfigFile(cfgFile)
	}
	if runOnce {
		cfg.SetRunOnce(true)
	}
	cfg.Load()
	cfg.Validate()
	logger.Init(cfg.Settings.Log)

	return cfg
}
