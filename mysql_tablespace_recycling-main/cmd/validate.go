package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"mysql_tablespace_recycling/cluster"

	"github.com/spf13/cobra"
)

// 验证命令选项
var (
	validateOutput     string
	validateStrictMode bool
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证节点角色和操作安全性",
	Long: `验证MySQL节点的角色（主/从）和执行表空间回收操作的安全性。

该命令使用多种方式验证节点身份，包括：
- 检查read_only和super_read_only状态
- 分析SLAVE STATUS和SLAVE HOSTS信息
- 调用外部集群管理API（如果配置）
- 评估操作风险等级

主要用于在执行表空间回收前确认目标节点的安全性。`,
	Example: `  # 验证本地MySQL节点
  mysql-recycler validate

  # 验证指定节点，输出详细信息
  mysql-recycler validate --host=192.168.1.101 --verbose

  # 严格模式验证（要求高信心度）
  mysql-recycler validate --host=192.168.1.101 --strict

  # 输出JSON格式的验证结果
  mysql-recycler validate --host=192.168.1.101 --output=json`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVarP(&validateOutput, "output", "o", "table", "输出格式 (table, json)")
	validateCmd.Flags().BoolVar(&validateStrictMode, "strict", false, "严格模式验证")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// 创建节点配置
	nodeConfig, err := createNodeConfig()
	if err != nil {
		return fmt.Errorf("failed to create node config: %w", err)
	}

	// 创建组件
	factory := &cluster.DefaultConnectionFactory{}
	options := &cluster.DiscoveryOptions{
		MaxConcurrency: 3,
		Timeout:        30 * time.Second,
		MaxDepth:       1, // 只需要验证当前节点
	}
	discoverer := cluster.NewTopologyDiscoverer(factory, options)
	validator := cluster.NewMasterValidator(discoverer, nil, factory, validateStrictMode)

	// 执行验证
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	if verbose {
		fmt.Printf("正在验证 %s 的节点角色和安全性...\n", nodeConfig.NodeKey())
	}

	clusterNodeConfig := nodeConfig.toClusterNodeConfig()
	result, err := validator.GetMasterValidationResult(ctx, clusterNodeConfig)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// 输出结果
	switch validateOutput {
	case "json":
		return printJSON(result)
	case "table":
		return printValidationReport(nodeConfig, result)
	default:
		return fmt.Errorf("unsupported output format: %s", validateOutput)
	}
}

