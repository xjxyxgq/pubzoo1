package cluster

import (
	"fmt"
	"time"
)

// NodeConfig MySQL节点配置
type NodeConfig struct {
	Host     string        `json:"host" yaml:"host"`
	Port     int           `json:"port" yaml:"port"`
	Username string        `json:"username" yaml:"username"`
	Password string        `json:"password" yaml:"password"`
	Database string        `json:"database" yaml:"database"`
	Timeout  time.Duration `json:"timeout" yaml:"timeout"`
	
	// 扩展超时配置
	ConnectTimeout   time.Duration `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty"`
	QueryTimeout     time.Duration `json:"query_timeout,omitempty" yaml:"query_timeout,omitempty"`
	AlterTimeout     time.Duration `json:"alter_timeout,omitempty" yaml:"alter_timeout,omitempty"`
	DiscoveryTimeout time.Duration `json:"discovery_timeout,omitempty" yaml:"discovery_timeout,omitempty"`
}

// NodeStatus 节点状态信息
type NodeStatus struct {
	IsMaster    bool          `json:"is_master"`
	IsReadOnly  bool          `json:"is_read_only"`
	Role        string        `json:"role"`
	SlaveStatus *SlaveStatus  `json:"slave_status,omitempty"`
	SlaveHosts  []string      `json:"slave_hosts,omitempty"`
	LastCheck   time.Time     `json:"last_check"`
	ServerID    uint32        `json:"server_id"`
}

// SlaveStatus 从库状态信息
type SlaveStatus struct {
	MasterHost             string `json:"master_host"`
	MasterPort             int    `json:"master_port"`
	MasterUser             string `json:"master_user"`
	SlaveIORunning         string `json:"slave_io_running"`
	SlaveSQLRunning        string `json:"slave_sql_running"`
	SecondsBehindMaster    *int   `json:"seconds_behind_master,omitempty"`
	MasterLogFile          string `json:"master_log_file"`
	ReadMasterLogPos       uint64 `json:"read_master_log_pos"`
	RelayLogFile           string `json:"relay_log_file"`
	RelayLogPos            uint64 `json:"relay_log_pos"`
	RelayMasterLogFile     string `json:"relay_master_log_file"`
	SlaveIOState           string `json:"slave_io_state"`
	ReplicationLag         int    `json:"replication_lag"`
}

// SlaveHost 从库主机信息
type SlaveHost struct {
	ServerID uint32 `json:"server_id"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user,omitempty"`
}

// ClusterTopology 集群拓扑结构
type ClusterTopology struct {
	MasterNode    *NodeConfig            `json:"master_node"`
	SlaveNodes    []*NodeConfig          `json:"slave_nodes"`
	Relationships map[string][]string    `json:"relationships"`
	NodeStatuses  map[string]*NodeStatus `json:"node_statuses"`
	DiscoveredAt  time.Time              `json:"discovered_at"`
	TotalNodes    int                    `json:"total_nodes"`
}

// ExternalNodeInfo 外部API返回的节点信息
type ExternalNodeInfo struct {
	ClusterName      string `json:"cluster_name"`
	ClusterGroupName string `json:"cluster_group_name"`
	InstanceRole     string `json:"instance_role"`
	InstanceReadOnly string `json:"instance_read_only"`
}

// ExternalAPIResponse 外部API响应格式
type ExternalAPIResponse struct {
	Status string            `json:"status"`
	Data   *ExternalNodeInfo `json:"data"`
}

// DiscoveryOptions 拓扑发现选项
type DiscoveryOptions struct {
	MaxConcurrency    int           `json:"max_concurrency"`
	Timeout           time.Duration `json:"timeout"`
	MaxDepth          int           `json:"max_depth"`
	EnableExternalAPI bool          `json:"enable_external_api"`
	ExternalAPIURL    string        `json:"external_api_url"`
}

// NodeKey 生成节点的唯一标识
func (n *NodeConfig) NodeKey() string {
	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}

// IsValid 检查节点配置是否有效
func (n *NodeConfig) IsValid() bool {
	return n.Host != "" && n.Port > 0 && n.Username != ""
}

// GetHost 获取主机地址
func (n *NodeConfig) GetHost() string {
	return n.Host
}

// GetPort 获取端口号
func (n *NodeConfig) GetPort() int {
	return n.Port
}

// GetUsername 获取用户名
func (n *NodeConfig) GetUsername() string {
	return n.Username
}

