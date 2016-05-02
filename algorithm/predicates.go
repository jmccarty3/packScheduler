package algorithm

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	pluginPred "k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
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
}

//ResourceOverCommit used to determine if scheduling pods would over commit a node
type ResourceOverCommit struct {
	info pluginPred.NodeInfo
}

//NodeDisk used to determine if a node is reporting out of disk
type NodeDisk struct {
	info pluginPred.NodeInfo
}

//NewNodeOutOfDiskPredicate crestes a new NodeOutOfDisk Predicate
func NewNodeOutOfDiskPredicate(info pluginPred.NodeInfo) algorithm.FitPredicate {
	disk := &NodeDisk{
		info: info,
	}

	return disk.NodeOutOfDisk
}

//NodeOutOfDisk determine if a node is reporting out of disk.
func (d *NodeDisk) NodeOutOfDisk(pod *api.Pod, existingPods []*api.Pod, node string) (bool, error) {
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
func (r *ResourceOverCommit) PodOverCommitNode(pod *api.Pod, existingPods []*api.Pod, node string) (bool, error) {
	info, err := r.info.GetNodeInfo(node)

	if err != nil {
		return false, err
	}

	pods := append(existingPods, pod)
	totalCPU := int64(0)
	totalMem := int64(0)

	if int64(len(pods)) > info.Status.Capacity.Pods().Value() {
		glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would exceed Pod capacity", pod.Name, info.Name)
		pluginPred.FailedResourceType = "PodExceedsMaxPodNumber"
		return false, nil
	}

	for _, p := range pods {
		cpu, mem := getResourcesForPod(p)
		totalCPU += cpu
		totalMem += mem

		if totalCPU > info.Status.Capacity.Cpu().MilliValue() {
			glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would be overcommited on CPU", pod.Name, info.Name)
			pluginPred.FailedResourceType = "PodOverCommitsCPU"
			return false, nil
		}
		if totalMem > info.Status.Capacity.Memory().Value() {
			glog.V(10).Infof("Cannot schedule Pod %s, Because Node %v would be overcommited on Memory", pod.Name, info.Name)
			pluginPred.FailedResourceType = "PodOverCommitsMemory"
			return false, nil
		}
	}

	return true, nil
}
