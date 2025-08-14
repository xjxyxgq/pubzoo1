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

// 拓扑命令选项
var (
	topologyOutput    string
	topologyMaxDepth  int
	topologyConcurrency int
)

var topologyCmd = &cobra.Command{
	Use:   "topology",
	Short: "发现MySQL集群拓扑结构",
	Long: `自动发现MySQL集群的主从复制拓扑结构。

该命令从指定的MySQL节点开始，通过SHOW SLAVE HOSTS和SHOW SLAVE STATUS
自动探测整个集群的复制关系，构建完整的拓扑图。`,
	Example: `  # 发现本地MySQL的集群拓扑
  mysql-recycler topology

  # 发现指定主机的集群拓扑，输出JSON格式
  mysql-recycler topology --host=192.168.1.100 --output=json

  # 限制探测深度和并发数
  mysql-recycler topology --max-depth=5 --concurrency=3`,
	RunE: runTopology,
}

func init() {
	topologyCmd.Flags().StringVarP(&topologyOutput, "output", "o", "table", "输出格式 (table, json)")
	topologyCmd.Flags().IntVar(&topologyMaxDepth, "max-depth", 5, "最大探测深度")
	topologyCmd.Flags().IntVar(&topologyConcurrency, "concurrency", 3, "并发数")
}

func runTopology(cmd *cobra.Command, args []string) error {
	// 创建节点配置
	nodeConfig, err := createNodeConfig()
	if err != nil {
		return fmt.Errorf("failed to create node config: %w", err)
	}

	// 创建拓扑发现器
	factory := &cluster.DefaultConnectionFactory{}
	options := &cluster.DiscoveryOptions{
		MaxConcurrency: topologyConcurrency,
		Timeout:        2 * time.Minute,
		MaxDepth:       topologyMaxDepth,
	}
	discoverer := cluster.NewTopologyDiscoverer(factory, options)

	// 执行拓扑发现
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if verbose {
		fmt.Printf("正在发现 %s 的集群拓扑...\n", nodeConfig.NodeKey())
	}

	topology, err := discoverer.DiscoverTopology(ctx, nodeConfig.toClusterNodeConfig())
	if err != nil {
		return fmt.Errorf("topology discovery failed: %w", err)
	}

	// 输出结果
	switch topologyOutput {
	case "json":
		return printJSON(topology)
	case "table":
		return printTopologyReport(topology)
	default:
		return fmt.Errorf("unsupported output format: %s", topologyOutput)
	}
}

// printTopologyReport 打印拓扑报告
func printTopologyReport(topology *cluster.ClusterTopology) error {
	fmt.Printf("MySQL集群拓扑结构\n")
	fmt.Printf("==================\n")
	fmt.Printf("发现时间: %s\n", topology.DiscoveredAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("总节点数: %d\n", topology.TotalNodes)
	fmt.Println()

	// 显示主节点
	if topology.MasterNode != nil {
		fmt.Printf("主节点:\n")
		fmt.Printf("------\n")
		status := topology.GetNodeStatus(topology.MasterNode)
		printNodeInfo(topology.MasterNode, status, "主")
		fmt.Println()
	}

	// 显示从节点
	if len(topology.SlaveNodes) > 0 {
		fmt.Printf("从节点:\n")
		fmt.Printf("------\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "节点\t角色\t只读状态\t服务器ID\t上游主库\t下游从库数\t复制状态")
		fmt.Fprintln(w, "----\t----\t--------\t--------\t--------\t----------\t--------")

		for _, slave := range topology.SlaveNodes {
			status := topology.GetNodeStatus(slave)
			if status == nil {
				continue
			}

			readOnlyStatus := "否"
			if status.IsReadOnly {
				readOnlyStatus = "是"
			}

			masterInfo := "无"
			replicationStatus := "未知"
			if status.SlaveStatus != nil {
				masterInfo = fmt.Sprintf("%s:%d", status.SlaveStatus.MasterHost, status.SlaveStatus.MasterPort)
				if status.SlaveStatus.IsHealthy() {
					replicationStatus = "正常"
				} else {
					replicationStatus = "异常"
				}
				if status.SlaveStatus.GetReplicationDelay() >= 0 {
					replicationStatus += fmt.Sprintf(" (延迟%ds)", status.SlaveStatus.GetReplicationDelay())
				}
			}

			downstreamNodes := topology.GetDownstreamNodes(slave)
			downstreamCount := len(downstreamNodes)

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%d\t%s\n",
				slave.NodeKey(),
				status.Role,
				readOnlyStatus,
				status.ServerID,
				masterInfo,
				downstreamCount,
				replicationStatus,
			)
		}
		w.Flush()
		fmt.Println()
	}

	// 显示复制关系
	if len(topology.Relationships) > 0 {
		fmt.Printf("复制关系:\n")
		fmt.Printf("--------\n")
		printReplicationTree(topology, topology.MasterNode, 0)
	}

	return nil
}

// printNodeInfo 打印节点信息
func printNodeInfo(node *cluster.NodeConfig, status *cluster.NodeStatus, role string) {
	fmt.Printf("  地址: %s\n", node.NodeKey())
	if status != nil {
		fmt.Printf("  角色: %s\n", role)
		fmt.Printf("  只读: %t\n", status.IsReadOnly)
		fmt.Printf("  服务器ID: %d\n", status.ServerID)
		fmt.Printf("  下游从库: %d个\n", len(status.SlaveHosts))
		fmt.Printf("  最后检查: %s\n", status.LastCheck.Format("2006-01-02 15:04:05"))
	}
}

// printReplicationTree 打印复制树
func printReplicationTree(topology *cluster.ClusterTopology, node *cluster.NodeConfig, depth int) {
	if node == nil {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		if i == depth-1 {
			indent += "├── "
		} else {
			indent += "│   "
		}
	}

	if depth == 0 {
		fmt.Printf("%s (主)\n", node.NodeKey())
	} else {
		status := topology.GetNodeStatus(node)
		statusInfo := ""
		if status != nil && status.SlaveStatus != nil {
			if status.SlaveStatus.IsHealthy() {
				statusInfo = " [正常]"
			} else {
				statusInfo = " [异常]"
			}
			if lag := status.SlaveStatus.GetReplicationDelay(); lag >= 0 {
				statusInfo += fmt.Sprintf(" (延迟%ds)", lag)
			}
		}
		fmt.Printf("%s%s%s\n", indent, node.NodeKey(), statusInfo)
	}

	// 递归打印下游节点
	downstreamNodes := topology.GetDownstreamNodes(node)
	for _, downstream := range downstreamNodes {
		printReplicationTree(topology, downstream, depth+1)
	}
}