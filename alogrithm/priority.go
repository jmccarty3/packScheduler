package algorithm

import (
	"math"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
)

func init() {
	factory.RegisterPriorityFunction("MostUsed", MostRequestedPriority, 1)
}

func MostRequestedPriority(pod *api.Pod, podLister algorithm.PodLister, nodeLister algorithm.NodeLister) (algorithm.HostPriorityList, error) {
	nodes, err := nodeLister.List()
	if err != nil {
		return algorithm.HostPriorityList{}, err
	}
	podsToMachines, err := predicates.MapPodsToMachines(podLister)

	list := algorithm.HostPriorityList{}
	for _, node := range nodes.Items {
		list = append(list, calculateResourceOccupancy(pod, node, podsToMachines[node.Name]))
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

// For each of these resources, a pod that doesn't request the resource explicitly
// will be treated as having requested the amount indicated below, for the purpose
// of computing priority only. This ensures that when scheduling zero-request pods, such
// pods will not all be scheduled to the machine with the smallest in-use request,
// and that when scheduling regular pods, such pods will not see zero-request pods as
// consuming no resources whatsoever. We chose these values to be similar to the
// resources that we give to cluster addon pods (#10653). But they are pretty arbitrary.
// As described in #11713, we use request instead of limit to deal with resource requirements.
const defaultMilliCPURequest int64 = 100             // 0.1 core
const defaultMemoryRequest int64 = 200 * 1024 * 1024 // 200 MB

// TODO: Consider setting default as a fixed fraction of machine capacity (take "capacity api.ResourceList"
// as an additional argument here) rather than using constants
func getNonzeroRequests(requests *api.ResourceList) (int64, int64) {
	var millicpu, memory int64
	// Override if un-set, but not if explicitly set to zero
	if (*requests.Cpu() == resource.Quantity{}) {
		millicpu = defaultMilliCPURequest
	} else {
		millicpu = requests.Cpu().MilliValue()
	}
	// Override if un-set, but not if explicitly set to zero
	if (*requests.Memory() == resource.Quantity{}) {
		memory = defaultMemoryRequest
	} else {
		memory = requests.Memory().Value()
	}
	return millicpu, memory
}

func getResourcesForPacking(resources *api.ResourceRequirements) (int64, int64) {
	rc, rm := getNonzeroRequests(&resources.Requests)
	lc, lm := getNonzeroRequests(&resources.Limits)

	glog.V(10).Infof("Requests: (%d, %d)  Limits: (%d, %d)", rc, rm, lc, lm)

	return int64(math.Max(float64(rc), float64(lc))), int64(math.Max(float64(rm), float64(lm)))
}

// Calculate the resource occupancy on a node.  'node' has information about the resources on the node.
// 'pods' is a list of pods currently scheduled on the node.
func calculateResourceOccupancy(pod *api.Pod, node api.Node, pods []*api.Pod) algorithm.HostPriority {
	totalMilliCPU := int64(0)
	totalMemory := int64(0)
	capacityMilliCPU := node.Status.Capacity.Cpu().MilliValue()
	capacityMemory := node.Status.Capacity.Memory().Value()

	for _, existingPod := range pods {
		for _, container := range existingPod.Spec.Containers {
			cpu, memory := getResourcesForPacking(&container.Resources)
			totalMilliCPU += cpu
			totalMemory += memory
		}
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

	return algorithm.HostPriority{
		Host:  node.Name,
		Score: score,
	}
}
