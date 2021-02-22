package watcher

import (
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
)

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
)

var containerTerminatedReasons = []string{Terminated, OOMKilled, Error, ContainerCannotRun, DeadlineExceeded}

// Start starts watcher
func Start(kubeClient *kubernetes.Clientset, metricsClient *metrics.Clientset, agentKubeClient *agentclientset.Clientset, cfg *config.Config) {
	log.Info().Msg("Start watcher")

	if cfg.EnablePodAlarms {
		go startPodInformer(kubeClient, agentKubeClient, cfg)
		go startPodChecker(kubeClient, metricsClient, agentKubeClient, cfg)
	}
	if cfg.EnableNodeAlarms {
		go startNodeInformer(kubeClient, agentKubeClient, cfg)
	}
}

// Stop Stops watcher
func Stop() {
	log.Info().Msg("Stop watcher")

	stopPodInformer()
	stopNodeInformer()
	stopPodMetricsChecker()
}
