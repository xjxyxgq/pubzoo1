package recycler

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"mysql_tablespace_recycling/pkg/database"
)

// DefaultFragmentationAnalyzer 默认碎片分析器实现
type DefaultFragmentationAnalyzer struct {
	connectionFactory database.ConnectionFactory
}

// ConnectionFactory 连接工厂接口 - 使用通用接口
type ConnectionFactory = database.ConnectionFactory

// DBConnection 数据库连接接口 - 使用通用接口
type DBConnection = database.DBConnection

// NodeConfigInterface 节点配置接口 - 使用通用接口
type NodeConfigInterface = database.NodeConfigInterface

// NewFragmentationAnalyzer 创建碎片分析器
func NewFragmentationAnalyzer(connectionFactory ConnectionFactory) FragmentationAnalyzer {
	return &DefaultFragmentationAnalyzer{
		connectionFactory: connectionFactory,
	}
}

// AnalyzeFragmentation 分析表空间碎片
func (a *DefaultFragmentationAnalyzer) AnalyzeFragmentation(ctx context.Context, nodeConfig NodeConfigInterface, options *FragmentationAnalysisOptions) (*FragmentationReport, error) {
	if options == nil {
		options = GetDefaultAnalysisOptions()
	}
	
	conn, err := a.connectionFactory.CreateConnection(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", nodeConfig.NodeKey(), err)
	}
	defer conn.Close()
	
	report := &FragmentationReport{
		NodeHost:        nodeConfig.GetHost(),
		NodePort:        nodeConfig.GetPort(),
		AnalysisTime:    time.Now(),
		AnalysisOptions: options,
	}
	
	// 构建查询SQL
	query, args := a.buildFragmentationQuery(options)
	
	// 执行查询
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query table information: %w", err)
	}
	defer rows.Close()
	
	// 解析结果
	var allTables []*TableFragmentation
	var fragmentedTables []*TableFragmentation
	
	for rows.Next() {
		table, err := a.scanTableFragmentation(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		
		// 计算碎片化指标
		table.CalculateFragmentationMetrics()
		allTables = append(allTables, table)
		
		// 检查是否需要整理
		if table.IsFragmented(options) {
			fragmentedTables = append(fragmentedTables, table)
		}
		
		// 更新统计信息
		report.TotalDataSize += table.DataLength
		report.TotalIndexSize += table.IndexLength
		report.TotalFragmentSize += table.DataFree
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table rows: %w", err)
	}
	
	// 排序碎片化表
	a.sortFragmentedTables(fragmentedTables, options)
	
	// 应用限制
	if options.Limit > 0 && len(fragmentedTables) > options.Limit {
		fragmentedTables = fragmentedTables[:options.Limit]
	}
	
	// 填充报告
	report.TotalTables = len(allTables)
	report.FragmentedTables = fragmentedTables
	report.EstimatedReclaimableSpace = a.calculateReclaimableSpace(fragmentedTables)
	
	if len(fragmentedTables) > 0 {
		report.AverageFragmentRatio = float64(report.TotalFragmentSize) / float64(report.TotalDataSize+report.TotalIndexSize)
		report.LargestFragmentTable = a.findLargestFragmentTable(fragmentedTables)
		report.HighestRatioTable = a.findHighestRatioTable(fragmentedTables)
	}
	
	report.RecommendedActions = a.generateRecommendations(report)
	
	return report, nil
}

// GetTableFragmentation 获取单个表的碎片信息
func (a *DefaultFragmentationAnalyzer) GetTableFragmentation(ctx context.Context, nodeConfig NodeConfigInterface, schema, tableName string) (*TableFragmentation, error) {
	conn, err := a.connectionFactory.CreateConnection(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", nodeConfig.NodeKey(), err)
	}
	defer conn.Close()
	
	query := `
		SELECT 
			table_schema, table_name, engine, table_rows, avg_row_length,
			data_length, max_data_length, index_length, data_free,
			auto_increment, create_time, update_time, check_time,
			table_collation, checksum, create_options, table_comment
		FROM information_schema.tables
		WHERE table_schema = ? AND table_name = ? AND table_type = 'BASE TABLE'
	`
	
	rows, err := conn.Query(query, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query table information: %w", err)
	}
	defer rows.Close()
	
	if !rows.Next() {
		return nil, fmt.Errorf("table %s.%s not found", schema, tableName)
	}
	
	table, err := a.scanTableFragmentation(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan table row: %w", err)
	}
	
	table.CalculateFragmentationMetrics()
	return table, nil
}

