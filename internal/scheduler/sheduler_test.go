package scheduler

import (
	"github.com/MarouaneBouaricha/cube/api/node"
	"github.com/MarouaneBouaricha/cube/internal/task"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var nodeList = []*node.Node{
	&node.Node{Name: "test-node-1", Memory: 33554432, MemoryAllocated: 8388608, Disk: 524288000, DiskAllocated: 104857600},
	&node.Node{Name: "test-node-2", Memory: 33554432, MemoryAllocated: 16777216, Disk: 524288000, DiskAllocated: 262144000},
	&node.Node{Name: "test-node-3", Memory: 33554432, MemoryAllocated: 30408704, Disk: 524288000, DiskAllocated: 262144000},
}

func TestRoundRobinSchedulerSelectCandidateNodes(t *testing.T) {
	rrs := RoundRobin{"test-rr-scheduler", 0}

	tt := task.Task{}
	nodeList := nodeList
	got := rrs.SelectCandidateNodes(tt, nodeList)

	if !cmp.Equal(got, nodeList) {
		t.Errorf("-want/+got: \n%s", cmp.Diff(nodeList, got))
	}
}

func TestRoundRobinSchedulerScoreCandidateNodes(t *testing.T) {
	tests := []struct {
		name       string
		lastWorker int
		want       map[string]float64
	}{
		{
			name:       "node 2 scored lowest",
			lastWorker: 0,
			want: map[string]float64{
				"test-node-1": 1.0,
				"test-node-2": 0.1,
				"test-node-3": 1.0,
			},
		},
		{
			name:       "node 3 scored lowest",
			lastWorker: 1,
			want: map[string]float64{
				"test-node-1": 1.0,
				"test-node-2": 1.0,
				"test-node-3": 0.1,
			},
		},
		{
			name:       "node 0 scored lowest",
			lastWorker: 2,
			want: map[string]float64{
				"test-node-1": 0.1,
				"test-node-2": 1.0,
				"test-node-3": 1.0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rrs := RoundRobin{"test-rr-scheduler", test.lastWorker}
			task1 := task.Task{}

			candidateNodes := nodeList
			got := rrs.Score(task1, candidateNodes)
			if !cmp.Equal(got, test.want) {
				t.Errorf("-want/+got: \n%s", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestRoundRobinSchedulerPickBestNode(t *testing.T) {
	tests := []struct {
		name           string
		candidateNodes []*node.Node
		scores         map[string]float64
		want           int
	}{
		{
			name:           "pick node 1 from scored candidates",
			candidateNodes: nodeList,
			scores: map[string]float64{
				"test-node-1": 0.1,
				"test-node-2": 1.0,
				"test-node-3": 1.0,
			},
			want: 0,
		},
		{
			name:           "pick node 2 from scored candidates",
			candidateNodes: nodeList,
			scores: map[string]float64{
				"test-node-1": 1.0,
				"test-node-2": 0.1,
				"test-node-3": 1.0,
			},
			want: 1,
		},
		{
			name:           "pick node 3 from scored candidates",
			candidateNodes: nodeList,
			scores: map[string]float64{
				"test-node-1": 1.0,
				"test-node-2": 1.0,
				"test-node-3": 0.1,
			},
			want: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rrs := RoundRobin{"test-rr-scheduler", 0}

			got := rrs.Pick(test.scores, test.candidateNodes)
			if !cmp.Equal(got, test.candidateNodes[test.want]) {
				t.Errorf("-want/+got: \n%s", cmp.Diff(test.candidateNodes[test.want], got))
			}
		})
	}
}
