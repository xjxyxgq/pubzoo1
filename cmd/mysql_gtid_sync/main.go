package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLInstance 表示一个MySQL实例
type MySQLInstance struct {
	Address  string
	User     string
	Password string
	GTIDSet  string
	IsMaster bool
}

// GTIDSync 用于同步MySQL集群的GTID集合
type GTIDSync struct {
	Instances []MySQLInstance
}

// NewGTIDSync 创建一个新的GTIDSync实例
func NewGTIDSync(masterAddr string, slaveAddrs []string, user, password string) *GTIDSync {
	gs := &GTIDSync{}

	// 添加主节点
	gs.Instances = append(gs.Instances, MySQLInstance{
		Address:  masterAddr,
		User:     user,
		Password: password,
		IsMaster: true,
	})

	// 添加从节点
	for _, addr := range slaveAddrs {
		gs.Instances = append(gs.Instances, MySQLInstance{
			Address:  addr,
			User:     user,
			Password: password,
			IsMaster: false,
		})
	}

	return gs
}

// ConnectAndFetchGTIDs 连接到所有MySQL实例并获取它们的GTID集合
func (gs *GTIDSync) ConnectAndFetchGTIDs() error {
	for i := range gs.Instances {
		instance := &gs.Instances[i]
		dsn := fmt.Sprintf("%s:%s@tcp(%s)/mysql", instance.User, instance.Password, instance.Address)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return fmt.Errorf("连接到 %s 失败: %v", instance.Address, err)
		}
		defer db.Close()

		// 检查连接
		err = db.Ping()
		if err != nil {
			return fmt.Errorf("Ping %s 失败: %v", instance.Address, err)
		}

		// 获取GTID集合
		var gtidSet string
		row := db.QueryRow("SHOW GLOBAL VARIABLES LIKE 'gtid_executed'")
		var name string
		err = row.Scan(&name, &gtidSet)
		if err != nil {
			return fmt.Errorf("获取 %s 的GTID集合失败: %v", instance.Address, err)
		}

		instance.GTIDSet = gtidSet
		fmt.Printf("实例 %s 的GTID集合: %s\n", instance.Address, gtidSet)
	}

	return nil
}

// AnalyzeGTIDDifferences 分析GTID集合的差异并生成SQL文件
func (gs *GTIDSync) AnalyzeGTIDDifferences() error {
	if len(gs.Instances) < 2 {
		return fmt.Errorf("至少需要一个主节点和一个从节点")
	}

	// 获取主节点的GTID集合
	var masterInstance *MySQLInstance
	for i := range gs.Instances {
		if gs.Instances[i].IsMaster {
			masterInstance = &gs.Instances[i]
			break
		}
	}

	if masterInstance == nil {
		return fmt.Errorf("未找到主节点")
	}

	// 解析主节点的GTID集合
	masterGTIDs, err := parseGTIDSet(masterInstance.GTIDSet)
	if err != nil {
		return err
	}

	// 创建一个映射，用于存储每个从节点的缺失GTID
	allMissingGTIDs := make(map[string]map[string][]int)

	// 分析每个从节点的GTID差异
	for i := range gs.Instances {
		instance := &gs.Instances[i]
		if instance.IsMaster {
			continue // 跳过主节点
		}

		// 解析从节点的GTID集合
		slaveGTIDs, err := parseGTIDSet(instance.GTIDSet)
		if err != nil {
			return err
		}

		// 计算差异
		missingGTIDs := calculateMissingGTIDs(slaveGTIDs, masterGTIDs)
		if len(missingGTIDs) > 0 {
			allMissingGTIDs[instance.Address] = missingGTIDs
		}
	}

	// 生成单个SQL文件，包含所有从节点缺失的GTID
	filename := fmt.Sprintf("gtid_sync_master_%s.sql", strings.Replace(masterInstance.Address, ":", "_", -1))
	err = generateCombinedSQLFile(filename, allMissingGTIDs)
	if err != nil {
		return err
	}

	fmt.Printf("生成的SQL文件: %s (用于在主节点 %s 执行)\n", filename, masterInstance.Address)

	return nil
}

// parseGTIDSet 解析GTID集合字符串
func parseGTIDSet(gtidSet string) (map[string]map[int]bool, error) {
	result := make(map[string]map[int]bool)

	// 如果GTID集合为空，直接返回空映射
	if gtidSet == "" {
		return result, nil
	}

	// 按UUID分割GTID集合
	uuidSets := strings.Split(gtidSet, ",")
	for _, uuidSet := range uuidSets {
		parts := strings.Split(strings.TrimSpace(uuidSet), ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("无效的GTID集合格式: %s", uuidSet)
		}

		uuid := parts[0]
		
		// 初始化UUID的事务ID映射
		if _, ok := result[uuid]; !ok {
			result[uuid] = make(map[int]bool)
		}

		// MySQL的GTID格式为 UUID:interval[:interval...]
		// 例如 3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5:11:47-49
		intervals := parts[1]
		for i := 2; i < len(parts); i++ {
			intervals += ":" + parts[i]
		}

		// 处理每个区间
		ranges := strings.Split(intervals, ":")
		for _, r := range ranges {
			if strings.Contains(r, "-") {
				// 处理范围，如 1-100
				rangeParts := strings.Split(r, "-")
				if len(rangeParts) != 2 {
					return nil, fmt.Errorf("无效的GTID范围格式: %s", r)
				}

				start, err := parseInt(rangeParts[0])
				if err != nil {
					return nil, err
				}

				end, err := parseInt(rangeParts[1])
				if err != nil {
					return nil, err
				}

				for i := start; i <= end; i++ {
					result[uuid][i] = true
				}
			} else {
				// 处理单个事务ID
				txnID, err := parseInt(r)
				if err != nil {
					return nil, err
				}
				result[uuid][txnID] = true
			}
		}
	}

	return result, nil
}