// GetSchemaFragmentation 获取指定数据库的碎片信息
func (a *DefaultFragmentationAnalyzer) GetSchemaFragmentation(ctx context.Context, nodeConfig NodeConfigInterface, schema string, options *FragmentationAnalysisOptions) ([]*TableFragmentation, error) {
	if options == nil {
		options = GetDefaultAnalysisOptions()
	}
	
	// 临时修改选项，只查询指定数据库
	tempOptions := *options
	tempOptions.IncludeSchemas = []string{schema}
	tempOptions.ExcludeSchemas = nil
	
	report, err := a.AnalyzeFragmentation(ctx, nodeConfig, &tempOptions)
	if err != nil {
		return nil, err
	}
	
	return report.FragmentedTables, nil
}

// EstimateReclaimBenefit 估算回收收益
func (a *DefaultFragmentationAnalyzer) EstimateReclaimBenefit(fragmentation *TableFragmentation) (*ReclaimBenefit, error) {
	benefit := &ReclaimBenefit{
		EstimatedReclaimedSpace: fragmentation.DataFree,
		Benefits:               []string{},
		Risks:                  []string{},
	}
	
	// 估算时间成本（基于表大小）
	totalSizeMB := fragmentation.TotalSize / (1024 * 1024)
	if totalSizeMB < 100 {
		benefit.EstimatedTimeCost = 1 * time.Minute
	} else if totalSizeMB < 1000 {
		benefit.EstimatedTimeCost = time.Duration(totalSizeMB/10) * time.Minute
	} else {
		benefit.EstimatedTimeCost = time.Duration(totalSizeMB/5) * time.Minute
	}
	
	// 计算优先级（基于碎片大小和比例）
	sizeScore := int(fragmentation.DataFree / (100 * 1024 * 1024)) // 每100MB得1分
	ratioScore := int(fragmentation.FragmentRatio * 100)           // 百分比得分
	benefit.Priority = sizeScore + ratioScore
	
	// 评估风险级别
	if fragmentation.TotalSize > 10*1024*1024*1024 { // 超过10GB
		benefit.RiskLevel = RiskHigh
		benefit.Risks = append(benefit.Risks, "Large table size may cause extended table lock time")
	} else if fragmentation.TotalSize > 1*1024*1024*1024 { // 超过1GB
		benefit.RiskLevel = RiskMedium
		benefit.Risks = append(benefit.Risks, "Medium table size requires careful scheduling")
	} else {
		benefit.RiskLevel = RiskLow
	}
	
	// 添加收益说明
	if fragmentation.FragmentRatio > 0.2 {
		benefit.Benefits = append(benefit.Benefits, fmt.Sprintf("High fragmentation ratio (%.1f%%) indicates significant space waste", fragmentation.FragmentRatio*100))
	}
	
	if fragmentation.DataFree > 500*1024*1024 {
		benefit.Benefits = append(benefit.Benefits, fmt.Sprintf("Large fragment size (%s) can be reclaimed", formatBytes(fragmentation.DataFree)))
	}
	
	// 生成建议
	if benefit.Priority > 50 && benefit.RiskLevel != RiskHigh {
		benefit.Recommendation = "Highly recommended for optimization"
	} else if benefit.Priority > 20 {
		benefit.Recommendation = "Recommended for optimization during maintenance window"
	} else {
		benefit.Recommendation = "Consider optimization if other factors permit"
	}
	
	return benefit, nil
}

