package algorithm

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

const (
	NodeDiskFull    string = "DiskFull"
	NodeDiskFine    string = "DiskFine"
	NodeDeisApps    string = "DeisApps"
	NodeNonDeisApps string = "NoDeisApps"
)

type testNodeInfo struct {
	nodes []*api.Node
}

func createDiskNode(name string, diskStatus api.ConditionStatus) *api.Node {
	return &api.Node{
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				api.NodeCondition{
					Type:   api.NodeOutOfDisk,
					Status: diskStatus,
				},
			},
		},
	}
}

func (i *testNodeInfo) GetNodeInfo(id string) (*api.Node, error) {
	for _, n := range i.nodes {
		if n.Name == id {
			return n, nil
		}
	}

	return nil, fmt.Errorf("Could not find node: %s", id)
}

func newTestNodeInfo(nodes []*api.Node) predicates.NodeInfo {
	return &testNodeInfo{
		nodes: nodes,
	}
}

func TestNodeDisk(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{
			name:     NodeDiskFine,
			expected: true,
		},
		{
			name:     NodeDiskFull,
			expected: false,
		},
	}
	pred := NewNodeOutOfDiskPredicate(newTestNodeInfo([]*api.Node{
		createDiskNode(NodeDiskFull, api.ConditionTrue),
		createDiskNode(NodeDiskFine, api.ConditionFalse),
	}))

	for _, test := range tests {
		actual, err := pred(&api.Pod{}, test.name, schedulercache.NewNodeInfo())
		if err != nil {
			t.Error("Error from predicate: ", err)
		}

		if actual != test.expected {
			t.Errorf("Expected: %t, Got: %t", test.expected, actual)
		}
	}
}

func createDeisPod(version string) *api.Pod {
	return &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Labels: map[string]string{
				"app":      "testApp",
				"heritage": "deis",
				"version":  version,
			},
		},
	}
}

func TestDeisPredicate(t *testing.T) {
	tests := []struct {
		testName string
		deisPod  *api.Pod
		cache    *schedulercache.NodeInfo
		expected bool
	}{
		{
			testName: "NoDeisApps",
			deisPod:  createDeisPod("v1"),
			cache:    schedulercache.NewNodeInfo(&api.Pod{}),
			expected: true,
		},
		{
			testName: "SameVersion",
			deisPod:  createDeisPod("v1"),
			cache:    schedulercache.NewNodeInfo(createDeisPod("v1")),
			expected: false,
		},
		{
			testName: "DifferentVersion",
			deisPod:  createDeisPod("v2"),
			cache:    schedulercache.NewNodeInfo(createDeisPod("v1")),
			expected: true,
		},
		{
			testName: "EmptyPod",
			deisPod:  &api.Pod{},
			cache:    schedulercache.NewNodeInfo(createDeisPod("v1")),
			expected: true,
		},
	}

	for _, test := range tests {
		actual, err := UniqueDeisApp(test.deisPod, "Node", test.cache)

		if err != nil {
			t.Errorf("Test %s had error %v", test.testName, err)
		}

		if actual != test.expected {
			t.Errorf("Test %s. Expected: %v Actual: %v", test.testName, test.expected, actual)
		}
	}
}
