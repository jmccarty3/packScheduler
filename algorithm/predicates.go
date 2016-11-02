package algorithm

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	pluginPred "k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

const (
	nodeOutOfDiskPred     = "NodeOutOfDisk"
	podOverCommitNodePred = "PodOverCommitNode"
	deisUniqueAppPred     = "DeisUniqueApp"
)

var (
	nodeOutOfDiskPredError     = newPredicateFailure(nodeOutOfDiskPred)
	podOverCommitNodePredError = newPredicateFailure(podOverCommitNodePred)
	deisUniqueAppPredError     = newPredicateFailure(deisUniqueAppPred)
)

func newPredicateFailure(predicateName string) *pluginPred.PredicateFailureError {
	return &pluginPred.PredicateFailureError{PredicateName: predicateName}
}

func init() {
	factory.RegisterFitPredicate(
		podOverCommitNodePred,
		PodOverCommitNode,
	)

	factory.RegisterFitPredicate(
		nodeOutOfDiskPred,
		NodeOutOfDisk,
	)

	factory.RegisterFitPredicate(deisUniqueAppPred, UniqueDeisApp)
}

//NodeOutOfDisk determine if a node is reporting out of disk.
func NodeOutOfDisk(pod *api.Pod, meta interface{}, cacheInfo *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	info := cacheInfo.Node()

	for _, c := range info.Status.Conditions {
		if c.Type != api.NodeOutOfDisk {
			continue
		}

		if c.Status == api.ConditionTrue {
			return false, []algorithm.PredicateFailureReason{nodeOutOfDiskPredError}, nil
		}
	}

	return true, nil, nil
}

//PodOverCommitNode determines if pod resource request/limits would cause overcommit for a node
func PodOverCommitNode(pod *api.Pod, meta interface{}, cacheInfo *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	info := cacheInfo.Node()

	pods := append(cacheInfo.Pods(), pod)
	totalCPU := int64(0)
	totalMem := int64(0)

	if int64(len(pods)) > info.Status.Capacity.Pods().Value() {
		glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would exceed Pod capacity", pod.Name, info.Name)
		return false, []algorithm.PredicateFailureReason{podOverCommitNodePredError}, nil
	}

	for _, p := range pods {
		cpu, mem := getResourcesForPod(p)
		totalCPU += cpu
		totalMem += mem

		if totalCPU > info.Status.Capacity.Cpu().MilliValue() {
			glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would be overcommited on CPU", pod.Name, info.Name)
			return false, []algorithm.PredicateFailureReason{podOverCommitNodePredError}, nil //TODO return newOverCommitError("CPU") when InsufficentResources can be modified
		}
		if totalMem > info.Status.Capacity.Memory().Value() {
			glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would be overcommited on Memory", pod.Name, info.Name)
			return false, []algorithm.PredicateFailureReason{podOverCommitNodePredError}, nil //TODO return newOverCommitError("Memory") when InsufficentResources can be modified
		}
	}

	return true, nil, nil
}

//UniqueDeisApp ensures that deis apps are unique by version on each node
func UniqueDeisApp(pod *api.Pod, meta interface{}, cacheInfo *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	if value, exists := pod.GetLabels()["heritage"]; !exists || value != "deis" {
		return true, nil, nil //Pod is not from deis. Move along
	}

	labelSelector := labels.SelectorFromSet(pod.Labels)

	for _, p := range cacheInfo.Pods() {
		if labelSelector.Matches(labels.Set(p.Labels)) {
			return false, []algorithm.PredicateFailureReason{deisUniqueAppPredError}, nil
		}
	}

	return true, nil, nil
}
