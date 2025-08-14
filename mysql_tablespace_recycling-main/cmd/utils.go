package cmd

import (
	"fmt"
	"strings"
)

// parseSizeStr 解析大小字符串为字节数（避免和parseSize冲突）
func parseSizeStr(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}
	
	original := sizeStr
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))
	
	var multiplier int64 = 1
	var numStr string
	
	// Check longer suffixes first to avoid conflicts
	if strings.HasSuffix(sizeStr, "TB") {
		numStr = strings.TrimSuffix(sizeStr, "TB")
		multiplier = 1024 * 1024 * 1024 * 1024
	} else if strings.HasSuffix(sizeStr, "GB") {
		numStr = strings.TrimSuffix(sizeStr, "GB")
		multiplier = 1024 * 1024 * 1024
	} else if strings.HasSuffix(sizeStr, "MB") {
		numStr = strings.TrimSuffix(sizeStr, "MB")
		multiplier = 1024 * 1024
	} else if strings.HasSuffix(sizeStr, "KB") {
		numStr = strings.TrimSuffix(sizeStr, "KB")
		multiplier = 1024
	} else if strings.HasSuffix(sizeStr, "B") {
		numStr = strings.TrimSuffix(sizeStr, "B")
		multiplier = 1
	} else {
		// No suffix, check if it contains letters (except for pure numbers)
		numStr = sizeStr
		for _, r := range sizeStr {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				return 0, fmt.Errorf("invalid size format: %s", original)
			}
		}
	}
	
	// Validate numStr doesn't contain invalid characters
	if numStr == "" {
		return 0, fmt.Errorf("invalid size format: %s", original)
	}
	
	// Check for multiple dots
	if strings.Count(numStr, ".") > 1 {
		return 0, fmt.Errorf("invalid size format: %s", original)
	}
	
	var size float64
	n, err := fmt.Sscanf(numStr, "%f", &size)
	if err != nil || n != 1 || size < 0 {
		return 0, fmt.Errorf("invalid size format: %s", original)
	}
	
	// Verify that Sscanf consumed the entire string
	formatted := fmt.Sprintf("%g", size)
	if formatted != numStr {
		// Try other formats
		alt1 := fmt.Sprintf("%.1f", size)
		alt2 := fmt.Sprintf("%.2f", size)
		if alt1 != numStr && alt2 != numStr {
			return 0, fmt.Errorf("invalid size format: %s", original)
		}
	}
	
	return int64(size * float64(multiplier)), nil
}