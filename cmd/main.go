package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/router"
	"github.com/iLert/ilert-kube-agent/pkg/storage"
	"github.com/iLert/ilert-kube-agent/pkg/watcher"

	"github.com/rs/zerolog/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
)

func main() {
	cfg := parseAndValidateFlags()

	log.Info().Interface("config", cfg).Msg("Starting agent with config")

	srg := &storage.Storage{}
	srg.Init()
	router := router.Setup(srg)

	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf(":%d", cfg.Settings.Port),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start Server
	go func() {
		log.Info().Str("address", fmt.Sprintf(":%d", cfg.Settings.Port)).Msg("Starting Server")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	id, err := os.Hostname()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to get hostname")
	}

	config, err := clientcmd.BuildConfigFromFlags(cfg.Settings.Master, cfg.Settings.KubeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build kubeconfig")
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create kube client")
	}

	agentKubeClient, err := agentclientset.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create kube client")
	}

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create metrics client")
	}

	// Validate that the client is ok.
	_, err = kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes from apiserver")
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      cfg.Settings.ElectionID,
			Namespace: cfg.Settings.Namespace,
		},
		Client: kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		log.Info().Msg("Received termination, signaling shutdown")
		cancel()
	}()

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   defaultLeaseDuration,
		RenewDeadline:   defaultRenewDeadline,
		RetryPeriod:     defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				log.Info().Str("identity", id).Msg("I am the new leader")
				watcher.Start(kubeClient, metricsClient, agentKubeClient, cfg)
			},
			OnStoppedLeading: func() {
				watcher.Stop()
				log.Info().Str("identity", id).Msg("I am not leader anymore")
			},
			OnNewLeader: func(identity string) {
				log.Info().Str("identity", identity).Msg("New leader elected")
			},
		},
	})
}
