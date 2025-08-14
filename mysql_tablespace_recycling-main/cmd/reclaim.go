package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"mysql_tablespace_recycling/cluster"
	"mysql_tablespace_recycling/monitor"
	"mysql_tablespace_recycling/recycler"

	"github.com/spf13/cobra"
)

// NodeConfigAdapter 适配cluster.NodeConfig到database.NodeConfigInterface
type NodeConfigAdapter struct {
	*cluster.NodeConfig
}

func (n *NodeConfigAdapter) GetHost() string {
	return n.Host
}

func (n *NodeConfigAdapter) GetPort() int {
	return n.Port
}

func (n *NodeConfigAdapter) GetUsername() string {
	return n.Username
}

func (n *NodeConfigAdapter) GetPassword() string {
	return n.Password
}

func (n *NodeConfigAdapter) GetDatabase() string {
	return n.Database
}

func (n *NodeConfigAdapter) GetTimeout() time.Duration {
	return n.Timeout
}

// 回收命令选项
var (
	reclaimMode        int
	reclaimThreshold   string
	reclaimConcurrency int
	reclaimDryRun      bool
	reclaimAllowMaster bool
	reclaimForce       bool
	reclaimDatabases   []string
	reclaimExcludeDB   []string
	reclaimTables      []string
	reclaimMinSize     string
	reclaimMaxSize     string
	reclaimOutput      string
	reclaimWatch       bool
	reclaimStartNode   string
)

var reclaimCmd = &cobra.Command{
	Use:   "reclaim",
	Short: "执行表空间碎片回收",
	Long: `在指定的MySQL节点上执行表空间碎片回收操作。

支持两种工作模式：
1. 独立节点模式（默认）：在每个节点独立执行，关闭binlog
2. 复制同步模式：在指定节点执行，通过binlog同步到下游

该命令包含多重安全检查，默认不在主节点执行操作。`,
	Example: `  # 在从节点执行基本回收操作
  mysql-recycler reclaim --host=192.168.1.101

  # 指定模式和参数
  mysql-recycler reclaim \
    --host=192.168.1.101 \
    --mode=1 \
    --threshold=200MB \
    --concurrency=3

  # 模拟运行，查看将要执行的操作
  mysql-recycler reclaim \
    --host=192.168.1.101 \
    --dry-run

  # 强制在主节点执行（谨慎使用）
  mysql-recycler reclaim \
    --host=192.168.1.100 \
    --allow-master

  # 持续监控回收进度
  mysql-recycler reclaim \
    --host=192.168.1.101 \
    --watch`,
	RunE: runReclaim,
}

func init() {
	reclaimCmd.Flags().IntVar(&reclaimMode, "mode", 1, "工作模式 (1=独立节点模式, 2=复制同步模式)")
	reclaimCmd.Flags().StringVar(&reclaimThreshold, "threshold", "100MB", "碎片回收阈值")
	reclaimCmd.Flags().IntVar(&reclaimConcurrency, "concurrency", 3, "并发数")
	reclaimCmd.Flags().BoolVar(&reclaimDryRun, "dry-run", false, "模拟运行，不实际执行")
	reclaimCmd.Flags().BoolVar(&reclaimAllowMaster, "allow-master", false, "允许在主节点执行")
	reclaimCmd.Flags().BoolVar(&reclaimForce, "force", false, "无需确认直接执行回收操作")
	reclaimCmd.Flags().StringSliceVar(&reclaimDatabases, "database", nil, "指定数据库")
	reclaimCmd.Flags().StringSliceVar(&reclaimExcludeDB, "exclude-db", []string{"information_schema", "performance_schema", "mysql", "sys"}, "排除数据库")
	reclaimCmd.Flags().StringSliceVar(&reclaimTables, "table", nil, "指定表名")
	reclaimCmd.Flags().StringVar(&reclaimMinSize, "min-size", "10MB", "表最小大小")
	reclaimCmd.Flags().StringVar(&reclaimMaxSize, "max-size", "", "表最大大小")
	reclaimCmd.Flags().StringVarP(&reclaimOutput, "output", "o", "table", "输出格式 (table, json)")
	reclaimCmd.Flags().BoolVar(&reclaimWatch, "watch", false, "持续监控进度")
	reclaimCmd.Flags().StringVar(&reclaimStartNode, "start-node", "", "指定起始节点，只对该节点及其下游节点执行回收 (格式: host:port)")

	// 添加到根命令
	RootCmd.AddCommand(reclaimCmd)
}

