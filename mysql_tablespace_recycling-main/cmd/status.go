package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"mysql_tablespace_recycling/monitor"
	"mysql_tablespace_recycling/recycler"

	"github.com/spf13/cobra"
)

// 状态命令选项
var (
	statusTaskID  string
	statusOutput  string
	statusWatch   bool
	statusHistory bool
	statusLimit   int
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看任务状态和监控信息",
	Long: `查看MySQL表空间回收任务的状态、进度和监控信息。

可以查看：
- 当前活跃任务的实时状态
- 特定任务的详细信息
- 任务历史记录
- 系统监控统计信息`,
	Example: `  # 查看所有活跃任务
  mysql-recycler status

  # 查看特定任务状态
  mysql-recycler status --task-id=reclaim-1703123456

  # 查看任务历史（最近10个）
  mysql-recycler status --history --limit=10

  # 持续监控模式
  mysql-recycler status --watch

  # 输出JSON格式
  mysql-recycler status --output=json`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusTaskID, "task-id", "", "特定任务ID")
	statusCmd.Flags().StringVarP(&statusOutput, "output", "o", "table", "输出格式 (table, json)")
	statusCmd.Flags().BoolVar(&statusWatch, "watch", false, "持续监控模式")
	statusCmd.Flags().BoolVar(&statusHistory, "history", false, "显示历史任务")
	statusCmd.Flags().IntVar(&statusLimit, "limit", 20, "限制显示数量")

	// 添加到根命令
	RootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// 创建监控器来读取状态
	monitorConfig := &monitor.MonitorConfig{
		StatusFile:        "/tmp/mysql-recycler-status.json",
		MaxHistoryRecords: 1000,
	}
	monitorInstance := monitor.NewMonitor(monitorConfig)
	defer monitorInstance.Shutdown()

	// 为status命令加载活跃任务（支持跨进程监控）
	if statusMonitor, ok := monitorInstance.(*monitor.DefaultMonitor); ok {
		if err := statusMonitor.LoadActiveTasksForStatus(); err != nil {
			fmt.Printf("Warning: failed to load active tasks: %v\n", err)
		}
	}

	// 持续监控模式
	if statusWatch {
		return runWatchMode(monitorInstance)
	}

	// 查看特定任务
	if statusTaskID != "" {
		return showTaskStatus(monitorInstance, statusTaskID)
	}

	// 显示历史任务
	if statusHistory {
		return showTaskHistory(monitorInstance)
	}

	// 默认显示活跃任务和统计信息
	return showActiveTasksAndStats(monitorInstance)
}

