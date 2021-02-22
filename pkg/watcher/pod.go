package watcher

import (
	"bytes"
	"fmt"
	"io"

	"github.com/iLert/ilert-kube-agent/pkg/utils"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func getPodKey(pod *api.Pod) string {
	return fmt.Sprintf("%s/%s", pod.GetName(), pod.GetNamespace())
}
func getPodDetailsWithUsageLimit(kubeClient *kubernetes.Clientset, pod *api.Pod, usage string, limit string) string {
	details := fmt.Sprintf("Name: %s\nNamespace: %s",
		pod.GetName(),
		pod.GetNamespace())

	if usage != "" {
		details += fmt.Sprintf("\nUsage: %s", usage)
	}
	if limit != "" {
		details += fmt.Sprintf("\nLimit: %s", limit)
	}
	return details
}

func getPodDetailsWithStatus(kubeClient *kubernetes.Clientset, pod *api.Pod, containerStatus *api.ContainerStatus) string {
	details := fmt.Sprintf("Name: %s\nNamespace: %s",
		pod.GetName(),
		pod.GetNamespace())

	if containerStatus != nil && containerStatus.State.Terminated != nil {
		details += fmt.Sprintf("\nReason: %s\nExit code: %d\nStarted at: %s\nFinished at: %s",
			containerStatus.State.Terminated.Reason,
			containerStatus.State.Terminated.ExitCode,
			containerStatus.State.Terminated.StartedAt,
			containerStatus.State.Terminated.FinishedAt,
		)
	}

	if containerStatus != nil && containerStatus.State.Waiting != nil {
		details += fmt.Sprintf("\nReason: %s\nMessage: %s",
			containerStatus.State.Waiting.Reason,
			containerStatus.State.Waiting.Message,
		)
	}

	podLogs := getPodLogs(kubeClient, pod, containerStatus.Name)
	if podLogs != "" {
		details += fmt.Sprintf("\nLogs:\n%s", podLogs)
	}

	return details
}

func getPodLogs(kubeClient *kubernetes.Clientset, pod *api.Pod, container string) string {
	podLogOpts := api.PodLogOptions{
		TailLines: utils.Int64(50),
		Container: container,
	}

	req := kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream()
	if err != nil {
		return ""
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return ""
	}

	return buf.String()

}
