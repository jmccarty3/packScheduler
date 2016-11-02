package algorithm

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

func makeNode(node string, milliCPU, memory int64) *api.Node {
	return &api.Node{
		ObjectMeta: api.ObjectMeta{Name: node},
		Status: api.NodeStatus{
			Capacity: api.ResourceList{
				"cpu":    *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				"memory": *resource.NewQuantity(memory, resource.BinarySI),
			},
		},
	}
}

func makeResourceRequirements(rc, rm, lc, lm int64) api.ResourceRequirements {
	return api.ResourceRequirements{
		Requests: api.ResourceList{
			api.ResourceCPU:    *resource.NewMilliQuantity(rc, resource.DecimalSI),
			api.ResourceMemory: *resource.NewQuantity(rm, resource.BinarySI),
		},
		Limits: api.ResourceList{
			api.ResourceCPU:    *resource.NewMilliQuantity(lc, resource.DecimalSI),
			api.ResourceMemory: *resource.NewQuantity(lm, resource.BinarySI),
		},
	}
}

func TestGetResourcesForPacking(t *testing.T) {
	cpu := int64(1000)
	memory := int64(2000)

	tests := []struct {
		test      string
		resources api.ResourceRequirements
	}{
		{
			test:      "RequestOnly",
			resources: makeResourceRequirements(cpu, memory, 0, 0),
		},
		{
			test:      "LimitOnly",
			resources: makeResourceRequirements(0, 0, cpu, memory),
		},
		{
			test:      "ReqCpuLimitMem",
			resources: makeResourceRequirements(cpu, 0, 0, memory),
		},
		{
			test:      "ReqMemLimitCpu",
			resources: makeResourceRequirements(0, memory, cpu, memory),
		},
	}

	for _, test := range tests {
		if ac, am := getResourcesForPacking(&test.resources); ac != cpu || am != memory {
			t.Errorf("Test: %s  Expected: (%d, %d)  Actual: (%d, %d)", test.test, cpu, memory, ac, am)
		}
	}
}

