package config

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// Validate analyze config values and throws an error if some problem found
func (cfg *Config) Validate() {
	if cfg.Settings.ElectionID == "" {
		log.Fatal().Msg("Election ID is required.")
	}

	if cfg.Settings.Namespace == "" {
		log.Fatal().Msg("Namespace is required. Use --settings.namespace flag or NAMESPACE env var")
	}

	if cfg.Settings.APIKey == "" {
		log.Fatal().Msg("iLert api key is required. Use --settings.apiKey flag or ILERT_API_KEY env var")
	}

	if cfg.Settings.Log.Level != "debug" && cfg.Settings.Log.Level != "info" && cfg.Settings.Log.Level != "warn" && cfg.Settings.Log.Level != "error" && cfg.Settings.Log.Level != "fatal" {
		log.Fatal().Msg("Invalid --settings.log.level flag value or config.")
	}

	checkPriority(cfg.Alarms.Pods.Terminate.Priority, "--alarms.pods.terminate.priority")
	checkPriority(cfg.Alarms.Pods.Waiting.Priority, "--alarms.pods.waiting.priority")
	checkPriority(cfg.Alarms.Pods.Restarts.Priority, "--alarms.pods.restarts.priority")
	checkPriority(cfg.Alarms.Pods.Resources.CPU.Priority, "--alarms.pods.resources.cpu.priority")
	checkPriority(cfg.Alarms.Pods.Resources.Memory.Priority, "--alarms.pods.resources.memory.priority")
	checkPriority(cfg.Alarms.Nodes.Terminate.Priority, "--alarms.nodes.terminate.priority")
	checkPriority(cfg.Alarms.Nodes.Resources.CPU.Priority, "--alarms.nodes.resources.cpu.priority")
	checkPriority(cfg.Alarms.Nodes.Resources.Memory.Priority, "--alarms.nodes.resources.memory.priority")

	checkThreshold(cfg.Alarms.Pods.Resources.CPU.Threshold, 1, 100, "--alarms.pods.resources.cpu.threshold")
	checkThreshold(cfg.Alarms.Pods.Resources.Memory.Threshold, 1, 100, "--alarms.pods.resources.memory.threshold")
	checkThreshold(cfg.Alarms.Pods.Restarts.Threshold, 1, 1000000, "--alarms.pods.restarts.threshold")
	checkThreshold(cfg.Alarms.Pods.Resources.CPU.Threshold, 1, 100, "--alarms.nodes.resources.cpu.threshold")
	checkThreshold(cfg.Alarms.Pods.Resources.Memory.Threshold, 1, 100, "--alarms.nodes.resources.memory.threshold")
}

func checkPriority(priority string, flag string) {
	if priority != "HIGH" && priority != "LOW" {
		log.Fatal().Msg(fmt.Sprintf("Invalid %s flag value.", flag))
	}
}

func checkThreshold(threshold int32, min int32, max int32, flag string) {
	if threshold < min || threshold > max {
		log.Fatal().Msg(fmt.Sprintf("Invalid %s flag value (min=%d max=%d).", flag, min, max))
	}
}
