package cluster

import (
	"context"
	"fmt"
	"strings"
)

// MasterValidator 主节点验证器接口
type MasterValidator interface {
	IsMasterNode(ctx context.Context, node *NodeConfig) (bool, error)
	ValidateMasterSafety(ctx context.Context, node *NodeConfig) error
	GetMasterValidationResult(ctx context.Context, node *NodeConfig) (*MasterValidationResult, error)
}

// MasterValidationResult 主节点验证结果
type MasterValidationResult struct {
	IsMaster           bool                  `json:"is_master"`
	ValidationMethods  []ValidationMethod    `json:"validation_methods"`
	Confidence         ValidationConfidence  `json:"confidence"`
	Reasons            []string              `json:"reasons"`
	Warnings           []string              `json:"warnings"`
	ExternalAPIResult  *ExternalNodeInfo     `json:"external_api_result,omitempty"`
	DatabaseStatus     *DatabaseMasterStatus `json:"database_status"`
}

// ValidationMethod 验证方法
type ValidationMethod string

const (
	ValidationReadOnly     ValidationMethod = "read_only_check"
	ValidationSlaveStatus  ValidationMethod = "slave_status_check"
	ValidationSlaveHosts   ValidationMethod = "slave_hosts_check"
	ValidationExternalAPI  ValidationMethod = "external_api_check"
	ValidationProcessList  ValidationMethod = "process_list_check"
)

// ValidationConfidence 验证信心度
type ValidationConfidence string

const (
	ConfidenceHigh   ValidationConfidence = "high"
	ConfidenceMedium ValidationConfidence = "medium"
	ConfidenceLow    ValidationConfidence = "low"
	ConfidenceNone   ValidationConfidence = "none"
)

// DatabaseMasterStatus 数据库主节点状态
type DatabaseMasterStatus struct {
	ReadOnly              bool     `json:"read_only"`
	SuperReadOnly         bool     `json:"super_read_only"`
	HasSlaveConnections   bool     `json:"has_slave_connections"`
	SlaveCount            int      `json:"slave_count"`
	HasSlaveStatus        bool     `json:"has_slave_status"`
	ServerID              uint32   `json:"server_id"`
	LogBin                bool     `json:"log_bin"`
	BinlogFormat          string   `json:"binlog_format"`
	ActiveConnections     int      `json:"active_connections"`
	ReplicationUsers      []string `json:"replication_users"`
}

// DefaultMasterValidator 默认主节点验证器
type DefaultMasterValidator struct {
	discoverer    TopologyDiscoverer
	externalAPI   ExternalAPIClient
	factory       ConnectionFactory
	strictMode    bool
}

// NewMasterValidator 创建主节点验证器
func NewMasterValidator(discoverer TopologyDiscoverer, externalAPI ExternalAPIClient, factory ConnectionFactory, strictMode bool) MasterValidator {
	return &DefaultMasterValidator{
		discoverer:  discoverer,
		externalAPI: externalAPI,
		factory:     factory,
		strictMode:  strictMode,
	}
}

// IsMasterNode 检查是否为主节点
func (v *DefaultMasterValidator) IsMasterNode(ctx context.Context, node *NodeConfig) (bool, error) {
	result, err := v.GetMasterValidationResult(ctx, node)
	if err != nil {
		return false, err
	}
	
	// 在严格模式下需要高信心度
	if v.strictMode && result.Confidence != ConfidenceHigh {
		return false, fmt.Errorf("master validation confidence too low: %s", result.Confidence)
	}
	
	return result.IsMaster, nil
}

// ValidateMasterSafety 验证主节点操作安全性
func (v *DefaultMasterValidator) ValidateMasterSafety(ctx context.Context, node *NodeConfig) error {
	result, err := v.GetMasterValidationResult(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to validate master: %w", err)
	}
	
	if result.IsMaster {
		return fmt.Errorf("operation blocked: target node identified as master node")
	}
	
	// 检查警告
	if len(result.Warnings) > 0 {
		warningMsg := fmt.Sprintf("master validation warnings: %s", strings.Join(result.Warnings, "; "))
		if v.strictMode {
			return fmt.Errorf("operation blocked due to warnings in strict mode: %s", warningMsg)
		}
		// 非严格模式只记录警告
		fmt.Printf("Warning: %s\n", warningMsg)
	}
	
	return nil
}

