package watcher

import (
	"context"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/logger"
	"github.com/iLert/ilert-kube-agent/pkg/memory"
)

var sharedFactory informers.SharedInformerFactory

// These are the valid reason for the container waiting
const (
	CrashLoopBackOff           = "CrashLoopBackOff"
	ErrImagePull               = "ErrImagePull"
	ImagePullBackOff           = "ImagePullBackOff"
	CreateContainerConfigError = "CreateContainerConfigError"
	InvalidImageName           = "InvalidImageName"
	CreateContainerError       = "CreateContainerError"
)

var containerWaitingReasons = []string{CrashLoopBackOff, ErrImagePull, ImagePullBackOff, CreateContainerConfigError, InvalidImageName, CreateContainerError}

// These are the valid reason for the container terminated
const (
	Terminated         = "Terminated"
	OOMKilled          = "OOMKilled"
	Error              = "Error"
	ContainerCannotRun = "ContainerCannotRun"
	DeadlineExceeded   = "DeadlineExceeded"
	Evicted            = "Evicted"
)

var containerTerminatedReasons = []string{Terminated, OOMKilled, Error, ContainerCannotRun, DeadlineExceeded, Evicted}

// Start starts watcher
func Start(cfg *config.Config) {
	log.Info().Msg("Start watcher")

	if cfg.Alarms.Pods.Enabled {
		memory.SafeGo("pod-informer", func() {
			startPodInformer(cfg)
		})
		memory.SafeGo("pod-checker", func() {
			startPodChecker(cfg)
		})
	}
	if cfg.Alarms.Nodes.Enabled {
		memory.SafeGo("node-informer", func() {
			startNodeInformer(cfg)
		})
		memory.SafeGo("node-checker", func() {
			startNodeChecker(cfg)
		})
	}
}

// Stop Stops watcher
func Stop() {
	log.Info().Msg("Stop watcher")

	stopPodInformer()
	stopPodMetricsChecker()
	stopNodeInformer()
	stopNodeMetricsChecker()

	if sharedFactory != nil {
		log.Info().Msg("Stopping shared informer factory")
		sharedFactory = nil
	}
}

// RunOnce run watcher runs e.g. serverless call
func RunOnce(cfg *config.Config) {
	log.Info().Msg("Run watcher once")

	cfg.Validate()
	logger.Init(cfg.Settings.Log)

	err := analyzeClusterStatus(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check cluster status")
		return
	}

	if cfg.Alarms.Pods.Enabled {
		pods, err := cfg.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
		}

		for _, pod := range pods.Items {
			analyzePodStatus(&pod, cfg)
			analyzePodResources(&pod, cfg)
		}
	}
	if cfg.Alarms.Nodes.Enabled {
		nodes, err := cfg.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
		}

		log.Debug().Msg("Running nodes resource check")
		for _, node := range nodes.Items {
			analyzeNodeStatus(&node, cfg)
			analyzeNodeResources(&node, cfg)
		}
	}

	log.Info().Msg("Watcher finished")
}
