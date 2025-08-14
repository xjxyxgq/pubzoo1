package recycler

import (
	"context"
	"fmt"
	"time"

	"mysql_tablespace_recycling/pkg/database"
)

// TableFragmentation 表碎片信息
type TableFragmentation struct {
	Schema            string    `json:"schema"`
	TableName         string    `json:"table_name"`
	Engine            string    `json:"engine"`
	TableRows         int64     `json:"table_rows"`
	AvgRowLength      int64     `json:"avg_row_length"`
	DataLength        int64     `json:"data_length"`
	MaxDataLength     int64     `json:"max_data_length"`
	IndexLength       int64     `json:"index_length"`
	DataFree          int64     `json:"data_free"`
	AutoIncrement     *int64     `json:"auto_increment,omitempty"`
	CreateTime        *time.Time `json:"create_time,omitempty"`
	UpdateTime        *time.Time `json:"update_time,omitempty"`
	CheckTime         *time.Time `json:"check_time,omitempty"`
	TableCollation    string    `json:"table_collation"`
	Checksum          *int64    `json:"checksum,omitempty"`
	CreateOptions     string    `json:"create_options"`
	TableComment      string    `json:"table_comment"`
	// 计算字段
	TotalSize         int64     `json:"total_size"`         // data_length + index_length
	FragmentRatio     float64   `json:"fragment_ratio"`     // data_free / total_size
	FragmentSize      int64     `json:"fragment_size"`      // data_free
	WastedRatio       float64   `json:"wasted_ratio"`       // fragment_ratio
	EstimatedRows     int64     `json:"estimated_rows"`     // 估算的行数
	EfficiencyRatio   float64   `json:"efficiency_ratio"`   // 数据利用效率
}

// FragmentationAnalysisOptions 碎片分析选项
type FragmentationAnalysisOptions struct {
	MinFragmentSize   int64    `json:"min_fragment_size"`   // 最小碎片大小阈值
	MinFragmentRatio  float64  `json:"min_fragment_ratio"`  // 最小碎片比例阈值
	MinTableSize      int64    `json:"min_table_size"`      // 最小表大小
	MaxTableSize      int64    `json:"max_table_size"`      // 最大表大小  
	IncludeSchemas    []string `json:"include_schemas"`     // 包含的数据库
	ExcludeSchemas    []string `json:"exclude_schemas"`     // 排除的数据库
	IncludeTables     []string `json:"include_tables"`      // 包含的表（支持通配符）
	ExcludeTables     []string `json:"exclude_tables"`      // 排除的表（支持通配符）
	SupportedEngines  []string `json:"supported_engines"`   // 支持的存储引擎
	SortBy            string   `json:"sort_by"`             // 排序字段
	SortOrder         string   `json:"sort_order"`          // 排序顺序
	Limit             int      `json:"limit"`               // 限制返回数量
}

// FragmentationReport 碎片分析报告
type FragmentationReport struct {
	NodeHost              string                  `json:"node_host"`
	NodePort              int                     `json:"node_port"`
	AnalysisTime          time.Time              `json:"analysis_time"`
	TotalTables           int                     `json:"total_tables"`
	FragmentedTables      []*TableFragmentation   `json:"fragmented_tables"`
	TotalFragmentSize     int64                  `json:"total_fragment_size"`
	TotalDataSize         int64                  `json:"total_data_size"`
	TotalIndexSize        int64                  `json:"total_index_size"`
	EstimatedReclaimableSpace int64              `json:"estimated_reclaimable_space"`
	AverageFragmentRatio  float64                `json:"average_fragment_ratio"`
	LargestFragmentTable  *TableFragmentation     `json:"largest_fragment_table,omitempty"`
	HighestRatioTable     *TableFragmentation     `json:"highest_ratio_table,omitempty"`
	RecommendedActions    []string               `json:"recommended_actions"`
	AnalysisOptions       *FragmentationAnalysisOptions `json:"analysis_options"`
}

