package watcher

import (
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/memory"
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
		podCheckerCron = nil
	}
}

func checkPods(cfg *config.Config) {
	defer memory.RecoverPanic("pod-checker")

	if memory.GetGlobalMonitor() != nil && memory.GetGlobalMonitor().IsUnderPressure() {
		pressureLevel := memory.GetGlobalMonitor().GetPressureLevel()
		if pressureLevel == "critical" || pressureLevel == "emergency" {
			log.Warn().Str("pressure_level", pressureLevel).Msg("Skipping pod resource check due to memory pressure")
			return
		}
	}

	if !cfg.Alarms.Pods.Resources.Enabled {
		return
	}

	informer := GetPodInformer()
	if informer == nil {
		log.Warn().Msg("Pod informer not available, skipping resource check")
		return
	}

	if !cache.WaitForCacheSync(nil, informer.HasSynced) {
		log.Warn().Msg("Pod informer cache not synced, skipping resource check")
		return
	}

	log.Debug().Msg("Running pods resource check")

	pods := informer.GetStore().List()
	for _, obj := range pods {
		pod, ok := obj.(*api.Pod)
		if !ok {
			log.Debug().Msg("Failed to convert object to pod, skipping")
			continue
		}
		analyzePodResources(pod, cfg)
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
