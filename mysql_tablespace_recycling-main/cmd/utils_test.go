package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSizeStr(t *testing.T) {
	t.Run("Parse empty string", func(t *testing.T) {
		size, err := parseSizeStr("")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), size)
	})

	t.Run("Parse bytes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"1024", 1024},
			{"1024B", 1024},
			{"2048b", 2048}, // 测试小写
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse kilobytes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"1KB", 1024},
			{"2kb", 2048}, // 测试小写
			{"1.5KB", 1536},
			{"10KB", 10 * 1024},
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse megabytes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"1MB", 1024 * 1024},
			{"2mb", 2 * 1024 * 1024}, // 测试小写
			{"1.5MB", int64(1.5 * 1024 * 1024)},
			{"100MB", 100 * 1024 * 1024},
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse gigabytes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"1GB", 1024 * 1024 * 1024},
			{"2gb", 2 * 1024 * 1024 * 1024}, // 测试小写
			{"1.5GB", int64(1.5 * 1024 * 1024 * 1024)},
			{"10GB", 10 * 1024 * 1024 * 1024},
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse terabytes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"1TB", 1024 * 1024 * 1024 * 1024},
			{"2tb", 2 * 1024 * 1024 * 1024 * 1024}, // 测试小写
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse invalid formats", func(t *testing.T) {
		invalidCases := []string{
			"invalid",
			"1.2.3MB",
			"abcMB",
			"-100MB",
			"MB",
			"1XB", // 无效单位
		}

		for _, invalid := range invalidCases {
			_, err := parseSizeStr(invalid)
			assert.Error(t, err, "Expected error for invalid input: %s", invalid)
			assert.Contains(t, err.Error(), "invalid size format", "Error message should contain 'invalid size format'")
		}
	})

	t.Run("Parse with whitespace", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{" 1MB ", 1024 * 1024},
			{"\t2GB\n", 2 * 1024 * 1024 * 1024},
			{"  100KB  ", 100 * 1024},
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse fractional values", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"0.5KB", 512},
			{"2.5MB", int64(2.5 * 1024 * 1024)},
			{"1.25GB", int64(1.25 * 1024 * 1024 * 1024)},
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})

	t.Run("Parse edge cases", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64
		}{
			{"0", 0},
			{"0MB", 0},
			{"0.0GB", 0},
		}

		for _, tc := range testCases {
			size, err := parseSizeStr(tc.input)
			assert.NoError(t, err, "Failed to parse %s", tc.input)
			assert.Equal(t, tc.expected, size, "Unexpected result for %s", tc.input)
		}
	})
}