package algorithm

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

const (
	NodeDiskFull string = "DiskFull"
	NodeDiskFine string = "DiskFine"
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

func newTestNodeInfo() predicates.NodeInfo {
	return &testNodeInfo{
		nodes: []*api.Node{
			createDiskNode(NodeDiskFull, api.ConditionTrue),
			createDiskNode(NodeDiskFine, api.ConditionFalse),
		},
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
	pred := NewNodeOutOfDiskPredicate(newTestNodeInfo())

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
