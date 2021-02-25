package watcher

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/iLert/ilert-go"
	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/incident"
)

var nodeCheckerCron *cron.Cron

func startNodeChecker(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	nodeCheckerCron = cron.New()
	nodeCheckerCron.AddFunc(fmt.Sprintf("@every %ds", cfg.ResourcesCheckerInterval), func() {
		checkNodes(kubeClient, metricsClient, agentKubeClient, cfg)
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

func checkNodes(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	nodes, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
	}

	for _, node := range nodes.Items {
		if cfg.EnableResourcesAlarms {
			nodeKey := getNodeKey(&node)
			incidentRef := incident.GetIncidentRef(agentKubeClient, node.GetName(), cfg.Namespace)

			nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().Get(node.GetName(), metav1.GetOptions{})
			if err != nil {
				log.Debug().Err(err).Msg("Failed to get node metrics")
				continue
			}

			healthy := true
			var cpuUsage, memoryUsage int64
			cpuUsage, ok := nodeMetrics.Usage.Cpu().AsInt64()
			if !ok {
				cpuUsage = 0
			}
			memoryUsage, ok = nodeMetrics.Usage.Memory().AsInt64()
			if !ok {
				memoryUsage = 0
			}

			cpuLimit, ok := node.Status.Capacity.Cpu().AsInt64()
			if ok && cpuLimit > 0 {
				log.Debug().
					Str("node", node.GetName()).
					Int64("limit", cpuLimit).
					Int64("usage", cpuUsage).
					Msg("Checking CPU limit")
				if cpuUsage >= (int64(cfg.ResourcesThreshold) * (cpuLimit / 100)) {
					healthy = false
					if incidentRef == nil {
						summary := fmt.Sprintf("Node %s CPU limit reached > %d%%", node.GetName(), cfg.ResourcesThreshold)
						details := getNodeDetailsWithUsageLimit(kubeClient, &node, fmt.Sprintf("%d CPU", cpuUsage), fmt.Sprintf("%d CPU", cpuLimit))
						incidentID := incident.CreateEvent(cfg.APIKey, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.ResourcesAlarmIncidentPriority)
						incident.CreateIncidentRef(agentKubeClient, node.GetName(), cfg.Namespace, incidentID, summary, details)
					}
				}
			}

			memoryLimit, ok := node.Status.Capacity.Memory().AsInt64()
			if ok && memoryLimit > 0 {
				log.Debug().
					Str("node", node.GetName()).
					Int64("limit", memoryLimit).
					Int64("usage", memoryUsage).
					Msg("Checking memory limit")
				if memoryUsage >= (int64(cfg.ResourcesThreshold) * (memoryLimit / 100)) {
					healthy = false
					if incidentRef == nil {
						summary := fmt.Sprintf("Node %s memory limit reached > %d%%", node.GetName(), cfg.ResourcesThreshold)
						details := getNodeDetailsWithUsageLimit(kubeClient, &node, humanize.Bytes(uint64(memoryUsage)), humanize.Bytes(uint64(memoryLimit)))
						incidentID := incident.CreateEvent(cfg.APIKey, nodeKey, summary, details, ilert.EventTypes.Alert, cfg.ResourcesAlarmIncidentPriority)
						incident.CreateIncidentRef(agentKubeClient, node.GetName(), cfg.Namespace, incidentID, summary, details)
					}
				}
			}

			if healthy && incidentRef != nil && incidentRef.Spec.ID > 0 {
				incident.CreateEvent(cfg.APIKey, nodeKey, fmt.Sprintf("Node %s recovered", node.GetName()), "", ilert.EventTypes.Resolve, cfg.ResourcesAlarmIncidentPriority)
				incident.DeleteIncidentRef(agentKubeClient, node.GetName(), cfg.Namespace)
			}
		}
	}
}