func TestMostRequested(t *testing.T) {
	labels1 := map[string]string{
		"foo": "bar",
		"baz": "blah",
	}
	labels2 := map[string]string{
		"bar": "foo",
		"baz": "blah",
	}
	machine1Spec := api.PodSpec{
		NodeName: "machine1",
	}
	machine2Spec := api.PodSpec{
		NodeName: "machine2",
	}
	noResources := api.PodSpec{
		Containers: []api.Container{},
	}
	cpuOnly := api.PodSpec{
		NodeName: "machine1",
		Containers: []api.Container{
			{
				Resources: api.ResourceRequirements{
					Requests: api.ResourceList{
						"cpu":    resource.MustParse("1000m"),
						"memory": resource.MustParse("0"),
					},
					Limits: api.ResourceList{
						"cpu":    resource.MustParse("0"),
						"memory": resource.MustParse("0"),
					},
				},
			},
			{
				Resources: api.ResourceRequirements{
					Requests: api.ResourceList{
						"cpu":    resource.MustParse("2000m"),
						"memory": resource.MustParse("0"),
					},
					Limits: api.ResourceList{
						"cpu":    resource.MustParse("0"),
						"memory": resource.MustParse("0"),
					},
				},
			},
		},
	}
	cpuOnly2 := cpuOnly
	cpuOnly2.NodeName = "machine2"
	cpuAndMemory := api.PodSpec{
		NodeName: "machine2",
		Containers: []api.Container{
			{
				Resources: api.ResourceRequirements{
					Requests: api.ResourceList{
						"cpu":    resource.MustParse("1000m"),
						"memory": resource.MustParse("2000"),
					},
					Limits: api.ResourceList{
						"cpu":    resource.MustParse("0"),
						"memory": resource.MustParse("0"),
					},
				},
			},
			{
				Resources: api.ResourceRequirements{
					Requests: api.ResourceList{
						"cpu":    resource.MustParse("2000m"),
						"memory": resource.MustParse("3000"),
					},
					Limits: api.ResourceList{
						"cpu":    resource.MustParse("0"),
						"memory": resource.MustParse("0"),
					},
				},
			},
		},
	}
	tests := []struct {
		pod          *api.Pod
		pods         []*api.Pod
		nodes        []*api.Node
		expectedList schedulerapi.HostPriorityList
		test         string
	}{
		{
			/*
				Node1 scores (remaining resources) on 0-10 scale
				CPU Score: 11 - ((4000 - 0) *10) / 4000 = 1
				Memory Score: 11 - ((10000 - 0) *10) / 10000 = 1
				Node1 Score: (1 + 1) / 2 = 1
				Node2 scores (remaining resources) on 0-10 scale
				CPU Score: 11 - ((4000 - 0) *10) / 4000 = 1
				Memory Score: 11 - ((10000 - 0) *10) / 10000 = 1
				Node2 Score: (1 + 1) / 2 = 1
			*/
			pod:          &api.Pod{Spec: noResources},
			nodes:        []*api.Node{makeNode("machine1", 4000, 10000), makeNode("machine2", 4000, 10000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 1}, {Host: "machine2", Score: 1}},
			test:         "nothing scheduled, nothing requested",
		},
		{
			/*
				Node1 scores on 0-10 scale
				CPU Score: 11 - ((4000 - 3000) *10) / 4000 = 8.5
				Memory Score: 11 - ((10000 - 5000) *10) / 10000 = 6
				Node1 Score: (8.5 + 6) / 2 = 7
				Node2 scores on 0-10 scale
				CPU Score: 11 - ((6000 - 3000) *10) / 6000 = 6
				Memory Score: 11 - ((10000 - 5000) *10) / 10000 = 6
				Node2 Score: (6 + 6) / 2 = 6
			*/
			pod:          &api.Pod{Spec: cpuAndMemory},
			nodes:        []*api.Node{makeNode("machine1", 4000, 10000), makeNode("machine2", 6000, 10000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 7}, {Host: "machine2", Score: 6}},
			test:         "nothing scheduled, resources requested, differently sized machines",
		},
		{
			/*
				Node1 scores on 0-10 scale
				CPU Score: 11 - ((4000 - 0) *10) / 4000 = 1
				Memory Score: 11 - ((10000 - 0) *10) / 10000 = 1
				Node1 Score: (1 + 1) / 2 = 1
				Node2 scores on 0-10 scale
				CPU Score: 11 - ((4000 - 0) *10) / 4000 = 1
				Memory Score: 11 - ((10000 - 0) *10) / 10000 = 1
				Node2 Score: (1 + 1) / 2 = 1
			*/
			pod:          &api.Pod{Spec: noResources},
			nodes:        []*api.Node{makeNode("machine1", 4000, 10000), makeNode("machine2", 4000, 10000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 1}, {Host: "machine2", Score: 1}},
			test:         "no resources requested, pods scheduled",
			pods: []*api.Pod{
				{Spec: machine1Spec, ObjectMeta: api.ObjectMeta{Labels: labels2}},
				{Spec: machine1Spec, ObjectMeta: api.ObjectMeta{Labels: labels1}},
				{Spec: machine2Spec, ObjectMeta: api.ObjectMeta{Labels: labels1}},
				{Spec: machine2Spec, ObjectMeta: api.ObjectMeta{Labels: labels1}},
			},
		},
		{
			/*
				Node1 scores on 0-10 scale
				CPU Score: 11 - ((10000 - 6000) *10) / 10000 = 7
				Memory Score: 11 - ((20000 - 0) *10) / 20000 = 1
				Node1 Score: (7 + 1) / 2 = 4
				Node2 scores on 0-10 scale
				CPU Score: 11 - ((10000 - 6000) *10) / 10000 = 7
				Memory Score: 11 - ((20000 - 5000) *10) / 20000 = 3.5
				Node2 Score: (4 + 3.5) / 2 = 5
			*/
			pod:          &api.Pod{Spec: noResources},
			nodes:        []*api.Node{makeNode("machine1", 10000, 20000), makeNode("machine2", 10000, 20000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 4}, {Host: "machine2", Score: 5}},
			test:         "no resources requested, pods scheduled with resources",
			pods: []*api.Pod{
				{Spec: cpuOnly, ObjectMeta: api.ObjectMeta{Labels: labels2}},
				{Spec: cpuOnly, ObjectMeta: api.ObjectMeta{Labels: labels1}},
				{Spec: cpuOnly2, ObjectMeta: api.ObjectMeta{Labels: labels1}},
				{Spec: cpuAndMemory, ObjectMeta: api.ObjectMeta{Labels: labels1}},
			},
		},
		{
			/*
				Node1 scores on 0-10 scale
				CPU Score: 11 - ((10000 - 6000) *10) / 10000 = 7
				Memory Score: 11 - ((20000 - 5000) *10) / 20000 = 3.5
				Node1 Score: (7 + 3.5) / 2 = 5
				Node2 scores on 0-10 scale
				CPU Score: 11 - ((10000 - 6000) *10) / 10000 = 7
				Memory Score: 11 - ((20000 - 10000) *10) / 20000 = 6
				Node2 Score: (7 + 6) / 2 = 6
			*/
			pod:          &api.Pod{Spec: cpuAndMemory},
			nodes:        []*api.Node{makeNode("machine1", 10000, 20000), makeNode("machine2", 10000, 20000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 5}, {Host: "machine2", Score: 6}},
			test:         "resources requested, pods scheduled with resources",
			pods: []*api.Pod{
				{Spec: cpuOnly},
				{Spec: cpuAndMemory},
			},
		},
		{
			/*
				Node1 scores on 0-10 scale
				CPU Score: 11 - ((10000 - 6000) *10) / 10000 = 7
				Memory Score: 11 - ((20000 - 5000) *10) / 20000 = 3.5
				Node1 Score: (7 + 3.5) / 2 = 5
				Node2 scores on 0-10 scale
				CPU Score: 11 - ((10000 - 6000) *10) / 10000 = 7
				Memory Score: 11 - ((50000 - 10000) *10) / 50000 = 3
				Node2 Score: (7 + 3) / 2 = 5
			*/
			pod:          &api.Pod{Spec: cpuAndMemory},
			nodes:        []*api.Node{makeNode("machine1", 10000, 20000), makeNode("machine2", 10000, 50000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 5}, {Host: "machine2", Score: 5}},
			test:         "resources requested, pods scheduled with resources, differently sized machines",
			pods: []*api.Pod{
				{Spec: cpuOnly},
				{Spec: cpuAndMemory},
			},
		},
		{
			/*
				Node1 scores on 0-10 scale
				CPU Score: ((4000 - 6000) *10) / 4000 = 0
				Memory Score: 11 - ((10000 - 0) *10) / 10000 = 1
				Node1 Score: (0 + 1) / 2 = 0
				Node2 scores on 0-10 scale
				CPU Score: ((4000 - 6000) *10) / 4000 = 0
				Memory Score: 11 - ((10000 - 5000) *10) / 10000 = 6
				Node2 Score: (0 + 6) / 2 = 3
			*/
			pod:          &api.Pod{Spec: cpuOnly},
			nodes:        []*api.Node{makeNode("machine1", 4000, 10000), makeNode("machine2", 4000, 10000)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 0}, {Host: "machine2", Score: 0}},
			test:         "requested resources exceed node capacity",
			pods: []*api.Pod{
				{Spec: cpuOnly},
				{Spec: cpuAndMemory},
			},
		},
		{
			pod:          &api.Pod{Spec: noResources},
			nodes:        []*api.Node{makeNode("machine1", 0, 0), makeNode("machine2", 0, 0)},
			expectedList: []schedulerapi.HostPriority{{Host: "machine1", Score: 0}, {Host: "machine2", Score: 0}},
			test:         "zero node resources, pods scheduled with resources",
			pods: []*api.Pod{
				{Spec: cpuOnly},
				{Spec: cpuAndMemory},
			},
		},
	}

	for _, test := range tests {
		list, err := MostRequestedPriority(test.pod, schedulercache.CreateNodeNameToInfoMap(test.pods, test.nodes), test.nodes)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(test.expectedList, list) {
			t.Errorf("%s: expected %#v, got %#v", test.test, test.expectedList, list)
		}
	}
}
