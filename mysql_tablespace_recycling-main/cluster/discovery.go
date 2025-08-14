package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TopologyDiscoverer 拓扑发现器接口
type TopologyDiscoverer interface {
	DiscoverTopology(ctx context.Context, entryPoint *NodeConfig) (*ClusterTopology, error)
	ValidateNode(ctx context.Context, node *NodeConfig) (*NodeStatus, error)
	GetSlaveHosts(ctx context.Context, node *NodeConfig) ([]*SlaveHost, error)
	GetSlaveStatus(ctx context.Context, node *NodeConfig) (*SlaveStatus, error)
}

// DefaultTopologyDiscoverer 默认拓扑发现器实现
type DefaultTopologyDiscoverer struct {
	connectionFactory ConnectionFactory
	options          *DiscoveryOptions
	externalAPI      ExternalAPIClient
	mu               sync.RWMutex
	visitedNodes     map[string]bool
	discoveredNodes  map[string]*NodeConfig
}

// NewTopologyDiscoverer 创建拓扑发现器
func NewTopologyDiscoverer(factory ConnectionFactory, options *DiscoveryOptions) TopologyDiscoverer {
	if options == nil {
		options = &DiscoveryOptions{
			MaxConcurrency: 5,
			Timeout:        30 * time.Second,
			MaxDepth:       3,
		}
	}

	return &DefaultTopologyDiscoverer{
		connectionFactory: factory,
		options:          options,
		visitedNodes:     make(map[string]bool),
		discoveredNodes:  make(map[string]*NodeConfig),
	}
}

// DiscoverTopology 发现集群拓扑
func (d *DefaultTopologyDiscoverer) DiscoverTopology(ctx context.Context, entryPoint *NodeConfig) (*ClusterTopology, error) {
	if !entryPoint.IsValid() {
		return nil, fmt.Errorf("invalid entry point node config: %+v", entryPoint)
	}

	// 首先验证入口节点是否可连接
	if _, err := d.ValidateNode(ctx, entryPoint); err != nil {
		return nil, fmt.Errorf("failed to validate entry point: %w", err)
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, d.options.Timeout)
	defer cancel()

	topology := &ClusterTopology{
		Relationships: make(map[string][]string),
		NodeStatuses:  make(map[string]*NodeStatus),
		DiscoveredAt:  time.Now(),
	}

	// 并发发现拓扑
	if err := d.discoverConcurrently(ctx, entryPoint, topology, 0); err != nil {
		return nil, fmt.Errorf("failed to discover topology: %w", err)
	}

	// 分析拓扑关系，找出主节点
	if err := d.analyzeMasterSlave(topology); err != nil {
		return nil, fmt.Errorf("failed to analyze master-slave relationships: %w", err)
	}

	return topology, nil
}

// discoverConcurrently 并发发现拓扑
func (d *DefaultTopologyDiscoverer) discoverConcurrently(ctx context.Context, entryPoint *NodeConfig, topology *ClusterTopology, depth int) error {
	if depth >= d.options.MaxDepth {
		return nil
	}

	// 使用信号量控制并发数
	sem := make(chan struct{}, d.options.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 待处理的节点队列 - 使用带缓冲的通道
	nodeQueue := make(chan *NodeConfig, 100)
	nodeQueue <- entryPoint

	// 跟踪活跃的goroutines数量
	activeGoroutines := int32(0)

	// 启动worker goroutines
	for i := 0; i < d.options.MaxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case node, ok := <-nodeQueue:
					if !ok {
						// 队列已关闭
						return
					}
					if node == nil {
						return
					}
					
					sem <- struct{}{}
					atomic.AddInt32(&activeGoroutines, 1)
					
					if err := d.discoverSingleNode(ctx, node, topology, nodeQueue); err != nil {
						// 收集错误但不立即失败，让拓扑发现继续进行
						mu.Lock()
						errors = append(errors, err)
						mu.Unlock()
					}
					
					atomic.AddInt32(&activeGoroutines, -1)
					<-sem
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// 监控队列状态，当队列为空且没有活跃的goroutines时关闭队列
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(nodeQueue)
				return
			case <-time.After(100 * time.Millisecond):
				// 检查是否应该关闭队列
				if len(nodeQueue) == 0 && atomic.LoadInt32(&activeGoroutines) == 0 {
					close(nodeQueue)
					return
				}
			}
		}
	}()

	// 等待所有goroutines完成
	wg.Wait()

	// 检查错误
	mu.Lock()
	defer mu.Unlock()
	
	// 检查是否成功发现了节点
	discoveredNodes := len(topology.NodeStatuses)
	
	// 如果有错误但成功发现了一些节点，仍然认为是成功的
	if len(errors) > 0 && discoveredNodes == 0 {
		// 如果完全没有发现任何节点，返回第一个错误
		return errors[0]
	}
	
	// 否则认为发现成功（即使有一些节点失败）
	return nil
}