// buildFragmentationQuery 构建碎片查询SQL
func (a *DefaultFragmentationAnalyzer) buildFragmentationQuery(options *FragmentationAnalysisOptions) (string, []interface{}) {
	query := `
		SELECT 
			table_schema, table_name, engine, table_rows, avg_row_length,
			data_length, max_data_length, index_length, data_free,
			auto_increment, create_time, update_time, check_time,
			table_collation, checksum, create_options, table_comment
		FROM information_schema.tables
		WHERE table_type = 'BASE TABLE'
	`
	
	var conditions []string
	var args []interface{}
	
	// 包含数据库条件
	if len(options.IncludeSchemas) > 0 {
		placeholders := strings.Repeat("?,", len(options.IncludeSchemas))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("table_schema IN (%s)", placeholders))
		for _, schema := range options.IncludeSchemas {
			args = append(args, schema)
		}
	}
	
	// 排除数据库条件
	if len(options.ExcludeSchemas) > 0 {
		placeholders := strings.Repeat("?,", len(options.ExcludeSchemas))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("table_schema NOT IN (%s)", placeholders))
		for _, schema := range options.ExcludeSchemas {
			args = append(args, schema)
		}
	}
	
	// 存储引擎条件
	if len(options.SupportedEngines) > 0 {
		placeholders := strings.Repeat("?,", len(options.SupportedEngines))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("engine IN (%s)", placeholders))
		for _, engine := range options.SupportedEngines {
			args = append(args, engine)
		}
	}
	
	// 表大小条件
	if options.MinTableSize > 0 {
		conditions = append(conditions, "(data_length + index_length) >= ?")
		args = append(args, options.MinTableSize)
	}
	
	if options.MaxTableSize > 0 {
		conditions = append(conditions, "(data_length + index_length) <= ?")
		args = append(args, options.MaxTableSize)
	}
	
	// 添加WHERE条件
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	
	// 添加排序
	orderBy := "data_free DESC" // 默认按碎片大小降序
	if options.SortBy != "" {
		switch options.SortBy {
		case "fragment_size":
			orderBy = "data_free"
		case "fragment_ratio":
			orderBy = "data_free / GREATEST(data_length + index_length, 1)"
		case "total_size":
			orderBy = "data_length + index_length"
		case "table_name":
			orderBy = "table_schema, table_name"
		default:
			orderBy = "data_free"
		}
		
		if options.SortOrder == "asc" {
			orderBy += " ASC"
		} else {
			orderBy += " DESC"
		}
	}
	query += " ORDER BY " + orderBy
	
	return query, args
}

// scanTableFragmentation 扫描表碎片信息
func (a *DefaultFragmentationAnalyzer) scanTableFragmentation(rows *sql.Rows) (*TableFragmentation, error) {
	var table TableFragmentation
	var autoIncrement sql.NullInt64
	var createTime sql.NullTime
	var updateTime sql.NullTime
	var checkTime sql.NullTime
	var checksum sql.NullInt64
	
	err := rows.Scan(
		&table.Schema, &table.TableName, &table.Engine, &table.TableRows, &table.AvgRowLength,
		&table.DataLength, &table.MaxDataLength, &table.IndexLength, &table.DataFree,
		&autoIncrement, &createTime, &updateTime, &checkTime,
		&table.TableCollation, &checksum, &table.CreateOptions, &table.TableComment,
	)
	
	if err != nil {
		return nil, err
	}
	
	// 处理可空字段
	if autoIncrement.Valid {
		table.AutoIncrement = &autoIncrement.Int64
	}
	if createTime.Valid {
		table.CreateTime = &createTime.Time
	}
	if updateTime.Valid {
		table.UpdateTime = &updateTime.Time
	}
	if checkTime.Valid {
		table.CheckTime = &checkTime.Time
	}
	if checksum.Valid {
		table.Checksum = &checksum.Int64
	}
	
	return &table, nil
}

