package watcher

import api "k8s.io/api/core/v1"

func getLabel(pod *api.Pod, label string) string {
	label, exists := pod.ObjectMeta.Labels[label]
	if exists {
		return label
	}
	return ""
}
