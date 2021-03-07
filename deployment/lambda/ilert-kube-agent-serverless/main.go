package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rs/zerolog/log"

	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/logger"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
	"github.com/iLert/ilert-kube-agent/pkg/watcher"
)

func main() {
	lambda.Start(run)
}

func run(_ context.Context, event events.CloudWatchEvent) error {
	clusterName := utils.GetEnv("CLUSTER_NAME", "")
	region := utils.GetEnv("REGION", "")

	cfg := config.GetDefaultConfig()

	kubeConfig, err := getKubeConfig(clusterName, region)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get kube config")
		return err
	}
	cfg.SetKubeConfig(kubeConfig)

	cfg.Load()
	logger.Init(cfg.Settings.Log)
	// cfg.Print()

	watcher.RunOnce(cfg)

	return nil
}
