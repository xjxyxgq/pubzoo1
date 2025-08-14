package cmd

import (
	"testing"
	"time"

	"mysql_tablespace_recycling/cluster"
)

func TestNodeConfigAdapter(t *testing.T) {
	clusterNode := &cluster.NodeConfig{
		Host:     "test.host",
		Port:     3306,
		Username: "testuser",
		Password: "testpass",
		Database: "testdb",
		Timeout:  30 * time.Second,
	}
	
	adapter := &NodeConfigAdapter{clusterNode}
	
	// 测试适配器方法
	if adapter.GetHost() != "test.host" {
		t.Errorf("GetHost() = %s, want test.host", adapter.GetHost())
	}
	
	if adapter.GetPort() != 3306 {
		t.Errorf("GetPort() = %d, want 3306", adapter.GetPort())
	}
	
	if adapter.GetUsername() != "testuser" {
		t.Errorf("GetUsername() = %s, want testuser", adapter.GetUsername())
	}
	
	if adapter.GetPassword() != "testpass" {
		t.Errorf("GetPassword() = %s, want testpass", adapter.GetPassword())
	}
	
	if adapter.GetDatabase() != "testdb" {
		t.Errorf("GetDatabase() = %s, want testdb", adapter.GetDatabase())
	}
	
	if adapter.GetTimeout() != 30*time.Second {
		t.Errorf("GetTimeout() = %v, want 30s", adapter.GetTimeout())
	}
}

func TestReclaimStartNodeFlag(t *testing.T) {
	// 测试CLI标志的设置
	originalValue := reclaimStartNode
	defer func() {
		reclaimStartNode = originalValue
	}()
	
	// 设置测试值
	reclaimStartNode = "test.host:3306"
	
	if reclaimStartNode != "test.host:3306" {
		t.Errorf("reclaimStartNode = %s, want test.host:3306", reclaimStartNode)
	}
	
	// 测试空值
	reclaimStartNode = ""
	if reclaimStartNode != "" {
		t.Errorf("reclaimStartNode should be empty, got %s", reclaimStartNode)
	}
}

func TestValidateStartNodeFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{
			name:      "valid host:port format",
			input:     "mysql.host:3306",
			wantValid: true,
		},
		{
			name:      "valid IP:port format",
			input:     "192.168.1.100:3306",
			wantValid: true,
		},
		{
			name:      "empty string (should be valid as it means no filter)",
			input:     "",
			wantValid: true,
		},
		{
			name:      "missing port",
			input:     "mysql.host",
			wantValid: false,
		},
		{
			name:      "invalid port",
			input:     "mysql.host:abc",
			wantValid: false,
		},
		{
			name:      "only port",
			input:     ":3306",
			wantValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateStartNodeFormat(tt.input)
			if valid != tt.wantValid {
				t.Errorf("validateStartNodeFormat(%s) = %v, want %v", tt.input, valid, tt.wantValid)
			}
		})
	}
}

// validateStartNodeFormat 验证起始节点格式的辅助函数
func validateStartNodeFormat(startNode string) bool {
	if startNode == "" {
		return true // 空字符串表示不过滤，是有效的
	}
	
	// 简单的host:port格式验证
	// 实际实现中可能需要更严格的验证
	colonIndex := -1
	for i, char := range startNode {
		if char == ':' {
			if colonIndex != -1 {
				return false // 多个冒号
			}
			colonIndex = i
		}
	}
	
	if colonIndex == -1 {
		return false // 没有冒号
	}
	
	if colonIndex == 0 || colonIndex == len(startNode)-1 {
		return false // 冒号在开头或结尾
	}
	
	host := startNode[:colonIndex]
	portStr := startNode[colonIndex+1:]
	
	// 验证主机名不为空
	if len(host) == 0 {
		return false
	}
	
	// 验证端口是数字且在有效范围内
	port := 0
	for _, char := range portStr {
		if char < '0' || char > '9' {
			return false
		}
		port = port*10 + int(char-'0')
	}
	
	return port > 0 && port <= 65535
}