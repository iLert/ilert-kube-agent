package watcher

import (
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/iLert/ilert-kube-agent/pkg/config"
)

var podCheckerCron *cron.Cron

func startPodChecker(cfg *config.Config) {
	podCheckerCron = cron.New()
	podCheckerCron.AddFunc(fmt.Sprintf("@every %s", cfg.Settings.CheckInterval), func() {
		checkPods(cfg)
	})

	log.Info().Msg("Starting pods checker")
	podCheckerCron.Start()
}

func stopPodMetricsChecker() {
	if podCheckerCron != nil {
		log.Info().Msg("Stopping pods checker")
		podCheckerCron.Stop()
	}
}

func checkPods(cfg *config.Config) {
	pods, err := cfg.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
	}

	if cfg.Alarms.Pods.Resources.Enabled {
		log.Debug().Msg("Running pods resource check")
		for _, pod := range pods.Items {
			analyzePodResources(&pod, cfg)
		}
	}
}

func getContainerByName(name string, containers []v1beta1.ContainerMetrics) *v1beta1.ContainerMetrics {
	var container v1beta1.ContainerMetrics
	for _, c := range containers {
		if c.Name == name {
			container = c
			break
		}
	}
	return &container
}
