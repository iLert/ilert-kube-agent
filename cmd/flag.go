package main

import (
	"flag"
	"os"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

var (
	kubeconfig                       string
	master                           string
	port                             string
	namespace                        string
	logLevel                         string
	electionID                       string
	ilertAPIKey                      string
	enablePodAlarms                  bool
	enablePodTerminateAlarms         bool
	enablePodWaitingAlarms           bool
	enablePodRestartsAlarms          bool
	enableNodeAlarms                 bool
	enableResourcesAlarms            bool
	podRestartThreshold              int32
	podAlarmIncidentPriority         string
	podRestartsAlarmIncidentPriority string
	nodeAlarmIncidentPriority        string
	resourcesAlarmIncidentPriority   string
	resourcesCheckerInterval         int32
	resourcesThreshold               int32
)

func parseAndValidateFlags() *config.Config {
	flags := pflag.NewFlagSet("", pflag.ExitOnError)

	flags.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flags.StringVar(&master, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flags.StringVar(&namespace, "namespace", "kube-system", "Namespace in which agent run.")
	flags.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, fatal).")
	flags.StringVar(&electionID, "election-id", "ilert-kube-agent", "The lease lock resource name")
	flags.StringVar(&port, "port", "9092", "The metrics server port")
	flags.StringVar(&ilertAPIKey, "ilert-api-key", "", "The iLert alert source api key")
	flags.BoolVar(&enablePodAlarms, "enable-pod-alarms", true, "Enable pod alarms")
	flags.BoolVar(&enablePodTerminateAlarms, "enable-pod-terminate-alarms", true, "Enable pod terminate alarms")
	flags.BoolVar(&enablePodWaitingAlarms, "enable-pod-waiting-alarms", true, "Enable pod waiting alarms")
	flags.BoolVar(&enablePodRestartsAlarms, "enable-pod-restarts-alarms", true, "Enable pod restarts alarms")
	flags.BoolVar(&enableNodeAlarms, "enable-node-alarms", true, "Enable node alarms")
	flags.BoolVar(&enableResourcesAlarms, "enable-resources-alarms", true, "Enable pod/node resources alarms")
	flags.Int32Var(&podRestartThreshold, "pod-restart-threshold", 10, "Pod restart threshold to alarm")
	flags.StringVar(&podAlarmIncidentPriority, "pod-alarm-incident-priority", "HIGH", "The pod alarm incident priority")
	flags.StringVar(&podRestartsAlarmIncidentPriority, "pod-restart-alarm-incident-priority", "LOW", "The pod restarts alarm incident priority")
	flags.StringVar(&nodeAlarmIncidentPriority, "node-alarm-incident-priority", "HIGH", "The node alarm incident priority")
	flags.StringVar(&resourcesAlarmIncidentPriority, "resources-alarm-incident-priority", "HIGH", "The node alarm incident priority")
	flags.Int32Var(&resourcesCheckerInterval, "resources-checker-interval", 30, "The resources checker interval in seconds")
	flags.Int32Var(&resourcesThreshold, "resources-threshold", 90, "The resources persentage threshold from 10 to 100")

	flags.Set("logtostderr", "true")
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)
	flag.CommandLine.Parse([]string{})

	pflag.VisitAll(func(flag *pflag.Flag) {
		log.Info().Str("name", flag.Name).Str("value", flag.Value.String()).Msg("Flag")
	})

	ilertAPIKeyEnv := utils.GetEnv("ILERT_API_KEY", "")
	if ilertAPIKeyEnv != "" {
		ilertAPIKey = ilertAPIKeyEnv
	}

	if electionID == "" {
		log.Fatal().Msg("Election ID is required.")
	}

	namespaceEnv := utils.GetEnv("NAMESPACE", "")
	if namespaceEnv != "" {
		namespace = namespaceEnv
	}
	if namespace == "" {
		log.Fatal().Msg("Namespace is required.")
	}

	if ilertAPIKey == "" {
		log.Fatal().Msg("iLert api key is required.")
	}

	logLevelEnv := utils.GetEnv("LOG_LEVEL", "")
	if logLevelEnv != "" {
		logLevel = logLevelEnv
	}
	if logLevel != "debug" && logLevel != "info" && logLevel != "warn" && logLevel != "error" && logLevel != "fatal" {
		log.Fatal().Msg("Invalid --log-level flag value.")
	}

	if podAlarmIncidentPriority != "HIGH" && podAlarmIncidentPriority != "LOW" {
		log.Fatal().Msg("Invalid --pod-alarm-incident-priority flag value.")
	}

	if podRestartsAlarmIncidentPriority != "HIGH" && podRestartsAlarmIncidentPriority != "LOW" {
		log.Fatal().Msg("Invalid --pod-restart-alarm-incident-priority flag value.")
	}

	if nodeAlarmIncidentPriority != "HIGH" && nodeAlarmIncidentPriority != "LOW" {
		log.Fatal().Msg("Invalid --node-alarm-incident-priority flag value.")
	}

	if resourcesAlarmIncidentPriority != "HIGH" && resourcesAlarmIncidentPriority != "LOW" {
		log.Fatal().Msg("Invalid --resources-alarm-incident-priority flag value.")
	}

	if resourcesThreshold < 10 || resourcesThreshold > 100 {
		log.Fatal().Msg("Invalid --resources-threshold flag value.")
	}

	return &config.Config{
		Master:                           master,
		KubeConfig:                       kubeconfig,
		Namespace:                        namespace,
		LogLevel:                         logLevel,
		APIKey:                           ilertAPIKey,
		EnablePodAlarms:                  enablePodAlarms,
		EnablePodTerminateAlarms:         enablePodTerminateAlarms,
		EnablePodWaitingAlarms:           enablePodWaitingAlarms,
		EnablePodRestartsAlarms:          enablePodRestartsAlarms,
		EnableNodeAlarms:                 enableNodeAlarms,
		EnableResourcesAlarms:            enableResourcesAlarms,
		PodRestartThreshold:              podRestartThreshold,
		PodAlarmIncidentPriority:         podAlarmIncidentPriority,
		PodRestartsAlarmIncidentPriority: podRestartsAlarmIncidentPriority,
		NodeAlarmIncidentPriority:        nodeAlarmIncidentPriority,
		ResourcesAlarmIncidentPriority:   resourcesAlarmIncidentPriority,
		ResourcesCheckerInterval:         resourcesCheckerInterval,
		ResourcesThreshold:               resourcesThreshold,
	}
}
