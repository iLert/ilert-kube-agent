package watcher

import (
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/memory"
)

var nodeCheckerCron *cron.Cron

func startNodeChecker(cfg *config.Config) {
	nodeCheckerCron = cron.New()
	nodeCheckerCron.AddFunc(fmt.Sprintf("@every %s", cfg.Settings.CheckInterval), func() {
		checkNodes(cfg)
	})

	log.Info().Msg("Starting nodes checker")
	nodeCheckerCron.Start()
}

func stopNodeMetricsChecker() {
	if nodeCheckerCron != nil {
		log.Info().Msg("Stopping nodes checker")
		nodeCheckerCron.Stop()
		nodeCheckerCron = nil
	}
}

func checkNodes(cfg *config.Config) {
	defer memory.RecoverPanic("node-checker")

	if memory.GetGlobalMonitor() != nil && memory.GetGlobalMonitor().IsUnderPressure() {
		pressureLevel := memory.GetGlobalMonitor().GetPressureLevel()
		if pressureLevel == "critical" || pressureLevel == "emergency" {
			log.Warn().Str("pressure_level", pressureLevel).Msg("Skipping node resource check due to memory pressure")
			return
		}
	}

	if !cfg.Alarms.Nodes.Resources.Enabled {
		return
	}

	informer := GetNodeInformer()
	if informer == nil {
		log.Warn().Msg("Node informer not available, skipping resource check")
		return
	}

	if !cache.WaitForCacheSync(nil, informer.HasSynced) {
		log.Warn().Msg("Node informer cache not synced, skipping resource check")
		return
	}

	log.Debug().Msg("Running nodes resource check")

	nodes := informer.GetStore().List()
	for _, obj := range nodes {
		node, ok := obj.(*api.Node)
		if !ok {
			log.Debug().Msg("Failed to convert object to node, skipping")
			continue
		}
		analyzeNodeResources(node, cfg)
	}
}
