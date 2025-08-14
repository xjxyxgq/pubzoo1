package cluster

import (
	"testing"
	"time"
)

func TestGetDownstreamNodesRecursive(t *testing.T) {
	// 创建测试拓扑
	master := &NodeConfig{
		Host: "master.db",
		Port: 3306,
	}
	
	slave1 := &NodeConfig{
		Host: "slave1.db",
		Port: 3306,
	}
	
	slave2 := &NodeConfig{
		Host: "slave2.db",
		Port: 3306,
	}
	
	slave3 := &NodeConfig{
		Host: "slave3.db",
		Port: 3306,
	}
	
	// 构建拓扑关系: master -> slave1 -> slave2
	//                         -> slave3
	topology := &ClusterTopology{
		MasterNode: master,
		SlaveNodes: []*NodeConfig{slave1, slave2, slave3},
		Relationships: map[string][]string{
			master.NodeKey(): {slave1.NodeKey()},
			slave1.NodeKey(): {slave2.NodeKey(), slave3.NodeKey()},
		},
		DiscoveredAt: time.Now(),
	}
	
	tests := []struct {
		name           string
		startNode      *NodeConfig
		expectedKeys   []string
		description    string
	}{
		{
			name:      "从主节点开始",
			startNode: master,
			expectedKeys: []string{
				master.NodeKey(),
				slave1.NodeKey(),
				slave2.NodeKey(),
				slave3.NodeKey(),
			},
			description: "应该返回主节点及其所有下游节点",
		},
		{
			name:      "从slave1开始",
			startNode: slave1,
			expectedKeys: []string{
				slave1.NodeKey(),
				slave2.NodeKey(),
				slave3.NodeKey(),
			},
			description: "应该返回slave1及其下游节点",
		},
		{
			name:      "从slave2开始",
			startNode: slave2,
			expectedKeys: []string{
				slave2.NodeKey(),
			},
			description: "应该只返回slave2自身（没有下游）",
		},
		{
			name:      "从slave3开始",
			startNode: slave3,
			expectedKeys: []string{
				slave3.NodeKey(),
			},
			description: "应该只返回slave3自身（没有下游）",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := topology.GetDownstreamNodesRecursive(tt.startNode)
			
			// 验证返回的节点数量
			if len(result) != len(tt.expectedKeys) {
				t.Errorf("GetDownstreamNodesRecursive() 返回了 %d 个节点，期望 %d 个",
					len(result), len(tt.expectedKeys))
			}
			
			// 验证每个期望的节点都在结果中
			resultKeys := make(map[string]bool)
			for _, node := range result {
				resultKeys[node.NodeKey()] = true
			}
			
			for _, expectedKey := range tt.expectedKeys {
				if !resultKeys[expectedKey] {
					t.Errorf("期望的节点 %s 不在结果中", expectedKey)
				}
			}
			
			// 验证没有重复节点
			if len(result) != len(resultKeys) {
				t.Errorf("结果中存在重复节点")
			}
		})
	}
}

func TestFilterNodesByStartNode(t *testing.T) {
	// 创建测试拓扑
	master := &NodeConfig{
		Host: "master.db",
		Port: 3306,
	}
	
	slave1 := &NodeConfig{
		Host: "slave1.db",
		Port: 3306,
	}
	
	slave2 := &NodeConfig{
		Host: "slave2.db",
		Port: 3306,
	}
	
	topology := &ClusterTopology{
		MasterNode: master,
		SlaveNodes: []*NodeConfig{slave1, slave2},
		Relationships: map[string][]string{
			master.NodeKey(): {slave1.NodeKey()},
			slave1.NodeKey(): {slave2.NodeKey()},
		},
		DiscoveredAt: time.Now(),
	}
	
	tests := []struct {
		name         string
		startNodeKey string
		expectedLen  int
		description  string
	}{
		{
			name:         "空字符串返回所有节点",
			startNodeKey: "",
			expectedLen:  3, // master + 2 slaves
			description:  "当startNodeKey为空时，应该返回所有节点",
		},
		{
			name:         "从主节点开始",
			startNodeKey: master.NodeKey(),
			expectedLen:  3, // master + 2 downstream slaves
			description:  "从主节点开始应该返回所有节点",
		},
		{
			name:         "从slave1开始",
			startNodeKey: slave1.NodeKey(),
			expectedLen:  2, // slave1 + slave2
			description:  "从slave1开始应该返回slave1和slave2",
		},
		{
			name:         "从slave2开始",
			startNodeKey: slave2.NodeKey(),
			expectedLen:  1, // 只有slave2
			description:  "从slave2开始应该只返回slave2",
		},
		{
			name:         "不存在的节点",
			startNodeKey: "nonexistent:3306",
			expectedLen:  0,
			description:  "不存在的节点应该返回空结果",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := topology.FilterNodesByStartNode(tt.startNodeKey)
			
			if len(result) != tt.expectedLen {
				t.Errorf("FilterNodesByStartNode() 返回了 %d 个节点，期望 %d 个",
					len(result), tt.expectedLen)
			}
			
			// 验证结果中没有重复节点
			seen := make(map[string]bool)
			for _, node := range result {
				key := node.NodeKey()
				if seen[key] {
					t.Errorf("结果中存在重复节点: %s", key)
				}
				seen[key] = true
			}
		})
	}
}

func TestComplexTopologyFilter(t *testing.T) {
	// 创建更复杂的拓扑结构
	// master -> slave1 -> slave1a
	//        -> slave2 -> slave2a
	//                 -> slave2b
	
	master := &NodeConfig{Host: "master.db", Port: 3306}
	slave1 := &NodeConfig{Host: "slave1.db", Port: 3306}
	slave2 := &NodeConfig{Host: "slave2.db", Port: 3306}
	slave1a := &NodeConfig{Host: "slave1a.db", Port: 3306}
	slave2a := &NodeConfig{Host: "slave2a.db", Port: 3306}
	slave2b := &NodeConfig{Host: "slave2b.db", Port: 3306}
	
	topology := &ClusterTopology{
		MasterNode: master,
		SlaveNodes: []*NodeConfig{slave1, slave2, slave1a, slave2a, slave2b},
		Relationships: map[string][]string{
			master.NodeKey():  {slave1.NodeKey(), slave2.NodeKey()},
			slave1.NodeKey():  {slave1a.NodeKey()},
			slave2.NodeKey():  {slave2a.NodeKey(), slave2b.NodeKey()},
		},
		DiscoveredAt: time.Now(),
	}
	
	// 测试从slave2开始的过滤
	result := topology.FilterNodesByStartNode(slave2.NodeKey())
	expected := 3 // slave2, slave2a, slave2b
	
	if len(result) != expected {
		t.Errorf("复杂拓扑从slave2开始，期望 %d 个节点，实际获得 %d 个", expected, len(result))
	}
	
	// 验证包含正确的节点
	expectedNodes := map[string]bool{
		slave2.NodeKey():  false,
		slave2a.NodeKey(): false,
		slave2b.NodeKey(): false,
	}
	
	for _, node := range result {
		key := node.NodeKey()
		if _, exists := expectedNodes[key]; exists {
			expectedNodes[key] = true
		} else {
			t.Errorf("意外的节点在结果中: %s", key)
		}
	}
	
	// 验证所有期望的节点都被找到
	for key, found := range expectedNodes {
		if !found {
			t.Errorf("期望的节点 %s 未在结果中找到", key)
		}
	}
}