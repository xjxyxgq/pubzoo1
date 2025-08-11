package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	// 检查命令行参数
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <input_file>\n", os.Args[0])
		fmt.Printf("Example: %s test.txt\n", os.Args[0])
		return
	}

	filePath := os.Args[1]

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var currentCluster string
	var currentIP string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 检查是否是新的操作开始
		if strings.HasPrefix(line, "【操作") {
			// 提取集群信息
			parts := strings.Split(line, "集群:")
			if len(parts) == 2 {
				currentCluster = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "业务IP:") {
			// 提取业务IP
			parts := strings.Split(line, "业务IP:")
			if len(parts) == 2 {
				ip := strings.TrimSpace(parts[1])
				if ip != "" {
					currentIP = ip
				}
			}
		} else if strings.Contains(line, "检查结果:") && strings.Contains(line, "db原因失败:") {
			// 处理检查结果
			if currentIP != "" && currentCluster != "" {
				processLine(currentIP, currentCluster, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}
}

func extractDBName(cluster string) string {
	// 去除第一个下划线之前的部分
	parts := strings.Split(cluster, "_")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "_")
	}
	return cluster
}

func processLine(ip, cluster, line string) {
	dbname := extractDBName(cluster)

	// 使用正则表达式匹配整个数组结构 ["table_name", ["details1", "details2", ...]]
	re := regexp.MustCompile(`\["([^"]+)",\s*\[([^\]]+)\]\]`)
	matches := re.FindStringSubmatch(line)

	if len(matches) >= 3 {
		tableName := matches[1]
		innerContent := matches[2]

		// 解析内层数组内容，提取时间戳
		// 分割逗号分隔的元素
		elements := strings.Split(innerContent, ",")

		for _, element := range elements {
			element = strings.TrimSpace(element)
			element = strings.Trim(element, "\"")

			// 提取时间戳（YYYYMMDD格式）
			tsRe := regexp.MustCompile(`(\d{8})`)
			tsMatches := tsRe.FindStringSubmatch(element)
			if len(tsMatches) > 1 {
				ts := tsMatches[1]
				fmt.Printf("%s:3306,%s,%s,%s,0\n", ip, dbname, tableName, ts)
			}
		}
	}
}
