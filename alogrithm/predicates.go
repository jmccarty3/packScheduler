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
}

type ResourceOverCommit struct {
	info pluginPred.NodeInfo
}

func NewPodOverCommitPredicate(info pluginPred.NodeInfo) algorithm.FitPredicate {
	commit := &ResourceOverCommit{
		info: info,
	}

	return commit.PodOverCommitNode
}

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
