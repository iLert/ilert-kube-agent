package commander

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/apps/v1"
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

func getNewPodNameForDeployment(deployment *v1.Deployment, currentRS *v1.ReplicaSet, clientset *kubernetes.Clientset, timeout time.Duration, chPodName chan *string, chError chan error) {
	for start := time.Now(); start.Add(timeout).After(time.Now()); {
		deployment, err := clientset.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		if err != nil {
			log.Error().Err(err).
				Str("deployment_name", deployment.Name).
				Str("namespace", deployment.Namespace).
				Msg("failed to get deployment")
			chPodName <- nil
			chError <- fmt.Errorf("failed to get deployment: %v", err)
			return
		}
		_, _, newRS, err := GetAllReplicaSets(deployment, clientset.AppsV1())
		if err != nil {
			chPodName <- nil
			chError <- err
			return
		}
		if newRS.UID != currentRS.UID {
			log.Info().Str("new pod-template-hash", newRS.Labels["pod-template-hash"]).Str("old pod-template-hash", currentRS.Labels["pod-template-hash"]).Msg("Found new replica set: " + newRS.Name)
			podTemplateHash := newRS.Labels["pod-template-hash"]

			for start.Add(timeout).After(time.Now()) {
				podList, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "pod-template-hash=" + podTemplateHash})
				if err != nil {
					log.Warn().Err(err).
						Str("deployment_name", deployment.Name).
						Str("namespace", deployment.Namespace).
						Msg("failed to list pods")
					chPodName <- nil
					chError <- fmt.Errorf("failed to list pods")
					return
				}

				if len(podList.Items) == 0 {
					time.Sleep(time.Second)
					continue
				} else {
					newPodName := podList.Items[0].Name
					log.Info().Interface("newPodName", newPodName).Msg("Found new pod name")
					chPodName <- &newPodName
					chError <- nil
					return
				}
			}
			chPodName <- nil
			chError <- errors.New("timeout before a new replica is found")
			return
		}
		time.Sleep(time.Second)
	}
	chPodName <- nil
	chError <- errors.New("timeout before a new replica is found")
}

func getNewPodNameForStatefulSet(statefulSet *v1.StatefulSet, currentRevision string, clientset *kubernetes.Clientset, timeout time.Duration, chPodName chan *string, chError chan error) {
	for start := time.Now(); start.Add(timeout).After(time.Now()); {
		statefulSet, err := clientset.AppsV1().StatefulSets(statefulSet.Namespace).Get(context.TODO(), statefulSet.Name, metav1.GetOptions{})
		updateRevision := statefulSet.Status.UpdateRevision
		if err != nil {
			log.Error().Err(err).
				Str("statefulset_name", statefulSet.Name).
				Str("namespace", statefulSet.Namespace).
				Msg("failed to get statefulset")
			chPodName <- nil
			chError <- fmt.Errorf("failed to get statefulset: %v", err)
			return
		}
		if updateRevision != currentRevision {
			log.Info().Str("new controller-revision-hash", updateRevision).Str("old controller-revision-hash", currentRevision).Msg("Found new replica set: " + statefulSet.Name)
			for start.Add(timeout).After(time.Now()) {
				podList, err := clientset.CoreV1().Pods(statefulSet.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "controller-revision-hash=" + updateRevision})
				if err != nil {
					log.Warn().Err(err).
						Str("statefulset_name", statefulSet.Name).
						Str("namespace", statefulSet.Namespace).
						Msg("failed to list pods")
					chPodName <- nil
					chError <- fmt.Errorf("failed to list pods")
					return
				}

				if len(podList.Items) == 0 {
					time.Sleep(time.Second)
					continue
				} else {
					newPodName := podList.Items[0].Name
					log.Info().Interface("newPodName", newPodName).Msg("Found new pod name")
					chPodName <- &newPodName
					chError <- nil
					return
				}
			}
			chPodName <- nil
			chError <- errors.New("timeout before a new replica is found")
			return
		}
		time.Sleep(time.Second)
	}
	chPodName <- nil
	chError <- errors.New("timeout before a new replica is found")
}
