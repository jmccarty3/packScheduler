package algorithm

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	pluginPred "k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

func init() {
	factory.RegisterFitPredicateFactory(
		"PodOverCommitNode",
		func(args factory.PluginFactoryArgs) algorithm.FitPredicate {
			return NewPodOverCommitPredicate(args.NodeInfo)
		},
	)

	factory.RegisterFitPredicateFactory(
		"NodeOutOfDisk",
		func(args factory.PluginFactoryArgs) algorithm.FitPredicate {
			return NewNodeOutOfDiskPredicate(args.NodeInfo)
		},
	)

	factory.RegisterFitPredicate("DeisUniqueApp", UniqueDeisApp)
}

//ResourceOverCommit used to determine if scheduling pods would over commit a node
type ResourceOverCommit struct {
	info pluginPred.NodeInfo
}

//NodeDisk used to determine if a node is reporting out of disk
type NodeDisk struct {
	info pluginPred.NodeInfo
}

//OverCommitError records what resource a node would be overcommited on
type OverCommitError struct {
	name string
}

func (e *OverCommitError) Error() string {
	return fmt.Sprintf("Node would be overcommited on %s", e.name)
}

func newOverCommitError(name string) *OverCommitError {
	return &OverCommitError{
		name: name,
	}
}

//NewNodeOutOfDiskPredicate crestes a new NodeOutOfDisk Predicate
func NewNodeOutOfDiskPredicate(info pluginPred.NodeInfo) algorithm.FitPredicate {
	disk := &NodeDisk{
		info: info,
	}

	return disk.NodeOutOfDisk
}

//NodeOutOfDisk determine if a node is reporting out of disk.
func (d *NodeDisk) NodeOutOfDisk(pod *api.Pod, node string, cacheInfo *schedulercache.NodeInfo) (bool, error) {
	info, err := d.info.GetNodeInfo(node)

	if err != nil {
		return false, err
	}

	for _, c := range info.Status.Conditions {
		if c.Type != api.NodeOutOfDisk {
			continue
		}

		if c.Status == api.ConditionTrue {
			return false, nil
		}
	}

	return true, nil
}

//NewPodOverCommitPredicate creates a new PodOverCommit predicate
func NewPodOverCommitPredicate(info pluginPred.NodeInfo) algorithm.FitPredicate {
	commit := &ResourceOverCommit{
		info: info,
	}

	return commit.PodOverCommitNode
}

//PodOverCommitNode determines if pod resource request/limits would cause overcommit for a node
func (r *ResourceOverCommit) PodOverCommitNode(pod *api.Pod, node string, cacheInfo *schedulercache.NodeInfo) (bool, error) {
	info, err := r.info.GetNodeInfo(node)

	if err != nil {
		return false, err
	}

	pods := append(cacheInfo.Pods(), pod)
	totalCPU := int64(0)
	totalMem := int64(0)

	if int64(len(pods)) > info.Status.Capacity.Pods().Value() {
		glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would exceed Pod capacity", pod.Name, info.Name)
		return false, nil
	}

	for _, p := range pods {
		cpu, mem := getResourcesForPod(p)
		totalCPU += cpu
		totalMem += mem

		if totalCPU > info.Status.Capacity.Cpu().MilliValue() {
			glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would be overcommited on CPU", pod.Name, info.Name)
			return false, nil //TODO return newOverCommitError("CPU") when InsufficentResources can be modified
		}
		if totalMem > info.Status.Capacity.Memory().Value() {
			glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would be overcommited on Memory", pod.Name, info.Name)
			return false, nil //TODO return newOverCommitError("Memory") when InsufficentResources can be modified
		}
	}

	return true, nil
}

//UniqueDeisApp ensures that deis apps are unique by version on each node
func UniqueDeisApp(pod *api.Pod, node string, cacheInfo *schedulercache.NodeInfo) (bool, error) {
	if value, exists := pod.GetLabels()["heritage"]; !exists || value != "deis" {
		return true, nil //Pod is not from deis. Move along
	}

	labelSelector := labels.SelectorFromSet(pod.Labels)

	for _, p := range cacheInfo.Pods() {
		if labelSelector.Matches(labels.Set(p.Labels)) {
			return false, nil
		}
	}

	return true, nil
}
