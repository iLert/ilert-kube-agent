package main

import (
	"encoding/base64"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

func getKubeConfig(clusterName string, region string) (*rest.Config, error) {
	if clusterName == "" {
		err := errors.New("Cluster name is required")
		log.Error().Err(err).Msg("Failed to create kube client")
		return nil, err
	}

	if region == "" {
		err := errors.New("Region is required")
		log.Error().Err(err).Msg("Failed to create kube client")
		return nil, err
	}

	s, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	svc := eks.New(s)
	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	clusterInfo, err := svc.DescribeCluster(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			log.Error().Err(aerr).Str("code", aerr.Code()).Msg("Failed to describe cluster")
		} else {
			log.Error().Err(err).Msg("Failed to describe cluster")
		}
		return nil, err
	}

	ca, err := base64.StdEncoding.DecodeString(*clusterInfo.Cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, err
	}

	gen, err := token.NewGenerator(false, false)
	if err != nil {
		return nil, err
	}

	tkn, err := gen.Get(*clusterInfo.Cluster.Name)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host:        *clusterInfo.Cluster.Endpoint,
		BearerToken: tkn.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}, nil
}