// discoverSingleNode 发现单个节点
func (d *DefaultTopologyDiscoverer) discoverSingleNode(ctx context.Context, node *NodeConfig, topology *ClusterTopology, nodeQueue chan<- *NodeConfig) error {
	nodeKey := node.NodeKey()

	// 检查是否已经访问过
	d.mu.Lock()
	if d.visitedNodes[nodeKey] {
		d.mu.Unlock()
		return nil
	}
	d.visitedNodes[nodeKey] = true
	d.discoveredNodes[nodeKey] = node
	d.mu.Unlock()

	// 验证节点状态
	status, err := d.ValidateNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to validate node %s: %w", nodeKey, err)
	}

	// 添加到拓扑
	topology.AddNode(node, status)

	// 获取从库列表
	slaveHosts, err := d.GetSlaveHosts(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to get slave hosts for %s: %w", nodeKey, err)
	}

	// 处理从库关系
	var downstreams []string
	for _, slave := range slaveHosts {
		slaveKey := fmt.Sprintf("%s:%d", slave.Host, slave.Port)
		downstreams = append(downstreams, slaveKey)

		// 创建从库节点配置
		slaveNode := &NodeConfig{
			Host:     slave.Host,
			Port:     slave.Port,
			Username: node.Username,
			Password: node.Password,
			Database: node.Database,
			Timeout:  node.Timeout,
		}

		// 将从库加入发现队列
		select {
		case nodeQueue <- slaveNode:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 队列满了，跳过
		}
	}

	if len(downstreams) > 0 {
		d.mu.Lock()
		topology.Relationships[nodeKey] = downstreams
		d.mu.Unlock()
	}

	// 获取上游主库信息
	slaveStatus, err := d.GetSlaveStatus(ctx, node)
	if err == nil && slaveStatus != nil {
		// 这是一个从库，发现上游主库
		masterKey := fmt.Sprintf("%s:%d", slaveStatus.MasterHost, slaveStatus.MasterPort)
		
		d.mu.Lock()
		if _, exists := d.discoveredNodes[masterKey]; !exists {
			// 主库还未发现，加入队列
			masterNode := &NodeConfig{
				Host:     slaveStatus.MasterHost,
				Port:     slaveStatus.MasterPort,
				Username: node.Username,
				Password: node.Password,
				Database: node.Database,
				Timeout:  node.Timeout,
			}
			
			select {
			case nodeQueue <- masterNode:
			case <-ctx.Done():
				d.mu.Unlock()
				return ctx.Err()
			default:
				// 队列满了，跳过
			}
		}
		d.mu.Unlock()
	}

	return nil
}