// sortFragmentedTables 排序碎片化表
func (a *DefaultFragmentationAnalyzer) sortFragmentedTables(tables []*TableFragmentation, options *FragmentationAnalysisOptions) {
	sort.Slice(tables, func(i, j int) bool {
		var less bool
		switch options.SortBy {
		case "fragment_ratio":
			less = tables[i].FragmentRatio > tables[j].FragmentRatio
		case "total_size":
			less = tables[i].TotalSize > tables[j].TotalSize
		case "table_name":
			less = tables[i].Schema < tables[j].Schema || 
				   (tables[i].Schema == tables[j].Schema && tables[i].TableName < tables[j].TableName)
		default: // fragment_size
			less = tables[i].DataFree > tables[j].DataFree
		}
		
		if options.SortOrder == "asc" {
			return !less
		}
		return less
	})
}

// calculateReclaimableSpace 计算可回收空间
func (a *DefaultFragmentationAnalyzer) calculateReclaimableSpace(tables []*TableFragmentation) int64 {
	var total int64
	for _, table := range tables {
		// 通常可以回收80-90%的碎片空间
		total += int64(float64(table.DataFree) * 0.85)
	}
	return total
}

// findLargestFragmentTable 找到碎片最大的表
func (a *DefaultFragmentationAnalyzer) findLargestFragmentTable(tables []*TableFragmentation) *TableFragmentation {
	if len(tables) == 0 {
		return nil
	}
	
	largest := tables[0]
	for _, table := range tables[1:] {
		if table.DataFree > largest.DataFree {
			largest = table
		}
	}
	return largest
}

// findHighestRatioTable 找到碎片率最高的表
func (a *DefaultFragmentationAnalyzer) findHighestRatioTable(tables []*TableFragmentation) *TableFragmentation {
	if len(tables) == 0 {
		return nil
	}
	
	highest := tables[0]
	for _, table := range tables[1:] {
		if table.FragmentRatio > highest.FragmentRatio {
			highest = table
		}
	}
	return highest
}

// generateRecommendations 生成优化建议
func (a *DefaultFragmentationAnalyzer) generateRecommendations(report *FragmentationReport) []string {
	var recommendations []string
	
	if len(report.FragmentedTables) == 0 {
		recommendations = append(recommendations, "No fragmented tables found, database is well optimized")
		return recommendations
	}
	
	// 基于碎片数量的建议
	if len(report.FragmentedTables) > 20 {
		recommendations = append(recommendations, "High number of fragmented tables detected, consider scheduling regular maintenance")
	}
	
	// 基于碎片大小的建议
	if report.TotalFragmentSize > 1024*1024*1024 { // > 1GB
		recommendations = append(recommendations, "Significant fragmentation detected (>1GB), optimization will improve performance")
	}
	
	// 基于碎片率的建议
	if report.AverageFragmentRatio > 0.2 {
		recommendations = append(recommendations, "High average fragmentation ratio (>20%), urgent optimization recommended")
	} else if report.AverageFragmentRatio > 0.1 {
		recommendations = append(recommendations, "Moderate fragmentation detected, optimization recommended during maintenance window")
	}
	
	// 针对最大碎片表的建议
	if report.LargestFragmentTable != nil && report.LargestFragmentTable.DataFree > 500*1024*1024 {
		recommendations = append(recommendations, 
			fmt.Sprintf("Table %s.%s has %s fragmented space, high priority for optimization",
				report.LargestFragmentTable.Schema, 
				report.LargestFragmentTable.TableName,
				formatBytes(report.LargestFragmentTable.DataFree)))
	}
	
	// 运行时间建议
	totalEstimatedTime := time.Duration(0)
	for _, table := range report.FragmentedTables {
		benefit, _ := a.EstimateReclaimBenefit(table)
		if benefit != nil {
			totalEstimatedTime += benefit.EstimatedTimeCost
		}
	}
	
	if totalEstimatedTime > 2*time.Hour {
		recommendations = append(recommendations, "Total optimization time exceeds 2 hours, consider splitting into multiple maintenance windows")
	}
	
	return recommendations
}