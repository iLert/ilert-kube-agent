package watcher

import (
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/iLert/ilert-kube-agent/pkg/config"
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
	}
}

func checkNodes(cfg *config.Config) {
	nodes, err := cfg.KubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
	}

	log.Debug().Msg("Running nodes resource check")
	for _, node := range nodes.Items {
		analyzeNodeResources(&node, cfg)
	}
}