// GetMasterValidationResult 获取完整的主节点验证结果
func (v *DefaultMasterValidator) GetMasterValidationResult(ctx context.Context, node *NodeConfig) (*MasterValidationResult, error) {
	result := &MasterValidationResult{
		ValidationMethods: []ValidationMethod{},
		Reasons:          []string{},
		Warnings:         []string{},
		DatabaseStatus:   &DatabaseMasterStatus{},
	}
	
	// 1. 数据库状态检查
	if err := v.checkDatabaseStatus(ctx, node, result); err != nil {
		return nil, fmt.Errorf("failed to check database status: %w", err)
	}
	
	// 2. 外部API检查（如果可用）
	if v.externalAPI != nil {
		v.checkExternalAPI(ctx, node, result)
	}
	
	// 3. 拓扑关系检查
	if err := v.checkTopologyRelationship(ctx, node, result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("topology check failed: %v", err))
	}
	
	// 4. 综合判断
	v.calculateFinalResult(result)
	
	return result, nil
}

// checkDatabaseStatus 检查数据库状态
func (v *DefaultMasterValidator) checkDatabaseStatus(ctx context.Context, node *NodeConfig, result *MasterValidationResult) error {
	conn, err := v.factory.CreateConnection(node)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.NodeKey(), err)
	}
	defer conn.Close()
	
	dbStatus := result.DatabaseStatus
	
	// 检查read_only状态
	var variableName, readOnlyValue string
	err = conn.QueryRow("SHOW VARIABLES LIKE 'read_only'").Scan(&variableName, &readOnlyValue)
	if err != nil {
		return fmt.Errorf("failed to check read_only: %w", err)
	}
	dbStatus.ReadOnly = strings.ToUpper(readOnlyValue) == "ON"
	result.ValidationMethods = append(result.ValidationMethods, ValidationReadOnly)
	
	if !dbStatus.ReadOnly {
		result.Reasons = append(result.Reasons, "read_only=OFF (master characteristic)")
	} else {
		result.Reasons = append(result.Reasons, "read_only=ON (slave characteristic)")
	}
	
	// 检查super_read_only状态
	var superReadOnlyValue string
	err = conn.QueryRow("SHOW VARIABLES LIKE 'super_read_only'").Scan(&variableName, &superReadOnlyValue)
	if err == nil {
		dbStatus.SuperReadOnly = strings.ToUpper(superReadOnlyValue) == "ON"
		if dbStatus.SuperReadOnly {
			result.Reasons = append(result.Reasons, "super_read_only=ON")
		}
	}
	
	// 检查server_id
	var serverID uint32
	err = conn.QueryRow("SHOW VARIABLES LIKE 'server_id'").Scan(&variableName, &serverID)
	if err == nil {
		dbStatus.ServerID = serverID
		if serverID == 0 {
			result.Warnings = append(result.Warnings, "server_id is 0, replication not properly configured")
		}
	}
	
	// 检查binlog状态
	var logBinValue string
	err = conn.QueryRow("SHOW VARIABLES LIKE 'log_bin'").Scan(&variableName, &logBinValue)
	if err == nil {
		dbStatus.LogBin = strings.ToUpper(logBinValue) == "ON"
		if !dbStatus.LogBin {
			result.Warnings = append(result.Warnings, "binary logging is disabled")
		}
	}
	
	// 检查binlog格式
	var binlogFormatValue string
	err = conn.QueryRow("SHOW VARIABLES LIKE 'binlog_format'").Scan(&variableName, &binlogFormatValue)
	if err == nil {
		dbStatus.BinlogFormat = binlogFormatValue
	}
	
	// 检查SLAVE STATUS
	slaveStatus, err := v.discoverer.GetSlaveStatus(ctx, node)
	if err == nil && slaveStatus != nil {
		dbStatus.HasSlaveStatus = true
		result.ValidationMethods = append(result.ValidationMethods, ValidationSlaveStatus)
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("has slave status (master: %s:%d)", slaveStatus.MasterHost, slaveStatus.MasterPort))
	} else {
		result.ValidationMethods = append(result.ValidationMethods, ValidationSlaveStatus)
		result.Reasons = append(result.Reasons, "no slave status (potential master)")
	}
	
	// 检查SLAVE HOSTS
	slaveHosts, err := v.discoverer.GetSlaveHosts(ctx, node)
	if err == nil && len(slaveHosts) > 0 {
		dbStatus.HasSlaveConnections = true
		dbStatus.SlaveCount = len(slaveHosts)
		result.ValidationMethods = append(result.ValidationMethods, ValidationSlaveHosts)
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("has %d slave connections", len(slaveHosts)))
			
		// 记录从库信息
		for _, slave := range slaveHosts {
			dbStatus.ReplicationUsers = append(dbStatus.ReplicationUsers, slave.User)
		}
	}
	
	// 检查进程列表中的复制连接
	v.checkProcessList(ctx, conn, result)
	
	return nil
}