func runReclaim(cmd *cobra.Command, args []string) error {
	// 创建节点配置
	nodeConfig, err := createNodeConfig()
	if err != nil {
		return fmt.Errorf("failed to create node config: %w", err)
	}

	// 解析参数
	thresholdBytes, err := parseSize(reclaimThreshold)
	if err != nil {
		return fmt.Errorf("invalid threshold: %w", err)
	}

	minSizeBytes, err := parseSize(reclaimMinSize)
	if err != nil {
		return fmt.Errorf("invalid min-size: %w", err)
	}

	var maxSizeBytes int64
	if reclaimMaxSize != "" {
		maxSizeBytes, err = parseSize(reclaimMaxSize)
		if err != nil {
			return fmt.Errorf("invalid max-size: %w", err)
		}
	}

	// 创建组件
	factory := &ConnectionFactory{}
	analyzer := recycler.NewFragmentationAnalyzer(factory)

	// 创建主节点验证器
	clusterFactory := &cluster.DefaultConnectionFactory{}
	discoverer := cluster.NewTopologyDiscoverer(clusterFactory, &cluster.DiscoveryOptions{
		MaxConcurrency: 3,
		Timeout:        30 * time.Second,
		MaxDepth:       3, // 增加深度以发现完整拓扑
	})
	validator := cluster.NewMasterValidator(discoverer, nil, clusterFactory, false)

	// 如果指定了起始节点，需要先发现集群拓扑并过滤节点
	var targetNodes []*cluster.NodeConfig
	if reclaimStartNode != "" {
		if verbose {
			fmt.Printf("发现集群拓扑以确定回收范围...\n")
		}
		
		// 转换当前节点配置用于拓扑发现
		entryPoint := &cluster.NodeConfig{
			Host:     nodeConfig.Host,
			Port:     nodeConfig.Port,
			Username: nodeConfig.Username,
			Password: nodeConfig.Password,
			Database: nodeConfig.Database,
			Timeout:  nodeConfig.Timeout,
		}
		
		// 发现集群拓扑
		discoverCtx, discoverCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer discoverCancel()
		topology, err := discoverer.DiscoverTopology(discoverCtx, entryPoint)
		if err != nil {
			return fmt.Errorf("failed to discover cluster topology: %w", err)
		}
		
		// 过滤出指定起始节点及其下游节点
		targetNodes = topology.FilterNodesByStartNode(reclaimStartNode)
		if len(targetNodes) == 0 {
			return fmt.Errorf("未找到起始节点 %s 或其下游节点", reclaimStartNode)
		}
		
		if verbose {
			fmt.Printf("找到 %d 个目标节点进行回收: ", len(targetNodes))
			for i, node := range targetNodes {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(node.NodeKey())
			}
			fmt.Println()
		}
	} else {
		// 如果没有指定起始节点，只对当前节点进行回收
		targetNodes = []*cluster.NodeConfig{{
			Host:     nodeConfig.Host,
			Port:     nodeConfig.Port,
			Username: nodeConfig.Username,
			Password: nodeConfig.Password,
			Database: nodeConfig.Database,
			Timeout:  nodeConfig.Timeout,
		}}
	}

	// 创建回收器
	recyclerInstance := recycler.NewTablespaceRecycler(factory, analyzer, &MasterValidatorAdapter{validator})

	// 创建监控器
	monitorConfig := &monitor.MonitorConfig{
		StatusFile:        "/tmp/mysql-recycler-status.json",
		MaxHistoryRecords: 100,
		PersistInterval:   30 * time.Second,
		EnableAutoSave:    true,
	}
	monitorInstance := monitor.NewMonitor(monitorConfig)
	defer monitorInstance.Shutdown()

	// 注册通知钩子
	notificationHook := monitor.NewNotificationHook("")
	monitorInstance.RegisterHook(notificationHook)

	// 为所有目标节点分析表空间碎片并创建回收任务
	analysisOptions := &recycler.FragmentationAnalysisOptions{
		MinFragmentSize:  thresholdBytes,
		MinFragmentRatio: 0.01,
		MinTableSize:     minSizeBytes,
		MaxTableSize:     maxSizeBytes,
		IncludeSchemas:   reclaimDatabases,
		ExcludeSchemas:   reclaimExcludeDB,
		SupportedEngines: []string{"InnoDB"},
		SortBy:           "fragment_size",
		SortOrder:        "desc",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var allTasks []*recycler.RecycleTask
	totalFragmentedTables := 0
	totalReclaimableSpace := int64(0)

	// 对每个目标节点进行分析
	for i, targetNode := range targetNodes {
		if verbose {
			fmt.Printf("正在分析节点 %s 的表空间碎片... (%d/%d)\n", 
				targetNode.NodeKey(), i+1, len(targetNodes))
		}

		// 创建节点配置适配器
		nodeConfigAdapter := &NodeConfigAdapter{targetNode}
		
		report, err := analyzer.AnalyzeFragmentation(ctx, nodeConfigAdapter, analysisOptions)
		if err != nil {
			fmt.Printf("警告: 节点 %s 分析失败: %v\n", targetNode.NodeKey(), err)
			continue
		}

		if len(report.FragmentedTables) == 0 {
			if verbose {
				fmt.Printf("节点 %s 未发现需要回收的碎片化表\n", targetNode.NodeKey())
			}
			continue
		}

		totalFragmentedTables += len(report.FragmentedTables)
		totalReclaimableSpace += report.EstimatedReclaimableSpace

		// 创建回收任务
		taskID := fmt.Sprintf("reclaim-%s-%d", targetNode.NodeKey(), time.Now().Unix())
		
		// 获取超时配置
		timeoutConfig, err := createTimeoutConfig()
		if err != nil {
			return fmt.Errorf("failed to create timeout config: %w", err)
		}
		
		// 设置新的超时配置
		targetNode.ConnectTimeout = timeoutConfig.Connect
		targetNode.QueryTimeout = timeoutConfig.Query
		targetNode.AlterTimeout = timeoutConfig.Alter
		targetNode.DiscoveryTimeout = timeoutConfig.Discovery
		
		task := &recycler.RecycleTask{
			ID:        taskID,
			CreatedAt: time.Now(),
			Status:    recycler.StatusPending,
			Target: &recycler.RecycleTarget{
				Node:         targetNode,
				Mode:         recycler.RecycleMode(reclaimMode),
				Threshold:    thresholdBytes,
				Concurrency:  reclaimConcurrency,
				DryRun:       reclaimDryRun,
				SafetyChecks: !reclaimAllowMaster,
			},
			Tables: report.FragmentedTables,
		}

		allTasks = append(allTasks, task)
		
		if verbose {
			fmt.Printf("节点 %s: 发现 %d 个碎片化表，预计可回收 %s 空间\n",
				targetNode.NodeKey(),
				len(report.FragmentedTables),
				formatBytes(report.EstimatedReclaimableSpace))
		}
	}

	if len(allTasks) == 0 {
		fmt.Println("所有目标节点均未发现需要回收的碎片化表")
		return nil
	}

	// 显示总体分析结果
	fmt.Printf("共发现 %d 个碎片化表，预计可回收 %s 空间\n",
		totalFragmentedTables,
		formatBytes(totalReclaimableSpace))

	// 显示即将执行的操作
	if err := printReclaimPlans(allTasks); err != nil {
		return err
	}

	// 确认执行
	if !reclaimDryRun {
		fmt.Print("\n是否继续执行回收操作？(y/N): ")
		var response string
		if !reclaimForce {
			fmt.Scanln(&response)
		} else {
			fmt.Println("Y（强制回收无需确认）")
			response = "y"
		}
		if response != "y" && response != "Y" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 执行所有回收任务
	fmt.Printf("\n开始执行表空间回收 (共%d个任务)...\n", len(allTasks))
	
	var allResults []*recycler.RecycleResult
	for i, task := range allTasks {
		fmt.Printf("\n执行任务 %d/%d: %s\n", i+1, len(allTasks), task.Target.Node.NodeKey())
		
		// 开始监控任务
		if err := monitorInstance.StartTask(task); err != nil {
			fmt.Printf("警告: 启动监控失败: %v\n", err)
		}

		// 注册进度回调
		if progressRecycler, ok := recyclerInstance.(*recycler.DefaultTablespaceRecycler); ok {
			progressRecycler.RegisterProgressCallback(func(progress *recycler.TaskProgress) {
				monitorInstance.UpdateProgress(task.ID, progress)

				if reclaimWatch {
					// --watch 模式：显示详细进度
					printProgress(progress)
				} else {
					// 默认模式：显示简单进度
					printSimpleProgress(progress)
				}
			})
		}
		
		// 通知监控器任务开始运行
		if err := monitorInstance.UpdateTaskStatus(task.ID, recycler.StatusRunning); err != nil {
			fmt.Printf("警告: 更新任务状态失败: %v\n", err)
		}
		
		result, err := recyclerInstance.ExecuteRecycle(ctx, task)

		// 完成监控
		if monitorErr := monitorInstance.CompleteTask(task.ID, result, err); monitorErr != nil {
			fmt.Printf("警告: 完成任务监控失败: %v\n", monitorErr)
		}

		if err != nil {
			fmt.Printf("任务 %s 执行失败: %v\n", task.ID, err)
			continue
		}

		allResults = append(allResults, result)
		fmt.Printf("任务 %s 执行完成\n", task.ID)
	}

	// 显示总体结果
	return printReclaimResults(allResults)
}

// printReclaimPlans 打印多个回收计划
func printReclaimPlans(tasks []*recycler.RecycleTask) error {
	fmt.Printf("\n回收计划:\n")
	fmt.Printf("========\n")
	fmt.Printf("总任务数: %d\n", len(tasks))
	
	totalTables := 0
	for _, task := range tasks {
		totalTables += len(task.Tables)
	}
	fmt.Printf("总待处理表数: %d\n", totalTables)
	
	for i, task := range tasks {
		fmt.Printf("\n任务 %d: %s\n", i+1, task.Target.Node.NodeKey())
		fmt.Printf("- 任务ID: %s\n", task.ID)
		
		modeDesc := "独立节点模式 (sql_log_bin=OFF)"
		if task.Target.Mode == recycler.ModeReplication {
			modeDesc = "复制同步模式 (sql_log_bin=ON)"
		}
		fmt.Printf("- 工作模式: %s\n", modeDesc)
		fmt.Printf("- 并发数: %d\n", task.Target.Concurrency)
		fmt.Printf("- 待处理表数: %d\n", len(task.Tables))
		
		if task.Target.DryRun {
			fmt.Printf("- 运行模式: 模拟运行 (不实际执行)\n")
		} else {
			fmt.Printf("- 运行模式: 实际执行\n")
		}
	}
	
	// 显示前几个表的详情
	fmt.Printf("\n待处理表详情 (前10个):\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "节点\t数据库\t表名\t表大小\t碎片大小\t碎片率")
	fmt.Fprintln(w, "----\t------\t----\t------\t--------\t------")

	count := 0
	for _, task := range tasks {
		for _, table := range task.Tables {
			if count >= 10 {
				fmt.Fprintf(w, "...\t...\t...\t...\t...\t...\n")
				fmt.Fprintf(w, "还有%d个表\t\t\t\t\t\n", totalTables-10)
				w.Flush()
				return nil
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.2f%%\n",
				task.Target.Node.NodeKey(),
				table.Schema,
				table.TableName,
				formatBytes(table.TotalSize),
				formatBytes(table.DataFree),
				table.FragmentRatio*100,
			)
			count++
		}
	}
	w.Flush()

	return nil
}

// printReclaimPlan 打印回收计划
func printReclaimPlan(task *recycler.RecycleTask) error {
	fmt.Printf("\n回收计划:\n")
	fmt.Printf("========\n")
	fmt.Printf("任务ID: %s\n", task.ID)
	fmt.Printf("目标节点: %s\n", task.Target.Node.NodeKey())

	modeDesc := "独立节点模式 (sql_log_bin=OFF)"
	if task.Target.Mode == recycler.ModeReplication {
		modeDesc = "复制同步模式 (sql_log_bin=ON)"
	}
	fmt.Printf("工作模式: %s\n", modeDesc)
	fmt.Printf("并发数: %d\n", task.Target.Concurrency)
	fmt.Printf("碎片阈值: %s\n", formatBytes(task.Target.Threshold))

	if task.Target.DryRun {
		fmt.Printf("运行模式: 模拟运行 (不实际执行)\n")
	} else {
		fmt.Printf("运行模式: 实际执行\n")
	}

	fmt.Printf("待处理表数: %d\n", len(task.Tables))

	// 显示前几个表的详情
	fmt.Printf("\n待处理表 (前10个):\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "数据库\t表名\t表大小\t碎片大小\t碎片率\t预估时间")
	fmt.Fprintln(w, "------\t----\t------\t--------\t------\t--------")

	count := 0
	for _, table := range task.Tables {
		if count >= 10 {
			fmt.Fprintf(w, "...\t...\t...\t...\t...\t...\n")
			fmt.Fprintf(w, "还有%d个表\t\t\t\t\t\n", len(task.Tables)-10)
			break
		}

		// 估算单表处理时间
		estimatedTime := estimateTableReclaimTime(table)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.2f%%\t%s\n",
			table.Schema,
			table.TableName,
			formatBytes(table.TotalSize),
			formatBytes(table.DataFree),
			table.FragmentRatio*100,
			formatDuration(estimatedTime),
		)
		count++
	}
	w.Flush()

	return nil
}

// printProgress 打印进度信息
func printProgress(progress *recycler.TaskProgress) {
	fmt.Printf("\r进度: %d/%d 表 (%.1f%%) | 当前: %s | 已回收: %s | 预计剩余: %s",
		progress.CompletedTables,
		progress.TotalTables,
		float64(progress.CompletedTables)/float64(progress.TotalTables)*100,
		progress.CurrentTable,
		formatBytes(progress.ReclaimedSpace),
		formatDuration(progress.EstimatedRemaining),
	)
}

// printSimpleProgress 打印简单进度信息
func printSimpleProgress(progress *recycler.TaskProgress) {
	// 只在任务开始和表完成时打印
	if progress.CompletedTables == 0 && progress.CurrentTable != "" {
		// 任务开始
		fmt.Printf("开始处理: %s (%d/%d)\n", progress.CurrentTable, progress.CompletedTables+1, progress.TotalTables)
	} else if progress.CompletedTables > 0 {
		// 表完成，显示进度更新
		percentage := float64(progress.CompletedTables) / float64(progress.TotalTables) * 100
		fmt.Printf("已完成: %d/%d 表 (%.1f%%) | 已回收: %s\n", 
			progress.CompletedTables, 
			progress.TotalTables, 
			percentage,
			formatBytes(progress.ReclaimedSpace))
	}
}

// printReclaimResult 打印回收结果
func printReclaimResult(result *recycler.RecycleResult) error {
	fmt.Printf("\n回收完成!\n")
	fmt.Printf("==========\n")
	fmt.Printf("任务ID: %s\n", result.TaskID)
	fmt.Printf("处理时间: %s\n", formatDuration(result.TotalTime))
	fmt.Printf("处理表数: %d\n", result.ProcessedTables)
	fmt.Printf("成功: %d, 失败: %d, 跳过: %d\n",
		result.SuccessfulTables, result.FailedTables, result.SkippedTables)
	fmt.Printf("回收空间: %s\n", formatBytes(result.ReclaimedSpace))

	if result.SuccessfulTables > 0 {
		avgTime := result.TotalTime / time.Duration(result.SuccessfulTables)
		fmt.Printf("平均每表用时: %s\n", formatDuration(avgTime))
	}

	// 显示错误和警告
	if len(result.Errors) > 0 {
		fmt.Printf("\n错误信息:\n")
		for i, err := range result.Errors {
			fmt.Printf("%d. %s\n", i+1, err)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n警告信息:\n")
		for i, warning := range result.Warnings {
			fmt.Printf("%d. %s\n", i+1, warning)
		}
	}

	// 详细的表处理结果
	if len(result.TableResults) > 0 {
		fmt.Printf("\n详细结果:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "数据库\t表名\t状态\t回收空间\t用时\t备注")
		fmt.Fprintln(w, "------\t----\t----\t--------\t----\t----")

		for _, tr := range result.TableResults {
			status := "✓"
			if tr.Status == "failed" {
				status = "✗"
			} else if tr.Status == "dry_run" {
				status = "○"
			}

			note := ""
			if tr.Error != "" {
				note = tr.Error
				if len(note) > 30 {
					note = note[:30] + "..."
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				tr.Schema,
				tr.TableName,
				status,
				formatBytes(tr.ReclaimedSpace),
				formatDuration(tr.Duration),
				note,
			)
		}
		w.Flush()
	}

	return nil
}

// estimateTableReclaimTime 估算单个表的回收时间
func estimateTableReclaimTime(table *recycler.TableFragmentation) time.Duration {
	// 基于表大小的简单估算：每GB约需2分钟
	sizeMB := float64(table.TotalSize) / (1024 * 1024)
	estimatedMinutes := sizeMB / 1024 * 2 // 每GB 2分钟

	// 最少30秒，最多30分钟
	if estimatedMinutes < 0.5 {
		estimatedMinutes = 0.5
	} else if estimatedMinutes > 30 {
		estimatedMinutes = 30
	}

	return time.Duration(estimatedMinutes * float64(time.Minute))
}

// MasterValidatorAdapter 适配器
type MasterValidatorAdapter struct {
	validator cluster.MasterValidator
}

func (a *MasterValidatorAdapter) ValidateMasterSafety(ctx context.Context, node recycler.NodeConfigInterface) error {
	clusterNode := &cluster.NodeConfig{
		Host:     node.GetHost(),
		Port:     node.GetPort(),
		Username: node.GetUsername(),
		Password: node.GetPassword(),
		Database: node.GetDatabase(),
		Timeout:  node.GetTimeout(),
	}

	return a.validator.ValidateMasterSafety(ctx, clusterNode)
}

// printReclaimResults 打印多个回收结果
func printReclaimResults(results []*recycler.RecycleResult) error {
	fmt.Printf("\n所有回收任务完成!\n")
	fmt.Printf("===================\n")
	fmt.Printf("总任务数: %d\n", len(results))
	
	totalProcessed := 0
	totalSuccessful := 0
	totalFailed := 0
	totalSkipped := 0
	totalReclaimedSpace := int64(0)
	totalTime := time.Duration(0)
	
	for _, result := range results {
		totalProcessed += result.ProcessedTables
		totalSuccessful += result.SuccessfulTables
		totalFailed += result.FailedTables
		totalSkipped += result.SkippedTables
		totalReclaimedSpace += result.ReclaimedSpace
		totalTime += result.TotalTime
	}
	
	fmt.Printf("总处理表数: %d\n", totalProcessed)
	fmt.Printf("成功: %d, 失败: %d, 跳过: %d\n", totalSuccessful, totalFailed, totalSkipped)
	fmt.Printf("总回收空间: %s\n", formatBytes(totalReclaimedSpace))
	fmt.Printf("总处理时间: %s\n", formatDuration(totalTime))
	
	if totalSuccessful > 0 {
		avgTime := totalTime / time.Duration(totalSuccessful)
		fmt.Printf("平均每表用时: %s\n", formatDuration(avgTime))
	}
	
	// 显示各节点的结果摘要
	fmt.Printf("\n各节点结果摘要:\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "任务ID\t节点\t处理表数\t成功\t失败\t跳过\t回收空间\t用时")
	fmt.Fprintln(w, "------\t----\t------\t----\t----\t----\t--------\t----")
	
	for _, result := range results {
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\n",
			result.TaskID,
			"N/A", // 这里需要从result中获取节点信息，或者传递额外参数
			result.ProcessedTables,
			result.SuccessfulTables,
			result.FailedTables,
			result.SkippedTables,
			formatBytes(result.ReclaimedSpace),
			formatDuration(result.TotalTime),
		)
	}
	w.Flush()
	
	// 收集并显示所有错误和警告
	var allErrors []string
	var allWarnings []string
	
	for _, result := range results {
		allErrors = append(allErrors, result.Errors...)
		allWarnings = append(allWarnings, result.Warnings...)
	}
	
	if len(allErrors) > 0 {
		fmt.Printf("\n所有错误信息:\n")
		for i, err := range allErrors {
			fmt.Printf("%d. %s\n", i+1, err)
		}
	}

	if len(allWarnings) > 0 {
		fmt.Printf("\n所有警告信息:\n")
		for i, warning := range allWarnings {
			fmt.Printf("%d. %s\n", i+1, warning)
		}
	}
	
	return nil
}