// FragmentationAnalyzer 表空间碎片分析器接口
type FragmentationAnalyzer interface {
	AnalyzeFragmentation(ctx context.Context, nodeConfig database.NodeConfigInterface, options *FragmentationAnalysisOptions) (*FragmentationReport, error)
	GetTableFragmentation(ctx context.Context, nodeConfig database.NodeConfigInterface, schema, tableName string) (*TableFragmentation, error)
	GetSchemaFragmentation(ctx context.Context, nodeConfig database.NodeConfigInterface, schema string, options *FragmentationAnalysisOptions) ([]*TableFragmentation, error)
	EstimateReclaimBenefit(fragmentation *TableFragmentation) (*ReclaimBenefit, error)
}

// ReclaimBenefit 回收收益评估
type ReclaimBenefit struct {
	EstimatedReclaimedSpace int64         `json:"estimated_reclaimed_space"`
	EstimatedTimeCost       time.Duration `json:"estimated_time_cost"`
	Priority               int           `json:"priority"`
	RiskLevel              RiskLevel     `json:"risk_level"`
	Benefits               []string      `json:"benefits"`
	Risks                  []string      `json:"risks"`
	Recommendation         string        `json:"recommendation"`
}

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// CalculateFragmentationMetrics 计算碎片化指标
func (tf *TableFragmentation) CalculateFragmentationMetrics() {
	tf.TotalSize = tf.DataLength + tf.IndexLength
	
	if tf.TotalSize > 0 {
		tf.FragmentRatio = float64(tf.DataFree) / float64(tf.TotalSize)
		tf.EfficiencyRatio = float64(tf.TotalSize-tf.DataFree) / float64(tf.TotalSize)
	}
	
	tf.FragmentSize = tf.DataFree
	tf.WastedRatio = tf.FragmentRatio
	
	// 估算行数
	if tf.AvgRowLength > 0 {
		tf.EstimatedRows = tf.DataLength / tf.AvgRowLength
	}
}

// IsFragmented 检查表是否需要碎片整理
func (tf *TableFragmentation) IsFragmented(options *FragmentationAnalysisOptions) bool {
	// 检查最小碎片大小
	if tf.DataFree < options.MinFragmentSize {
		return false
	}
	
	// 检查最小碎片比例
	if tf.FragmentRatio < options.MinFragmentRatio {
		return false
	}
	
	// 检查表大小范围
	if tf.TotalSize < options.MinTableSize || 
	   (options.MaxTableSize > 0 && tf.TotalSize > options.MaxTableSize) {
		return false
	}
	
	// 检查存储引擎
	if len(options.SupportedEngines) > 0 {
		supported := false
		for _, engine := range options.SupportedEngines {
			if tf.Engine == engine {
				supported = true
				break
			}
		}
		if !supported {
			return false
		}
	}
	
	return true
}

// GetFragmentationSeverity 获取碎片化严重程度
func (tf *TableFragmentation) GetFragmentationSeverity() string {
	if tf.FragmentRatio >= 0.3 {
		return "severe"
	} else if tf.FragmentRatio >= 0.15 {
		return "moderate"
	} else if tf.FragmentRatio >= 0.05 {
		return "mild"
	}
	return "minimal"
}

// String 返回表碎片信息的字符串表示
func (tf *TableFragmentation) String() string {
	return fmt.Sprintf("%s.%s: %s engine, %.2f%% fragmented (%s), %s total, %s free",
		tf.Schema, tf.TableName, tf.Engine, tf.FragmentRatio*100,
		tf.GetFragmentationSeverity(), 
		formatBytes(tf.TotalSize), formatBytes(tf.DataFree))
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	}
}

// GetDefaultAnalysisOptions 获取默认分析选项
func GetDefaultAnalysisOptions() *FragmentationAnalysisOptions {
	return &FragmentationAnalysisOptions{
		MinFragmentSize:  100 * 1024 * 1024, // 100MB
		MinFragmentRatio: 0.05,              // 5%
		MinTableSize:     10 * 1024 * 1024,  // 10MB
		MaxTableSize:     0,                 // 无限制
		SupportedEngines: []string{"InnoDB", "MyISAM"},
		SortBy:          "fragment_size",
		SortOrder:       "desc",
		Limit:           0, // 无限制
	}
}