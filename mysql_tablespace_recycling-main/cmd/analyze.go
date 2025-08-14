package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"mysql_tablespace_recycling/recycler"

	"github.com/spf13/cobra"
)

// 分析命令选项
var (
	analyzeOutput     string
	analyzeThreshold  string
	analyzeDatabases  []string
	analyzeExcludeDB  []string
	analyzeTables     []string
	analyzeEngines    []string
	analyzeSortBy     string
	analyzeSortOrder  string
	analyzeLimit      int
	analyzeMinSize    string
	analyzeMaxSize    string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "分析表空间碎片",
	Long: `分析指定MySQL实例的表空间碎片情况。

该命令会扫描数据库中的所有表，分析其碎片化程度，并生成详细的分析报告。
支持多种过滤和排序选项，帮助识别需要优化的表。`,
	Example: `  # 分析本地MySQL实例
  mysql-recycler analyze

  # 分析特定主机的碎片，输出JSON格式
  mysql-recycler analyze --host=192.168.1.100 --output=json

  # 只分析特定数据库，碎片阈值100MB
  mysql-recycler analyze --database=myapp --threshold=100MB

  # 排除系统数据库，按碎片大小排序，限制前10个
  mysql-recycler analyze --exclude-db=mysql,information_schema,performance_schema --sort-by=fragment_size --limit=10`,
	RunE: runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVarP(&analyzeOutput, "output", "o", "table", "输出格式 (table, json)")
	analyzeCmd.Flags().StringVar(&analyzeThreshold, "threshold", "50MB", "碎片大小阈值")
	analyzeCmd.Flags().StringSliceVar(&analyzeDatabases, "database", nil, "指定数据库 (可多次使用)")
	analyzeCmd.Flags().StringSliceVar(&analyzeExcludeDB, "exclude-db", []string{"information_schema", "performance_schema", "mysql", "sys"}, "排除数据库")
	analyzeCmd.Flags().StringSliceVar(&analyzeTables, "table", nil, "指定表名 (支持通配符)")
	analyzeCmd.Flags().StringSliceVar(&analyzeEngines, "engine", []string{"InnoDB"}, "存储引擎")
	analyzeCmd.Flags().StringVar(&analyzeSortBy, "sort-by", "fragment_size", "排序字段 (fragment_size, fragment_ratio, total_size, table_name)")
	analyzeCmd.Flags().StringVar(&analyzeSortOrder, "sort-order", "desc", "排序顺序 (asc, desc)")
	analyzeCmd.Flags().IntVar(&analyzeLimit, "limit", 0, "限制结果数量 (0表示无限制)")
	analyzeCmd.Flags().StringVar(&analyzeMinSize, "min-size", "10MB", "表最小大小")
	analyzeCmd.Flags().StringVar(&analyzeMaxSize, "max-size", "", "表最大大小")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// 创建节点配置
	nodeConfig, err := createNodeConfig()
	if err != nil {
		return fmt.Errorf("failed to create node config: %w", err)
	}

	// 解析阈值
	thresholdBytes, err := parseSize(analyzeThreshold)
	if err != nil {
		return fmt.Errorf("invalid threshold: %w", err)
	}

	minSizeBytes, err := parseSize(analyzeMinSize)
	if err != nil {
		return fmt.Errorf("invalid min-size: %w", err)
	}

	var maxSizeBytes int64
	if analyzeMaxSize != "" {
		maxSizeBytes, err = parseSize(analyzeMaxSize)
		if err != nil {
			return fmt.Errorf("invalid max-size: %w", err)
		}
	}

	// 创建分析选项
	options := &recycler.FragmentationAnalysisOptions{
		MinFragmentSize:  thresholdBytes,
		MinFragmentRatio: 0.01, // 1%最小碎片率
		MinTableSize:     minSizeBytes,
		MaxTableSize:     maxSizeBytes,
		IncludeSchemas:   analyzeDatabases,
		ExcludeSchemas:   analyzeExcludeDB,
		SupportedEngines: analyzeEngines,
		SortBy:           analyzeSortBy,
		SortOrder:        analyzeSortOrder,
		Limit:            analyzeLimit,
	}

	// 创建分析器
	factory := &ConnectionFactory{}
	analyzer := recycler.NewFragmentationAnalyzer(factory)

	// 执行分析
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if verbose {
		fmt.Printf("正在分析 %s 的表空间碎片...\n", nodeConfig.NodeKey())
	}

	report, err := analyzer.AnalyzeFragmentation(ctx, nodeConfig, options)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// 输出结果
	switch analyzeOutput {
	case "json":
		return printJSON(report)
	case "table":
		return printAnalysisReport(report)
	default:
		return fmt.Errorf("unsupported output format: %s", analyzeOutput)
	}
}