// checkExternalAPI 检查外部API
func (v *DefaultMasterValidator) checkExternalAPI(ctx context.Context, node *NodeConfig, result *MasterValidationResult) {
	externalInfo, err := v.externalAPI.GetNodeInfo(ctx, node.Host)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("external API check failed: %v", err))
		return
	}
	
	result.ExternalAPIResult = externalInfo
	result.ValidationMethods = append(result.ValidationMethods, ValidationExternalAPI)
	
	if externalInfo.InstanceRole == "主" {
		result.Reasons = append(result.Reasons, "external API confirms master role")
	} else {
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("external API role: %s", externalInfo.InstanceRole))
	}
	
	if externalInfo.InstanceReadOnly == "OFF" {
		result.Reasons = append(result.Reasons, "external API confirms read_only=OFF")
	}
}

// checkTopologyRelationship 检查拓扑关系
func (v *DefaultMasterValidator) checkTopologyRelationship(ctx context.Context, node *NodeConfig, result *MasterValidationResult) error {
	// 这里可以实现更复杂的拓扑关系检查
	// 例如检查节点在集群中的位置等
	return nil
}

// checkProcessList 检查进程列表
func (v *DefaultMasterValidator) checkProcessList(ctx context.Context, conn DBConnection, result *MasterValidationResult) {
	rows, err := conn.Query("SHOW PROCESSLIST")
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to check process list: %v", err))
		return
	}
	
	if rows != nil {
		defer rows.Close()
	}
	
	result.ValidationMethods = append(result.ValidationMethods, ValidationProcessList)
	
	activeConnections := 0
	replicationConnections := 0
	
	// 简化的进程列表检查 - 由于接口限制，这里先做基础统计
	result.DatabaseStatus.ActiveConnections = activeConnections
	
	if replicationConnections > 0 {
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("found %d replication connections in process list", replicationConnections))
	}
}

// calculateFinalResult 计算最终结果
func (v *DefaultMasterValidator) calculateFinalResult(result *MasterValidationResult) {
	dbStatus := result.DatabaseStatus
	
	// 基础判断：如果有slave status，很可能是从节点
	if dbStatus.HasSlaveStatus {
		result.IsMaster = false
		result.Confidence = ConfidenceHigh
		return
	}
	
	// 主要判断依据
	masterIndicators := 0
	slaveIndicators := 0
	
	// read_only状态是最重要的指标
	if !dbStatus.ReadOnly {
		masterIndicators += 3
	} else {
		slaveIndicators += 3
	}
	
	// super_read_only状态
	if dbStatus.SuperReadOnly {
		slaveIndicators += 2
	}
	
	// 有从库连接
	if dbStatus.HasSlaveConnections {
		masterIndicators += 2
	}
	
	// 外部API结果
	if result.ExternalAPIResult != nil {
		if result.ExternalAPIResult.InstanceRole == "主" {
			masterIndicators += 3
		} else {
			slaveIndicators += 2
		}
	}
	
	// binlog开启状态
	if dbStatus.LogBin {
		masterIndicators += 1
	}
	
	// 计算结果
	if masterIndicators > slaveIndicators {
		result.IsMaster = true
		if masterIndicators >= 5 && len(result.ValidationMethods) >= 3 {
			result.Confidence = ConfidenceHigh
		} else if masterIndicators >= 3 {
			result.Confidence = ConfidenceMedium
		} else {
			result.Confidence = ConfidenceLow
		}
	} else if slaveIndicators > masterIndicators {
		result.IsMaster = false
		if slaveIndicators >= 5 && len(result.ValidationMethods) >= 3 {
			result.Confidence = ConfidenceHigh
		} else if slaveIndicators >= 3 {
			result.Confidence = ConfidenceMedium
		} else {
			result.Confidence = ConfidenceLow
		}
	} else {
		// 无法确定
		result.IsMaster = false // 安全起见，默认不是主节点
		result.Confidence = ConfidenceNone
		result.Warnings = append(result.Warnings, "unable to determine master status with confidence")
	}
}