// ValidateNode 验证节点状态
func (d *DefaultTopologyDiscoverer) ValidateNode(ctx context.Context, node *NodeConfig) (*NodeStatus, error) {
	conn, err := d.connectionFactory.CreateConnection(node)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", node.NodeKey(), err)
	}
	defer conn.Close()

	status := &NodeStatus{
		LastCheck: time.Now(),
	}

	// 检查read_only状态
	var variableName, readOnlyValue string
	err = conn.QueryRow("SHOW VARIABLES LIKE 'read_only'").Scan(&variableName, &readOnlyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to check read_only for %s: %w", node.NodeKey(), err)
	}
	status.IsReadOnly = strings.ToUpper(readOnlyValue) == "ON"

	// 获取server_id
	var serverID uint32
	err = conn.QueryRow("SHOW VARIABLES LIKE 'server_id'").Scan(&variableName, &serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server_id for %s: %w", node.NodeKey(), err)
	}
	status.ServerID = serverID

	// 获取从库状态
	slaveStatus, err := d.GetSlaveStatus(ctx, node)
	if err == nil && slaveStatus != nil {
		status.SlaveStatus = slaveStatus
		status.IsMaster = false
		status.Role = "从"
	} else {
		// 没有从库状态，可能是主库
		status.IsMaster = !status.IsReadOnly
		if status.IsMaster {
			status.Role = "主"
		} else {
			status.Role = "未知"
		}
	}

	// 获取从库主机列表
	slaveHosts, err := d.GetSlaveHosts(ctx, node)
	if err == nil {
		for _, slave := range slaveHosts {
			status.SlaveHosts = append(status.SlaveHosts, fmt.Sprintf("%s:%d", slave.Host, slave.Port))
		}
	}

	// 使用外部API验证（如果可用）
	if d.externalAPI != nil && d.options.EnableExternalAPI {
		if externalInfo, err := d.externalAPI.GetNodeInfo(ctx, node.Host); err == nil {
			// 外部API验证结果优先级更高
			if externalInfo.InstanceRole == "主" {
				status.IsMaster = true
				status.Role = "主"
			} else {
				status.IsMaster = false
				status.Role = externalInfo.InstanceRole
			}
			
			if externalInfo.InstanceReadOnly == "OFF" {
				status.IsReadOnly = false
			} else {
				status.IsReadOnly = true
			}
		}
	}

	return status, nil
}

// GetSlaveHosts 获取从库主机列表
func (d *DefaultTopologyDiscoverer) GetSlaveHosts(ctx context.Context, node *NodeConfig) ([]*SlaveHost, error) {
	conn, err := d.connectionFactory.CreateConnection(node)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", node.NodeKey(), err)
	}
	defer conn.Close()

	rows, err := conn.Query("SHOW SLAVE HOSTS")
	if err != nil {
		return nil, fmt.Errorf("failed to execute SHOW SLAVE HOSTS on %s: %w", node.NodeKey(), err)
	}
	defer rows.Close()

	var slaves []*SlaveHost
	for rows.Next() {
		var serverID uint32
		var host string
		var port int
		var user sql.NullString

		if err := rows.Scan(&serverID, &host, &port, &user); err != nil {
			continue // 跳过错误的行
		}

		slave := &SlaveHost{
			ServerID: serverID,
			Host:     host,
			Port:     port,
		}
		if user.Valid {
			slave.User = user.String
		}

		slaves = append(slaves, slave)
	}

	return slaves, nil
}

