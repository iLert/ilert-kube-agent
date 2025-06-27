/*

This file contains source code from kubectl.
https://github.com/kubernetes/kubectl.git

Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package commander

import (
	"context"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeresource "k8s.io/cli-runtime/pkg/resource"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
)

// GetAllReplicaSets returns the old and new replica sets targeted by the given Deployment. It gets PodList and
// ReplicaSetList from client interface. Note that the first set of old replica sets doesn't include the ones
// with no pods, and the second set of old replica sets include all old replica sets. The third returned value
// is the new replica set, and it may be nil if it doesn't exist yet.
func GetAllReplicaSets(deployment *appsv1.Deployment, c appsclient.AppsV1Interface) ([]*appsv1.ReplicaSet, []*appsv1.ReplicaSet, *appsv1.ReplicaSet, error) {
	rsList, err := listReplicaSets(deployment, rsListFromClient(c), nil)
	if err != nil {
		return nil, nil, nil, err
	}
	newRS := findNewReplicaSet(deployment, rsList)
	oldRSes, allOldRSes := findOldReplicaSets(deployment, rsList, newRS)
	return oldRSes, allOldRSes, newRS, nil
}

// TODO: switch this to full namespacers
type rsListFunc func(string, metav1.ListOptions) ([]*appsv1.ReplicaSet, error)

// listReplicaSets returns a slice of RSes the given deployment targets.
// Note that this does NOT attempt to reconcile ControllerRef (adopt/orphan),
// because only the controller itself should do that.
// However, it does filter out anything whose ControllerRef doesn't match.
func listReplicaSets(deployment *appsv1.Deployment, getRSList rsListFunc, chunkSize *int64) ([]*appsv1.ReplicaSet, error) {
	// TODO: Right now we list replica sets by their labels. We should list them by selector, i.e. the replica set's selector
	//       should be a superset of the deployment's selector, see https://github.com/kubernetes/kubernetes/issues/19830.
	namespace := deployment.Namespace
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: selector.String()}
	if chunkSize != nil {
		options.Limit = *chunkSize
	}
	all, err := getRSList(namespace, options)
	if err != nil {
		return nil, err
	}
	// Only include those whose ControllerRef matches the Deployment.
	owned := make([]*appsv1.ReplicaSet, 0, len(all))
	for _, rs := range all {
		if metav1.IsControlledBy(rs, deployment) {
			owned = append(owned, rs)
		}
	}
	return owned, nil
}

// RsListFromClient returns an rsListFunc that wraps the given client.
func rsListFromClient(c appsclient.AppsV1Interface) rsListFunc {
	return func(namespace string, initialOpts metav1.ListOptions) ([]*appsv1.ReplicaSet, error) {
		rsList := &appsv1.ReplicaSetList{}
		err := runtimeresource.FollowContinue(&initialOpts,
			func(opts metav1.ListOptions) (runtime.Object, error) {
				newRs, err := c.ReplicaSets(namespace).List(context.TODO(), opts)
				if err != nil {
					return nil, runtimeresource.EnhanceListError(err, opts, "replicasets")
				}
				rsList.Items = append(rsList.Items, newRs.Items...)
				return newRs, nil
			})
		if err != nil {
			return nil, err
		}
		var ret []*appsv1.ReplicaSet
		for i := range rsList.Items {
			ret = append(ret, &rsList.Items[i])
		}
		return ret, err
	}
}

// FindNewReplicaSet returns the new RS this given deployment targets (the one with the same pod template).
func findNewReplicaSet(deployment *appsv1.Deployment, rsList []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	sort.Sort(replicaSetsByCreationTimestamp(rsList))
	for i := range rsList {
		if equalIgnoreHash(&rsList[i].Spec.Template, &deployment.Spec.Template) {
			// In rare cases, such as after cluster upgrades, Deployment may end up with
			// having more than one new ReplicaSets that have the same template as its template,
			// see https://github.com/kubernetes/kubernetes/issues/40415
			// We deterministically choose the oldest new ReplicaSet.
			return rsList[i]
		}
	}
	// new ReplicaSet does not exist.
	return nil
}

// replicaSetsByCreationTimestamp sorts a list of ReplicaSet by creation timestamp, using their names as a tie breaker.
type replicaSetsByCreationTimestamp []*appsv1.ReplicaSet

func (o replicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o replicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o replicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// // FindOldReplicaSets returns the old replica sets targeted by the given Deployment, with the given slice of RSes.
// // Note that the first set of old replica sets doesn't include the ones with no pods, and the second set of old replica sets include all old replica sets.
func findOldReplicaSets(deployment *appsv1.Deployment, rsList []*appsv1.ReplicaSet, newRS *appsv1.ReplicaSet) ([]*appsv1.ReplicaSet, []*appsv1.ReplicaSet) {
	var requiredRSs []*appsv1.ReplicaSet
	var allRSs []*appsv1.ReplicaSet
	for _, rs := range rsList {
		// Filter out new replica set
		if newRS != nil && rs.UID == newRS.UID {
			continue
		}
		allRSs = append(allRSs, rs)
		if *(rs.Spec.Replicas) != 0 {
			requiredRSs = append(requiredRSs, rs)
		}
	}
	return requiredRSs, allRSs
}

// EqualIgnoreHash returns true if two given podTemplateSpec are equal, ignoring the diff in value of Labels[pod-template-hash]
// We ignore pod-template-hash because:
//  1. The hash result would be different upon podTemplateSpec API changes
//     (e.g. the addition of a new field will cause the hash code to change)
//  2. The deployment template won't have hash labels
func equalIgnoreHash(template1, template2 *corev1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	// Remove hash labels from template.Labels before comparing
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}