// parseSize 解析大小字符串
func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}

	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	
	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		numStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "B") {
		multiplier = 1
		numStr = strings.TrimSuffix(sizeStr, "B")
	} else {
		// 默认为字节
		numStr = sizeStr
	}

	num, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	return int64(num * float64(multiplier)), nil
}

// printAnalysisReport 打印分析报告
func printAnalysisReport(report *recycler.FragmentationReport) error {
	// 打印摘要
	fmt.Printf("MySQL表空间碎片分析报告\n")
	fmt.Printf("==============================\n")
	fmt.Printf("节点: %s:%d\n", report.NodeHost, report.NodePort)
	fmt.Printf("分析时间: %s\n", report.AnalysisTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("总表数: %d\n", report.TotalTables)
	fmt.Printf("碎片化表数: %d\n", len(report.FragmentedTables))
	fmt.Printf("总数据大小: %s\n", formatBytes(report.TotalDataSize))
	fmt.Printf("总索引大小: %s\n", formatBytes(report.TotalIndexSize))
	fmt.Printf("总碎片大小: %s\n", formatBytes(report.TotalFragmentSize))
	fmt.Printf("预计可回收空间: %s\n", formatBytes(report.EstimatedReclaimableSpace))
	
	if report.AverageFragmentRatio > 0 {
		fmt.Printf("平均碎片率: %.2f%%\n", report.AverageFragmentRatio*100)
	}
	
	fmt.Println()

	// 如果没有碎片化表，直接返回
	if len(report.FragmentedTables) == 0 {
		fmt.Println("未发现碎片化表，数据库已优化")
		return nil
	}

	// 打印碎片化表详情
	fmt.Printf("碎片化表详情:\n")
	fmt.Printf("============\n")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "数据库\t表名\t存储引擎\t表大小\t碎片大小\t碎片率\t严重程度\t预估回收时间")
	fmt.Fprintln(w, "------\t----\t--------\t------\t--------\t------\t--------\t-----------")

	factory := &ConnectionFactory{}
	analyzer := recycler.NewFragmentationAnalyzer(factory)

	for i, table := range report.FragmentedTables {
		benefit, _ := analyzer.EstimateReclaimBenefit(table)
		estimatedTime := "未知"
		if benefit != nil {
			estimatedTime = formatDuration(benefit.EstimatedTimeCost)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.2f%%\t%s\t%s\n",
			table.Schema,
			table.TableName,
			table.Engine,
			formatBytes(table.TotalSize),
			formatBytes(table.DataFree),
			table.FragmentRatio*100,
			getSeverityDisplay(table.GetFragmentationSeverity()),
			estimatedTime,
		)

		// 限制显示数量，避免输出过长
		if i >= 19 && len(report.FragmentedTables) > 20 {
			fmt.Fprintf(w, "...\t...\t...\t...\t...\t...\t...\t...\n")
			fmt.Fprintf(w, "(%d more tables)\t\t\t\t\t\t\t\n", len(report.FragmentedTables)-20)
			break
		}
	}
	w.Flush()

	// 显示最大碎片表
	if report.LargestFragmentTable != nil {
		fmt.Printf("\n最大碎片表: %s.%s (%s)\n", 
			report.LargestFragmentTable.Schema,
			report.LargestFragmentTable.TableName,
			formatBytes(report.LargestFragmentTable.DataFree))
	}

	// 显示最高碎片率表
	if report.HighestRatioTable != nil {
		fmt.Printf("最高碎片率表: %s.%s (%.2f%%)\n", 
			report.HighestRatioTable.Schema,
			report.HighestRatioTable.TableName,
			report.HighestRatioTable.FragmentRatio*100)
	}

	// 显示建议
	if len(report.RecommendedActions) > 0 {
		fmt.Printf("\n优化建议:\n")
		fmt.Printf("========\n")
		for i, action := range report.RecommendedActions {
			fmt.Printf("%d. %s\n", i+1, action)
		}
	}

	return nil
}

// getSeverityDisplay 获取严重程度的显示文本
func getSeverityDisplay(severity string) string {
	switch severity {
	case "severe":
		return "严重"
	case "moderate":
		return "中等"
	case "mild":
		return "轻微"
	case "minimal":
		return "微小"
	default:
		return severity
	}
}