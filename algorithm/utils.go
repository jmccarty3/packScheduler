package algorithm

import (
	"math"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
)

// For each of these resources, a pod that doesn't request the resource explicitly
// will be treated as having requested the amount indicated below, for the purpose
// of computing priority only. This ensures that when scheduling zero-request pods, such
// pods will not all be scheduled to the machine with the smallest in-use request,
// and that when scheduling regular pods, such pods will not see zero-request pods as
// consuming no resources whatsoever. We chose these values to be similar to the
// resources that we give to cluster addon pods (#10653). But they are pretty arbitrary.
// As described in #11713, we use request instead of limit to deal with resource requirements.
const defaultMilliCPURequest int64 = 250             // 0.25 core
const defaultMemoryRequest int64 = 500 * 1024 * 1024 // 500 MB

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

func getResourcesForPod(pod *api.Pod) (int64, int64) {
	totalCPU := int64(0)
	totalMemory := int64(0)
	for _, container := range pod.Spec.Containers {
		cpu, memory := getResourcesForPacking(&container.Resources)
		totalCPU += cpu
		totalMemory += memory
	}

	return totalCPU, totalMemory
}