// parseInt 将字符串解析为整数
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return 0, fmt.Errorf("无法解析整数 %s: %v", s, err)
	}
	return result, nil
}

// calculateMissingGTIDs 计算节点缺少的GTID
func calculateMissingGTIDs(sourceGTIDs, targetGTIDs map[string]map[int]bool) map[string][]int {
	missingGTIDs := make(map[string][]int)

	// 遍历目标节点的所有UUID
	for uuid, targetTxns := range targetGTIDs {
		// 检查源节点是否有此UUID的事务
		sourceTxns, ok := sourceGTIDs[uuid]
		if !ok {
			// 源节点完全缺少此UUID的所有事务
			missingGTIDs[uuid] = make([]int, 0, len(targetTxns))
			for txnID := range targetTxns {
				missingGTIDs[uuid] = append(missingGTIDs[uuid], txnID)
			}
			continue
		}

		// 检查源节点缺少的事务
		for txnID := range targetTxns {
			if !sourceTxns[txnID] {
				if _, ok := missingGTIDs[uuid]; !ok {
					missingGTIDs[uuid] = make([]int, 0)
				}
				missingGTIDs[uuid] = append(missingGTIDs[uuid], txnID)
			}
		}
	}

	return missingGTIDs
}

// generateCombinedSQLFile 生成包含所有从节点缺失GTID的SQL文件
func generateCombinedSQLFile(filename string, allMissingGTIDs map[string]map[string][]int) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建SQL文件失败: %v", err)
	}
	defer file.Close()

	// 写入文件头
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = file.WriteString(fmt.Sprintf("-- GTID同步SQL文件 (用于在主节点执行)\n-- 生成时间: %s\n\n", timestamp))
	if err != nil {
		return fmt.Errorf("写入文件头失败: %v", err)
	}

	// 写入SET语句
	_, err = file.WriteString("-- 禁用二进制日志，避免生成新的事务\nSET SESSION sql_log_bin = 0;\n\n")
	if err != nil {
		return fmt.Errorf("写入SET语句失败: %v", err)
	}

	// 为每个从节点的缺失GTID生成GTID_NEXT语句
	for slaveAddr, missingGTIDs := range allMissingGTIDs {
		// 添加从节点标识的注释
		_, err = file.WriteString(fmt.Sprintf("-- 以下是从节点 %s 缺失的GTIDs\n", slaveAddr))
		if err != nil {
			return fmt.Errorf("写入从节点注释失败: %v", err)
		}

		// 为每个UUID的事务ID生成GTID_NEXT语句
		for uuid, txnIDs := range missingGTIDs {
			_, err = file.WriteString(fmt.Sprintf("-- UUID: %s\n", uuid))
			if err != nil {
				return fmt.Errorf("写入UUID注释失败: %v", err)
			}

			for _, txnID := range txnIDs {
				gtidNext := fmt.Sprintf("SET GTID_NEXT = '%s:%d';\nBEGIN;\nCOMMIT;\n\n", uuid, txnID)
				_, err = file.WriteString(gtidNext)
				if err != nil {
					return fmt.Errorf("写入GTID_NEXT语句失败: %v", err)
				}
			}
		}

		_, err = file.WriteString("\n")
		if err != nil {
			return fmt.Errorf("写入分隔符失败: %v", err)
		}
	}

	// 写入恢复语句
	_, err = file.WriteString("-- 恢复二进制日志和自动GTID分配\nSET SESSION sql_log_bin = 1;\nSET GTID_NEXT = 'AUTOMATIC';\n")
	if err != nil {
		return fmt.Errorf("写入恢复语句失败: %v", err)
	}

	return nil
}

func main() {
	// 定义命令行参数
	masterAddr := flag.String("master", "", "主节点MySQL地址，格式为ip:port")
	slaveAddrsStr := flag.String("slaves", "", "从节点MySQL地址列表，格式为ip1:port1,ip2:port2,...")
	user := flag.String("user", "root", "MySQL用户名")
	password := flag.String("password", "", "MySQL密码")
	help := flag.Bool("help", false, "显示帮助信息")

	// 解析命令行参数
	flag.Parse()

	// 显示帮助信息
	if *help || *masterAddr == "" || *slaveAddrsStr == "" {
		fmt.Println("MySQL GTID同步工具")
		fmt.Println("用法: mysql_gtid_sync -master=ip:port -slaves=ip1:port1,ip2:port2,... -user=用户名 -password=密码")
		fmt.Println("参数:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// 解析从节点地址列表
	slaveAddrs := strings.Split(*slaveAddrsStr, ",")
	if len(slaveAddrs) < 1 {
		fmt.Println("错误: 至少需要提供一个从节点地址")
		os.Exit(1)
	}

	// 执行GTID同步
	fmt.Printf("开始同步GTID集合...\n")
	fmt.Printf("主节点: %s\n", *masterAddr)
	fmt.Printf("从节点: %s\n", strings.Join(slaveAddrs, ", "))

	// 创建GTIDSync实例
	gs := NewGTIDSync(*masterAddr, slaveAddrs, *user, *password)

	// 连接到所有MySQL实例并获取GTID集合
	err := gs.ConnectAndFetchGTIDs()
	if err != nil {
		fmt.Printf("同步GTID集合失败: %v\n", err)
		os.Exit(1)
	}

	// 分析GTID差异并生成SQL文件
	err = gs.AnalyzeGTIDDifferences()
	if err != nil {
		fmt.Printf("分析GTID差异失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("GTID同步完成，已生成SQL文件，请在主节点上执行该文件以注入空事务")
}