package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HTTPExternalAPIClient HTTP外部API客户端
type HTTPExternalAPIClient struct {
	baseURL    string
	httpClient *http.Client
	cache      map[string]*cachedNodeInfo
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
}

// cachedNodeInfo 缓存的节点信息
type cachedNodeInfo struct {
	info      *ExternalNodeInfo
	timestamp time.Time
}

// NewHTTPExternalAPIClient 创建HTTP外部API客户端
func NewHTTPExternalAPIClient(baseURL string, timeout time.Duration, cacheTTL time.Duration) *HTTPExternalAPIClient {
	return &HTTPExternalAPIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cache:    make(map[string]*cachedNodeInfo),
		cacheTTL: cacheTTL,
	}
}

// GetNodeInfo 获取节点信息
func (c *HTTPExternalAPIClient) GetNodeInfo(ctx context.Context, host string) (*ExternalNodeInfo, error) {
	// 检查缓存
	if info := c.getCachedNodeInfo(host); info != nil {
		return info, nil
	}

	// 准备请求数据
	requestData := map[string]string{
		"search_ins_ip": host,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	// 解析响应
	var apiResp ExternalAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Status != "ok" {
		return nil, fmt.Errorf("API returned error status: %s", apiResp.Status)
	}

	if apiResp.Data == nil {
		return nil, fmt.Errorf("API returned no data")
	}

	// 缓存结果
	c.cacheNodeInfo(host, apiResp.Data)

	return apiResp.Data, nil
}

// getCachedNodeInfo 获取缓存的节点信息
func (c *HTTPExternalAPIClient) getCachedNodeInfo(host string) *ExternalNodeInfo {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	cached, exists := c.cache[host]
	if !exists {
		return nil
	}

	// 检查缓存是否过期
	if time.Since(cached.timestamp) > c.cacheTTL {
		return nil
	}

	return cached.info
}

// cacheNodeInfo 缓存节点信息
func (c *HTTPExternalAPIClient) cacheNodeInfo(host string, info *ExternalNodeInfo) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache[host] = &cachedNodeInfo{
		info:      info,
		timestamp: time.Now(),
	}
}

// ClearCache 清理缓存
func (c *HTTPExternalAPIClient) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache = make(map[string]*cachedNodeInfo)
}

// CleanupExpiredCache 清理过期缓存
func (c *HTTPExternalAPIClient) CleanupExpiredCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	now := time.Now()
	for host, cached := range c.cache {
		if now.Sub(cached.timestamp) > c.cacheTTL {
			delete(c.cache, host)
		}
	}
}

// StartCacheCleanup 启动缓存清理goroutine
func (c *HTTPExternalAPIClient) StartCacheCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				c.CleanupExpiredCache()
			case <-ctx.Done():
				return
			}
		}
	}()
}