// GetSlaveStatus 获取从库状态
func (d *DefaultTopologyDiscoverer) GetSlaveStatus(ctx context.Context, node *NodeConfig) (*SlaveStatus, error) {
	conn, err := d.connectionFactory.CreateConnection(node)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", node.NodeKey(), err)
	}
	defer conn.Close()

	rows, err := conn.Query("SHOW SLAVE STATUS")
	if err != nil {
		return nil, fmt.Errorf("failed to execute SHOW SLAVE STATUS on %s: %w", node.NodeKey(), err)
	}
	defer rows.Close()

	if !rows.Next() {
		// 没有从库状态，不是从库
		return nil, nil
	}

	// SHOW SLAVE STATUS 返回的列很多，我们只取需要的
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// 创建扫描目标
	values := make([]interface{}, len(columns))
	scanTargets := make([]interface{}, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	if err := rows.Scan(scanTargets...); err != nil {
		return nil, fmt.Errorf("failed to scan slave status: %w", err)
	}

	// 解析结果
	status := &SlaveStatus{}
	columnMap := make(map[string]interface{})
	for i, col := range columns {
		columnMap[col] = values[i]
	}

	// 提取关键字段
	if val, ok := columnMap["Master_Host"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			status.MasterHost = string(v)
		default:
			status.MasterHost = fmt.Sprintf("%v", v)
		}
	}
	if val, ok := columnMap["Master_Port"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			if port, err := strconv.Atoi(string(v)); err == nil {
				status.MasterPort = port
			}
		case int64:
			status.MasterPort = int(v)
		default:
			// 尝试将值转换为字符串再解析
			if port, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
				status.MasterPort = port
			}
		}
	}
	if val, ok := columnMap["Master_User"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			status.MasterUser = string(v)
		default:
			status.MasterUser = fmt.Sprintf("%v", v)
		}
	}
	if val, ok := columnMap["Slave_IO_Running"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			status.SlaveIORunning = string(v)
		default:
			status.SlaveIORunning = fmt.Sprintf("%v", v)
		}
	}
	if val, ok := columnMap["Slave_SQL_Running"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			status.SlaveSQLRunning = string(v)
		default:
			status.SlaveSQLRunning = fmt.Sprintf("%v", v)
		}
	}
	if val, ok := columnMap["Seconds_Behind_Master"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			if lag, err := strconv.Atoi(string(v)); err == nil {
				status.SecondsBehindMaster = &lag
			}
		case int64:
			lag := int(v)
			status.SecondsBehindMaster = &lag
		default:
			if lag, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
				status.SecondsBehindMaster = &lag
			}
		}
	}
	if val, ok := columnMap["Master_Log_File"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			status.MasterLogFile = string(v)
		default:
			status.MasterLogFile = fmt.Sprintf("%v", v)
		}
	}
	if val, ok := columnMap["Slave_IO_State"]; ok && val != nil {
		switch v := val.(type) {
		case []byte:
			status.SlaveIOState = string(v)
		default:
			status.SlaveIOState = fmt.Sprintf("%v", v)
		}
	}

	return status, nil
}

// analyzeMasterSlave 分析主从关系
func (d *DefaultTopologyDiscoverer) analyzeMasterSlave(topology *ClusterTopology) error {
	var masterCandidates []*NodeConfig
	var slaves []*NodeConfig

	// 分析所有节点
	for nodeKey, status := range topology.NodeStatuses {
		// 根据节点key找到对应的NodeConfig
		var node *NodeConfig
		if topology.MasterNode != nil && topology.MasterNode.NodeKey() == nodeKey {
			node = topology.MasterNode
		} else {
			for _, slaveNode := range topology.SlaveNodes {
				if slaveNode.NodeKey() == nodeKey {
					node = slaveNode
					break
				}
			}
		}

		if node == nil {
			continue
		}

		if status.IsMasterRole() {
			masterCandidates = append(masterCandidates, node)
		} else {
			slaves = append(slaves, node)
		}
	}

	// 确定主节点
	if len(masterCandidates) == 1 {
		topology.MasterNode = masterCandidates[0]
	} else if len(masterCandidates) > 1 {
		// 多个主节点候选，选择没有上游复制关系的
		for _, candidate := range masterCandidates {
			status := topology.NodeStatuses[candidate.NodeKey()]
			if status.SlaveStatus == nil {
				topology.MasterNode = candidate
				break
			}
		}
		if topology.MasterNode == nil {
			topology.MasterNode = masterCandidates[0] // 降级选择第一个
		}
	}

	// 设置从节点列表
	topology.SlaveNodes = slaves
	topology.TotalNodes = len(topology.NodeStatuses)

	return nil
}

// ExternalAPIClient 外部API客户端接口
type ExternalAPIClient interface {
	GetNodeInfo(ctx context.Context, host string) (*ExternalNodeInfo, error)
}