// printValidationReport 打印验证报告
func printValidationReport(nodeConfig *NodeConfig, result *cluster.MasterValidationResult) error {
	fmt.Printf("MySQL节点角色验证报告\n")
	fmt.Printf("====================\n")
	fmt.Printf("节点: %s\n", nodeConfig.NodeKey())
	fmt.Printf("验证时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	// 主要结论
	fmt.Printf("验证结果:\n")
	fmt.Printf("--------\n")
	
	nodeType := "从节点"
	if result.IsMaster {
		nodeType = "主节点"
	}
	fmt.Printf("节点类型: %s\n", nodeType)
	fmt.Printf("验证信心度: %s\n", getConfidenceDisplay(result.Confidence))
	
	safetyStatus := "✓ 安全"
	if result.IsMaster {
		safetyStatus = "⚠ 危险 - 主节点操作需谨慎"
	}
	fmt.Printf("操作安全性: %s\n", safetyStatus)
	fmt.Println()

	// 验证方法
	fmt.Printf("验证方法:\n")
	fmt.Printf("--------\n")
	for i, method := range result.ValidationMethods {
		fmt.Printf("%d. %s\n", i+1, getMethodDisplay(method))
	}
	fmt.Println()

	// 数据库状态
	if result.DatabaseStatus != nil {
		fmt.Printf("数据库状态:\n")
		fmt.Printf("----------\n")
		
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "项目\t状态\t说明")
		fmt.Fprintln(w, "----\t----\t----")
		
		readOnlyStatus := "ON (只读)"
		if !result.DatabaseStatus.ReadOnly {
			readOnlyStatus = "OFF (可写)"
		}
		fmt.Fprintf(w, "read_only\t%s\t%s\n", readOnlyStatus, getReadOnlyDescription(result.DatabaseStatus.ReadOnly))
		
		if result.DatabaseStatus.SuperReadOnly {
			fmt.Fprintf(w, "super_read_only\tON\t超级只读模式\n")
		}
		
		fmt.Fprintf(w, "server_id\t%d\t服务器标识\n", result.DatabaseStatus.ServerID)
		
		binlogStatus := "OFF"
		if result.DatabaseStatus.LogBin {
			binlogStatus = "ON"
		}
		fmt.Fprintf(w, "log_bin\t%s\t二进制日志\n", binlogStatus)
		
		if result.DatabaseStatus.BinlogFormat != "" {
			fmt.Fprintf(w, "binlog_format\t%s\t二进制日志格式\n", result.DatabaseStatus.BinlogFormat)
		}
		
		if result.DatabaseStatus.HasSlaveStatus {
			fmt.Fprintf(w, "复制状态\t有\t作为从库运行\n")
		} else {
			fmt.Fprintf(w, "复制状态\t无\t不是从库\n")
		}
		
		if result.DatabaseStatus.SlaveCount > 0 {
			fmt.Fprintf(w, "从库连接\t%d个\t作为主库为从库提供服务\n", result.DatabaseStatus.SlaveCount)
		}
		
		w.Flush()
		fmt.Println()
	}

	// 验证理由
	if len(result.Reasons) > 0 {
		fmt.Printf("验证依据:\n")
		fmt.Printf("--------\n")
		for i, reason := range result.Reasons {
			fmt.Printf("%d. %s\n", i+1, reason)
		}
		fmt.Println()
	}

	// 外部API结果
	if result.ExternalAPIResult != nil {
		fmt.Printf("外部API验证:\n")
		fmt.Printf("-----------\n")
		fmt.Printf("集群名称: %s\n", result.ExternalAPIResult.ClusterName)
		fmt.Printf("集群组: %s\n", result.ExternalAPIResult.ClusterGroupName)
		fmt.Printf("实例角色: %s\n", result.ExternalAPIResult.InstanceRole)
		fmt.Printf("只读状态: %s\n", result.ExternalAPIResult.InstanceReadOnly)
		fmt.Println()
	}

	// 警告信息
	if len(result.Warnings) > 0 {
		fmt.Printf("警告信息:\n")
		fmt.Printf("--------\n")
		for i, warning := range result.Warnings {
			fmt.Printf("⚠ %d. %s\n", i+1, warning)
		}
		fmt.Println()
	}

	// 操作建议
	fmt.Printf("操作建议:\n")
	fmt.Printf("--------\n")
	if result.IsMaster {
		fmt.Printf("❌ 不建议在主节点上执行表空间回收操作\n")
		fmt.Printf("   - 表空间回收会锁定表，影响业务读写\n")
		fmt.Printf("   - 建议在从节点上执行，或在维护窗口期间执行\n")
		fmt.Printf("   - 如确需在主节点执行，请使用 --allow-master 参数\n")
	} else {
		fmt.Printf("✓ 可以安全地在此从节点上执行表空间回收操作\n")
		fmt.Printf("   - 建议在业务低峰期执行以减少影响\n")
		fmt.Printf("   - 执行前请确保有完整的数据备份\n")
		
		if result.Confidence == cluster.ConfidenceLow || result.Confidence == cluster.ConfidenceNone {
			fmt.Printf("   - 验证信心度较低，建议进一步确认节点状态\n")
		}
	}

	return nil
}

// getConfidenceDisplay 获取信心度显示文本
func getConfidenceDisplay(confidence cluster.ValidationConfidence) string {
	switch confidence {
	case cluster.ConfidenceHigh:
		return "高 (可信)"
	case cluster.ConfidenceMedium:
		return "中 (较可信)"
	case cluster.ConfidenceLow:
		return "低 (需谨慎)"
	case cluster.ConfidenceNone:
		return "无 (无法确定)"
	default:
		return string(confidence)
	}
}

// getMethodDisplay 获取验证方法显示文本
func getMethodDisplay(method cluster.ValidationMethod) string {
	switch method {
	case cluster.ValidationReadOnly:
		return "read_only状态检查"
	case cluster.ValidationSlaveStatus:
		return "SLAVE STATUS分析"
	case cluster.ValidationSlaveHosts:
		return "SLAVE HOSTS检查"
	case cluster.ValidationExternalAPI:
		return "外部API验证"
	case cluster.ValidationProcessList:
		return "进程列表检查"
	default:
		return string(method)
	}
}

// getReadOnlyDescription 获取只读状态描述
func getReadOnlyDescription(readOnly bool) string {
	if readOnly {
		return "通常表示从节点"
	} else {
		return "通常表示主节点"
	}
}