// GetPassword 获取密码
func (n *NodeConfig) GetPassword() string {
	return n.Password
}

// GetDatabase 获取数据库名
func (n *NodeConfig) GetDatabase() string {
	return n.Database
}

// GetTimeout 获取超时时间
func (n *NodeConfig) GetTimeout() time.Duration {
	return n.Timeout
}

// IsMasterRole 检查是否为主节点角色
func (s *NodeStatus) IsMasterRole() bool {
	return s.IsMaster && !s.IsReadOnly && s.SlaveStatus == nil
}

// HasSlaves 检查是否有从库
func (s *NodeStatus) HasSlaves() bool {
	return len(s.SlaveHosts) > 0
}

// IsHealthy 检查从库复制状态是否健康
func (s *SlaveStatus) IsHealthy() bool {
	return s.SlaveIORunning == "Yes" && s.SlaveSQLRunning == "Yes"
}

// GetReplicationDelay 获取复制延迟
func (s *SlaveStatus) GetReplicationDelay() int {
	if s.SecondsBehindMaster == nil {
		return -1
	}
	return *s.SecondsBehindMaster
}

// AddNode 添加节点到拓扑
func (t *ClusterTopology) AddNode(node *NodeConfig, status *NodeStatus) {
	key := node.NodeKey()
	
	if status.IsMasterRole() {
		t.MasterNode = node
	} else {
		t.SlaveNodes = append(t.SlaveNodes, node)
	}
	
	if t.NodeStatuses == nil {
		t.NodeStatuses = make(map[string]*NodeStatus)
	}
	t.NodeStatuses[key] = status
	
	t.TotalNodes = len(t.NodeStatuses)
}

// GetNodeStatus 获取节点状态
func (t *ClusterTopology) GetNodeStatus(node *NodeConfig) *NodeStatus {
	if t.NodeStatuses == nil {
		return nil
	}
	return t.NodeStatuses[node.NodeKey()]
}

// FindMasterNode 查找主节点
func (t *ClusterTopology) FindMasterNode() *NodeConfig {
	return t.MasterNode
}

// GetSlaveNodes 获取所有从节点
func (t *ClusterTopology) GetSlaveNodes() []*NodeConfig {
	return t.SlaveNodes
}

// GetDownstreamNodes 获取指定节点的下游节点
func (t *ClusterTopology) GetDownstreamNodes(node *NodeConfig) []*NodeConfig {
	key := node.NodeKey()
	downstreams, exists := t.Relationships[key]
	if !exists {
		return nil
	}
	
	var nodes []*NodeConfig
	for _, downstream := range downstreams {
		for _, slave := range t.SlaveNodes {
			if slave.NodeKey() == downstream {
				nodes = append(nodes, slave)
				break
			}
		}
	}
	return nodes
}

// GetDownstreamNodesRecursive 递归获取指定节点的所有下游节点（包括节点本身）
func (t *ClusterTopology) GetDownstreamNodesRecursive(startNode *NodeConfig) []*NodeConfig {
	visited := make(map[string]bool)
	var result []*NodeConfig
	
	// 递归函数
	var traverse func(node *NodeConfig)
	traverse = func(node *NodeConfig) {
		key := node.NodeKey()
		if visited[key] {
			return
		}
		visited[key] = true
		result = append(result, node)
		
		// 获取直接下游节点
		downstreams := t.GetDownstreamNodes(node)
		for _, downstream := range downstreams {
			traverse(downstream)
		}
	}
	
	traverse(startNode)
	return result
}

// FilterNodesByStartNode 根据起始节点过滤节点列表，只返回起始节点及其下游节点
func (t *ClusterTopology) FilterNodesByStartNode(startNodeKey string) []*NodeConfig {
	if startNodeKey == "" {
		// 如果没有指定起始节点，返回所有节点
		allNodes := []*NodeConfig{}
		if t.MasterNode != nil {
			allNodes = append(allNodes, t.MasterNode)
		}
		allNodes = append(allNodes, t.SlaveNodes...)
		return allNodes
	}
	
	// 找到起始节点
	var startNode *NodeConfig
	if t.MasterNode != nil && t.MasterNode.NodeKey() == startNodeKey {
		startNode = t.MasterNode
	} else {
		for _, slave := range t.SlaveNodes {
			if slave.NodeKey() == startNodeKey {
				startNode = slave
				break
			}
		}
	}
	
	if startNode == nil {
		return nil
	}
	
	return t.GetDownstreamNodesRecursive(startNode)
}