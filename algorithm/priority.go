package algorithm

import (
	"math"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

func init() {
	factory.RegisterPriorityFunction("MostUsed", MostRequestedPriority, 1)
}

//MostRequestedPriority determines the priority of nodes so that the highest utilization is chosen first
func MostRequestedPriority(pod *api.Pod, nodeNameToInfo map[string]*schedulercache.NodeInfo, nodes []*api.Node) (schedulerapi.HostPriorityList, error) {

	list := schedulerapi.HostPriorityList{}
	for _, node := range nodes {
		list = append(list, calculateResourceOccupancy(pod, node, nodeNameToInfo[node.Name].Pods()))
	}
	return list, nil
}

// Copied from normal scheduler priorities.go

// the unused capacity is calculated on a scale of 0-10
// 0 being the lowest priority and 10 being the highest
func calculateScore(requested int64, capacity int64, node string) int {
	if capacity == 0 {
		return 0
	}
	if requested > capacity {
		glog.V(2).Infof("Combined requested resources %d from existing pods exceeds capacity %d on node %s",
			requested, capacity, node)
		return 0
	}

	// Inverse of normal
	return 11 - int(math.Ceil(float64((capacity-requested)*10)/float64(capacity)))
}

// Calculate the resource occupancy on a node.  'node' has information about the resources on the node.
// 'pods' is a list of pods currently scheduled on the node.
func calculateResourceOccupancy(pod *api.Pod, node *api.Node, pods []*api.Pod) schedulerapi.HostPriority {
	totalMilliCPU := int64(0)
	totalMemory := int64(0)
	capacityMilliCPU := node.Status.Capacity.Cpu().MilliValue()
	capacityMemory := node.Status.Capacity.Memory().Value()

	for _, existingPod := range pods {
		cpu, memory := getResourcesForPod(existingPod)
		totalMilliCPU += cpu
		totalMemory += memory
	}
	// Add the resources requested by the current pod being scheduled.
	// This also helps differentiate between differently sized, but empty, nodes.
	for _, container := range pod.Spec.Containers {
		cpu, memory := getResourcesForPacking(&container.Resources)
		totalMilliCPU += cpu
		totalMemory += memory
	}

	cpuScore := calculateScore(totalMilliCPU, capacityMilliCPU, node.Name)
	memoryScore := calculateScore(totalMemory, capacityMemory, node.Name)
	glog.V(10).Infof(
		"%v -> %v: Most Requested Priority, Absolute/Requested: (%d, %d) / (%d, %d) Score: (%d, %d)",
		pod.Name, node.Name,
		totalMilliCPU, totalMemory,
		capacityMilliCPU, capacityMemory,
		cpuScore, memoryScore,
	)

	score := 0
	if cpuScore != 0 && memoryScore != 0 {
		score = int((cpuScore + memoryScore) / 2)
	}

	return schedulerapi.HostPriority{
		Host:  node.Name,
		Score: score,
	}
}