// showActiveTasksAndStats 显示活跃任务和统计信息
func showActiveTasksAndStats(monitorInstance monitor.Monitor) error {
	// 获取活跃任务
	activeTasks, err := monitorInstance.ListActiveTasks()
	if err != nil {
		return fmt.Errorf("failed to get active tasks: %w", err)
	}

	// 获取统计信息
	stats := monitorInstance.GetStats()

	if statusOutput == "json" {
		data := map[string]interface{}{
			"active_tasks": activeTasks,
			"stats":        stats,
			"timestamp":    time.Now(),
		}
		return printJSON(data)
	}

	// 表格格式输出
	fmt.Printf("MySQL表空间回收工具 - 系统状态\n")
	fmt.Printf("===============================\n")
	fmt.Printf("查询时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("运行时长: %s\n", formatDuration(time.Since(stats.UptimeStart)))
	fmt.Println()

	// 统计信息
	fmt.Printf("系统统计:\n")
	fmt.Printf("--------\n")
	fmt.Printf("活跃任务: %d\n", stats.ActiveTasks)
	fmt.Printf("已完成任务: %d\n", stats.CompletedTasks)
	fmt.Printf("失败任务: %d\n", stats.FailedTasks)
	fmt.Printf("总回收空间: %s\n", formatBytes(stats.TotalReclaimedSpace))
	fmt.Printf("总处理表数: %d\n", stats.TotalProcessedTables)

	if stats.AverageTaskDuration > 0 {
		fmt.Printf("平均任务时长: %s\n", formatDuration(stats.AverageTaskDuration))
	}

	if !stats.LastPersistTime.IsZero() {
		fmt.Printf("上次保存: %s\n", stats.LastPersistTime.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	// 活跃任务
	if len(activeTasks) == 0 {
		fmt.Printf("当前无活跃任务\n")
		return nil
	}

	fmt.Printf("活跃任务 (%d个):\n", len(activeTasks))
	fmt.Printf("===============\n")

	for _, task := range activeTasks {
		printTaskSummary(task)
		fmt.Println()
	}

	return nil
}

// showTaskStatus 显示特定任务状态
func showTaskStatus(monitorInstance monitor.Monitor, taskID string) error {
	taskStatus, err := monitorInstance.GetTaskStatus(taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if statusOutput == "json" {
		return printJSON(taskStatus)
	}

	// 详细任务信息
	fmt.Printf("任务详细信息\n")
	fmt.Printf("============\n")
	fmt.Printf("任务ID: %s\n", taskStatus.TaskID)
	fmt.Printf("状态: %s\n", getStatusDisplay(taskStatus.Status))
	fmt.Printf("目标节点: %s\n", taskStatus.Target.Node.NodeKey())

	modeDesc := "独立节点模式"
	if taskStatus.Target.Mode == 2 {
		modeDesc = "复制同步模式"
	}
	fmt.Printf("工作模式: %s\n", modeDesc)
	fmt.Printf("创建时间: %s\n", taskStatus.CreatedAt.Format("2006-01-02 15:04:05"))

	if taskStatus.StartedAt != nil {
		fmt.Printf("开始时间: %s\n", taskStatus.StartedAt.Format("2006-01-02 15:04:05"))
	}

	if taskStatus.CompletedAt != nil {
		fmt.Printf("完成时间: %s\n", taskStatus.CompletedAt.Format("2006-01-02 15:04:05"))
		duration := taskStatus.CompletedAt.Sub(*taskStatus.StartedAt)
		fmt.Printf("执行时长: %s\n", formatDuration(duration))
	}

	fmt.Printf("最后更新: %s\n", taskStatus.LastUpdate.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// 进度信息
	if taskStatus.Progress != nil {
		fmt.Printf("执行进度:\n")
		fmt.Printf("--------\n")
		progress := taskStatus.Progress
		fmt.Printf("总表数: %d\n", progress.TotalTables)
		fmt.Printf("已完成: %d (%.1f%%)\n",
			progress.CompletedTables,
			float64(progress.CompletedTables)/float64(progress.TotalTables)*100)

		if progress.CurrentTable != "" {
			fmt.Printf("当前表: %s\n", progress.CurrentTable)
		}

		fmt.Printf("已处理大小: %s\n", formatBytes(progress.ProcessedSize))
		fmt.Printf("已回收空间: %s\n", formatBytes(progress.ReclaimedSpace))

		if progress.EstimatedRemaining > 0 {
			fmt.Printf("预计剩余时间: %s\n", formatDuration(progress.EstimatedRemaining))
		}

		if progress.ErrorCount > 0 {
			fmt.Printf("错误次数: %d\n", progress.ErrorCount)
		}
		fmt.Println()
	}

	// 结果信息
	if taskStatus.Result != nil {
		fmt.Printf("执行结果:\n")
		fmt.Printf("--------\n")
		result := taskStatus.Result
		fmt.Printf("处理表数: %d\n", result.ProcessedTables)
		fmt.Printf("成功: %d, 失败: %d, 跳过: %d\n",
			result.SuccessfulTables, result.FailedTables, result.SkippedTables)
		fmt.Printf("回收空间: %s\n", formatBytes(result.ReclaimedSpace))
		fmt.Printf("总用时: %s\n", formatDuration(result.TotalTime))

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
	}

	// 错误信息
	if taskStatus.ErrorMessage != "" {
		fmt.Printf("错误信息: %s\n", taskStatus.ErrorMessage)
	}

	// 警告信息
	if len(taskStatus.Warnings) > 0 {
		fmt.Printf("\n警告信息:\n")
		for i, warning := range taskStatus.Warnings {
			fmt.Printf("%d. %s\n", i+1, warning)
		}
	}

	return nil
}

// showTaskHistory 显示任务历史
func showTaskHistory(monitorInstance monitor.Monitor) error {
	history, err := monitorInstance.GetTaskHistory(statusLimit)
	if err != nil {
		return fmt.Errorf("failed to get task history: %w", err)
	}

	if statusOutput == "json" {
		return printJSON(history)
	}

	if len(history) == 0 {
		fmt.Printf("暂无任务历史记录\n")
		return nil
	}

	fmt.Printf("任务历史记录 (最近%d个):\n", len(history))
	fmt.Printf("========================\n")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "任务ID\t状态\t目标节点\t开始时间\t完成时间\t用时\t回收空间")
	fmt.Fprintln(w, "------\t----\t--------\t--------\t--------\t----\t--------")

	for _, task := range history {
		startTime := ""
		if task.StartedAt != nil {
			startTime = task.StartedAt.Format("01-02 15:04")
		}

		completedTime := ""
		duration := ""
		if task.CompletedAt != nil {
			completedTime = task.CompletedAt.Format("01-02 15:04")
			if task.StartedAt != nil {
				duration = formatDuration(task.CompletedAt.Sub(*task.StartedAt))
			}
		}

		reclaimedSpace := ""
		if task.Result != nil {
			reclaimedSpace = formatBytes(task.Result.ReclaimedSpace)
		}

		// 截断长任务ID
		taskID := task.TaskID
		if len(taskID) > 30 {
			taskID = taskID[:30] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			taskID,
			getStatusDisplay(task.Status),
			task.Target.Node.NodeKey(),
			startTime,
			completedTime,
			duration,
			reclaimedSpace,
		)
	}
	w.Flush()

	return nil
}

// runWatchMode 运行持续监控模式
func runWatchMode(monitorInstance monitor.Monitor) error {
	fmt.Printf("进入持续监控模式 (按 Ctrl+C 退出)...\n\n")

	for {
		// 每次循环重新加载活跃任务以获取最新状态
		if statusMonitor, ok := monitorInstance.(*monitor.DefaultMonitor); ok {
			if err := statusMonitor.LoadActiveTasksForStatus(); err != nil {
				fmt.Printf("Warning: failed to reload active tasks: %v\n", err)
			}
		}

		// 清屏
		fmt.Print("\033[2J\033[H")

		// 显示时间戳
		fmt.Printf("MySQL表空间回收工具 - 实时监控\n")
		fmt.Printf("==============================\n")
		fmt.Printf("更新时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

		// 获取并显示活跃任务
		activeTasks, err := monitorInstance.ListActiveTasks()
		if err != nil {
			fmt.Printf("获取任务状态失败: %v\n", err)
		} else if len(activeTasks) == 0 {
			fmt.Printf("当前无活跃任务\n")
		} else {
			for _, task := range activeTasks {
				printTaskProgress(task)
				fmt.Println()
			}
		}

		// 显示统计信息
		stats := monitorInstance.GetStats()
		fmt.Printf("统计: 活跃 %d | 完成 %d | 失败 %d | 总回收 %s\n",
			stats.ActiveTasks,
			stats.CompletedTasks,
			stats.FailedTasks,
			formatBytes(stats.TotalReclaimedSpace))

		// 等待3秒后刷新
		time.Sleep(3 * time.Second)
	}
}

// printTaskSummary 打印任务摘要
func printTaskSummary(task *monitor.TaskStatus) {
	fmt.Printf("任务: %s\n", task.TaskID)
	fmt.Printf("  状态: %s\n", getStatusDisplay(task.Status))
	fmt.Printf("  节点: %s\n", task.Target.Node.NodeKey())

	// 显示任务基本信息
	modeDesc := "独立节点模式"
	if task.Target.Mode == 2 {
		modeDesc = "复制同步模式"
	}
	fmt.Printf("  模式: %s\n", modeDesc)
	fmt.Printf("  并发数: %d\n", task.Target.Concurrency)
	fmt.Printf("  阈值: %s\n", formatBytes(task.Target.Threshold))

	if task.StartedAt != nil {
		elapsed := time.Since(*task.StartedAt)
		fmt.Printf("  已运行: %s\n", formatDuration(elapsed))
	}

	if task.Progress != nil {
		progress := task.Progress
		completionRate := float64(progress.CompletedTables) / float64(progress.TotalTables) * 100
		
		fmt.Printf("  进度: %d/%d (%.1f%%)\n",
			progress.CompletedTables,
			progress.TotalTables,
			completionRate)

		if progress.CurrentTable != "" {
			fmt.Printf("  当前表: %s\n", progress.CurrentTable)
			if progress.CurrentTableSize > 0 {
				fmt.Printf("  当前表大小: %s\n", formatBytes(progress.CurrentTableSize))
			}
		}

		if progress.ProcessedSize > 0 {
			fmt.Printf("  已处理: %s\n", formatBytes(progress.ProcessedSize))
		}

		if progress.ReclaimedSpace > 0 {
			fmt.Printf("  已回收: %s\n", formatBytes(progress.ReclaimedSpace))
		}

		if progress.EstimatedRemaining > 0 {
			fmt.Printf("  预计剩余: %s\n", formatDuration(progress.EstimatedRemaining))
		}

		if progress.ErrorCount > 0 {
			fmt.Printf("  错误次数: %d\n", progress.ErrorCount)
		}

		// 显示处理速度
		if progress.CompletedTables > 0 && task.StartedAt != nil {
			elapsed := time.Since(*task.StartedAt)
			avgTimePerTable := elapsed / time.Duration(progress.CompletedTables)
			fmt.Printf("  平均每表: %s\n", formatDuration(avgTimePerTable))
		}
	}
}

// printTaskProgress 打印任务进度（监控模式）
func printTaskProgress(task *monitor.TaskStatus) {
	fmt.Printf("┌─ 任务: %s\n", task.TaskID)
	fmt.Printf("├─ 节点: %s\n", task.Target.Node.NodeKey())
	fmt.Printf("├─ 状态: %s\n", getStatusDisplay(task.Status))

	// 显示任务配置信息
	modeDesc := "独立节点模式"
	if task.Target.Mode == 2 {
		modeDesc = "复制同步模式"
	}
	fmt.Printf("├─ 模式: %s (并发: %d)\n", modeDesc, task.Target.Concurrency)
	fmt.Printf("├─ 阈值: %s\n", formatBytes(task.Target.Threshold))

	// 显示运行时间
	if task.StartedAt != nil {
		elapsed := time.Since(*task.StartedAt)
		fmt.Printf("├─ 已运行: %s\n", formatDuration(elapsed))
	}

	if task.Progress != nil {
		progress := task.Progress
		completionRate := float64(progress.CompletedTables) / float64(progress.TotalTables)

		// 进度条 - 更宽更美观
		barLength := 40
		filledLength := int(completionRate * float64(barLength))
		bar := "["
		for i := 0; i < barLength; i++ {
			if i < filledLength {
				bar += "█"
			} else {
				bar += "░"
			}
		}
		bar += "]"

		fmt.Printf("├─ 进度: %s %.1f%% (%d/%d)\n",
			bar, completionRate*100, progress.CompletedTables, progress.TotalTables)

		if progress.CurrentTable != "" {
			fmt.Printf("├─ 当前表: %s", progress.CurrentTable)
			if progress.CurrentTableSize > 0 {
				fmt.Printf(" (%s)", formatBytes(progress.CurrentTableSize))
			}
			fmt.Printf("\n")
		}

		// 显示处理统计
		if progress.ProcessedSize > 0 {
			fmt.Printf("├─ 已处理: %s", formatBytes(progress.ProcessedSize))
			if progress.TotalTables > 0 && progress.CompletedTables > 0 {
				// 估算总大小（基于已处理的平均大小）
				avgSize := progress.ProcessedSize / int64(progress.CompletedTables)
				totalEstimated := avgSize * int64(progress.TotalTables)
				if totalEstimated > progress.ProcessedSize {
					remainingSize := totalEstimated - progress.ProcessedSize
					fmt.Printf(" (剩余约: %s)", formatBytes(remainingSize))
				}
			}
			fmt.Printf("\n")
		}

		if progress.ReclaimedSpace > 0 {
			fmt.Printf("├─ 已回收: %s", formatBytes(progress.ReclaimedSpace))
			if progress.ProcessedSize > 0 {
				reclaimeRate := float64(progress.ReclaimedSpace) / float64(progress.ProcessedSize) * 100
				fmt.Printf(" (回收率: %.1f%%)", reclaimeRate)
			}
			fmt.Printf("\n")
		}

		// 显示速度和时间信息
		if progress.CompletedTables > 0 && task.StartedAt != nil {
			elapsed := time.Since(*task.StartedAt)
			avgTimePerTable := elapsed / time.Duration(progress.CompletedTables)
			fmt.Printf("├─ 平均每表: %s", formatDuration(avgTimePerTable))

			// 估算处理速度（表/分钟）
			tablesPerMinute := float64(progress.CompletedTables) / elapsed.Minutes()
			if tablesPerMinute > 0 {
				fmt.Printf(" (%.1f表/分钟)", tablesPerMinute)
			}
			fmt.Printf("\n")
		}

		if progress.EstimatedRemaining > 0 {
			fmt.Printf("├─ 预计剩余: %s", formatDuration(progress.EstimatedRemaining))
			
			// 估算完成时间
			estimatedFinish := time.Now().Add(progress.EstimatedRemaining)
			fmt.Printf(" (完成时间: %s)", estimatedFinish.Format("15:04:05"))
			fmt.Printf("\n")
		}

		if progress.ErrorCount > 0 {
			fmt.Printf("├─ 错误计数: %d\n", progress.ErrorCount)
		}

		// 最后一行
		if task.Progress.LastUpdate.IsZero() {
			fmt.Printf("└─ 最后更新: %s\n", formatDuration(time.Since(task.Progress.StartTime)))
		} else {
			fmt.Printf("└─ 最后更新: %s\n", formatDuration(time.Since(task.Progress.LastUpdate)))
		}
	} else {
		fmt.Printf("└─ 暂无进度信息\n")
	}
}

// getStatusDisplay 获取状态显示文本
func getStatusDisplay(status recycler.TaskStatus) string {
	switch status {
	case recycler.StatusPending:
		return "⏳ 等待中"
	case recycler.StatusRunning:
		return "🔄 运行中"
	case recycler.StatusCompleted:
		return "✅ 已完成"
	case recycler.StatusFailed:
		return "❌ 失败"
	case recycler.StatusCancelled:
		return "⏹️ 已取消"
	case recycler.StatusPaused:
		return "⏸️ 已暂停"
	default:
		return "❓ 未知"
	}
}
