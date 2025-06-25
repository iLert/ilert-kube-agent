package commander

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func FindWorkloadByPodName(clientset *kubernetes.Clientset, namespace, podName string) (*WorkloadInfo, error, bool) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("failed to get pod")

		if ErrorMatchers.PodNotFound.Match([]byte(err.Error())) {
			return nil, fmt.Errorf("pod not found: %v", err), true
		}

		return nil, fmt.Errorf("failed to get pod: %v", err), false
	}

	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "ReplicaSet":
			rs, err := clientset.AppsV1().ReplicaSets(namespace).Get(context.TODO(), owner.Name, metav1.GetOptions{})
			if err != nil {
				log.Error().Err(err).
					Str("pod_name", podName).
					Str("namespace", namespace).
					Msg("failed to get replica set")
				continue
			}
			for _, rsOwner := range rs.OwnerReferences {
				if rsOwner.Kind == "Deployment" {
					return &WorkloadInfo{Type: WorkloadTypeDeployment, Name: rsOwner.Name}, nil, false
				}
			}
		case "StatefulSet":
			return &WorkloadInfo{Type: WorkloadTypeStatefulSet, Name: owner.Name}, nil, false
		case "Deployment":
			return &WorkloadInfo{Type: WorkloadTypeDeployment, Name: owner.Name}, nil, false
		}
	}

	return nil, fmt.Errorf("could not determine workload type for pod %s", podName), false
}
