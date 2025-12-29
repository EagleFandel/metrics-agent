package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
)

var (
	dockerClient *client.Client
	authToken    string
	startTime    time.Time
	version      = "1.2.3"

	// 历史数据存储
	historyMutex   sync.RWMutex
	networkHistory = make(map[string][]NetworkHistoryPoint) // containerID -> history
	cpuHistory     = make(map[string][]MetricHistoryPoint)
	memoryHistory  = make(map[string][]MetricHistoryPoint)

	// 最新 CPU 数据缓存（用于快速响应）
	cpuCacheMutex sync.RWMutex
	cpuCache      = make(map[string]float64) // containerID -> cpu percent

	// Traefik 日志路径
	traefikLogPath = "/var/log/traefik/access.log"
)

// 数据结构
type ContainerInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Created   time.Time `json:"created"`
	StartedAt time.Time `json:"started_at"`
}

type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        float64 `json:"cores"`
	LimitCores   float64 `json:"limit_cores"`
}

type MemoryStats struct {
	UsageBytes   uint64  `json:"usage_bytes"`
	UsageMB      float64 `json:"usage_mb"`
	LimitBytes   uint64  `json:"limit_bytes"`
	LimitMB      float64 `json:"limit_mb"`
	UsagePercent float64 `json:"usage_percent"`
}

type NetworkStats struct {
	RxBytes uint64  `json:"rx_bytes"`
	RxMB    float64 `json:"rx_mb"`
	TxBytes uint64  `json:"tx_bytes"`
	TxMB    float64 `json:"tx_mb"`
}

type ContainerStats struct {
	ContainerID   string       `json:"container_id"`
	ContainerName string       `json:"container_name"`
	Timestamp     time.Time    `json:"timestamp"`
	StartedAt     time.Time    `json:"started_at"`
	CPU           CPUStats     `json:"cpu"`
	Memory        MemoryStats  `json:"memory"`
	Network       NetworkStats `json:"network"`
}

type ResourceLimits struct {
	CPUCores float64 `json:"cpu_cores"`
	MemoryMB int64   `json:"memory_mb"`
}

// 历史数据点
type NetworkHistoryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	RxMB      float64   `json:"rx_mb"`
	TxMB      float64   `json:"tx_mb"`
}

type MetricHistoryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// 请求统计
type RequestStats struct {
	Domain string `json:"domain"`
	Today  int64  `json:"today"`
	Total  int64  `json:"total"`
}

func main() {
	startTime = time.Now()

	// 获取配置
	authToken = os.Getenv("METRICS_AGENT_TOKEN")
	if authToken == "" {
		log.Fatal("METRICS_AGENT_TOKEN is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("METRICS_AGENT_PORT")
	}
	if port == "" {
		port = "3000"
	}

	// Traefik 日志路径可配置
	if logPath := os.Getenv("TRAEFIK_LOG_PATH"); logPath != "" {
		traefikLogPath = logPath
	}

	// 初始化 Docker 客户端
	var err error
	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	// 测试 Docker 连接
	ctx := context.Background()
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}
	log.Println("Connected to Docker")

	// 启动历史数据采集
	go collectHistoryData()

	// 设置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 路由
	r.GET("/api/health", healthHandler)

	// 需要认证的路由
	api := r.Group("/api")
	api.Use(authMiddleware())
	{
		api.GET("/containers", listContainersHandler)
		api.GET("/containers/:id/stats", containerStatsHandler)
		api.GET("/containers/:id/history", containerHistoryHandler)
		api.GET("/containers/:id/all", containerAllHandler) // 新增：一次获取所有数据
		api.POST("/containers/:id/limits", setLimitsHandler)
		api.GET("/stats", allStatsHandler)
		api.GET("/requests", requestStatsHandler)
	}

	log.Printf("Starting metrics-agent v%s on port %s", version, port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// 定时采集历史数据
func collectHistoryData() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// 立即采集一次
	collectOnce()

	for range ticker.C {
		collectOnce()
	}
}

func collectOnce() {
	ctx := context.Background()
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		log.Printf("Failed to list containers for history: %v", err)
		return
	}

	now := time.Now()
	maxPoints := 288 // 24小时 * 12 (每5分钟一个点)

	for _, ctr := range containers {
		// 使用流式模式获取准确的 CPU 数据
		statsResp, err := dockerClient.ContainerStats(ctx, ctr.ID, true)
		if err != nil {
			continue
		}

		decoder := json.NewDecoder(statsResp.Body)
		var dockerStats types.StatsJSON
		if err := decoder.Decode(&dockerStats); err != nil {
			statsResp.Body.Close()
			continue
		}
		statsResp.Body.Close()

		cpuPercent := calculateCPUPercent(&dockerStats)
		memUsage := float64(dockerStats.MemoryStats.Usage) / 1024 / 1024

		var rxBytes, txBytes uint64
		for _, netStats := range dockerStats.Networks {
			rxBytes += netStats.RxBytes
			txBytes += netStats.TxBytes
		}

		id := ctr.ID[:12]

		// 更新 CPU 缓存
		cpuCacheMutex.Lock()
		cpuCache[id] = cpuPercent
		cpuCacheMutex.Unlock()

		historyMutex.Lock()

		// 网络历史
		networkHistory[id] = append(networkHistory[id], NetworkHistoryPoint{
			Timestamp: now,
			RxMB:      float64(rxBytes) / 1024 / 1024,
			TxMB:      float64(txBytes) / 1024 / 1024,
		})
		if len(networkHistory[id]) > maxPoints {
			networkHistory[id] = networkHistory[id][len(networkHistory[id])-maxPoints:]
		}

		// CPU 历史
		cpuHistory[id] = append(cpuHistory[id], MetricHistoryPoint{
			Timestamp: now,
			Value:     cpuPercent,
		})
		if len(cpuHistory[id]) > maxPoints {
			cpuHistory[id] = cpuHistory[id][len(cpuHistory[id])-maxPoints:]
		}

		// 内存历史
		memoryHistory[id] = append(memoryHistory[id], MetricHistoryPoint{
			Timestamp: now,
			Value:     memUsage,
		})
		if len(memoryHistory[id]) > maxPoints {
			memoryHistory[id] = memoryHistory[id][len(memoryHistory[id])-maxPoints:]
		}

		historyMutex.Unlock()
	}
}

// 认证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token != authToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// 健康检查
func healthHandler(c *gin.Context) {
	uptime := time.Since(startTime).Round(time.Second).String()
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": version,
		"uptime":  uptime,
	})
}

// 列出容器
func listContainersHandler(c *gin.Context) {
	filter := c.Query("filter")

	ctx := context.Background()
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []ContainerInfo
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}

		// 过滤
		if filter != "" && !strings.Contains(name, filter) {
			continue
		}

		// 获取容器详情以获取 StartedAt
		inspect, err := dockerClient.ContainerInspect(ctx, ctr.ID)
		startedAt := time.Time{}
		if err == nil && inspect.State != nil {
			startedAt, _ = time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		}

		result = append(result, ContainerInfo{
			ID:        ctr.ID[:12],
			Name:      name,
			Status:    ctr.State,
			Created:   time.Unix(ctr.Created, 0),
			StartedAt: startedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"containers": result})
}

// 获取单个容器统计
func containerStatsHandler(c *gin.Context) {
	containerID := c.Param("id")

	stats, err := getContainerStats(containerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// 获取容器历史数据
func containerHistoryHandler(c *gin.Context) {
	containerID := c.Param("id")

	historyMutex.RLock()
	defer historyMutex.RUnlock()

	// 如果 ID 是短格式，尝试匹配
	var matchedID string
	for id := range networkHistory {
		if strings.HasPrefix(id, containerID) || strings.HasPrefix(containerID, id) {
			matchedID = id
			break
		}
	}

	if matchedID == "" {
		matchedID = containerID
	}

	c.JSON(http.StatusOK, gin.H{
		"container_id": matchedID,
		"network":      networkHistory[matchedID],
		"cpu":          cpuHistory[matchedID],
		"memory":       memoryHistory[matchedID],
	})
}

// 获取所有容器统计
func allStatsHandler(c *gin.Context) {
	filter := c.Query("filter")

	ctx := context.Background()
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []ContainerStats
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}

		// 过滤
		if filter != "" && !strings.Contains(name, filter) {
			continue
		}

		stats, err := getContainerStats(ctr.ID)
		if err != nil {
			continue
		}
		result = append(result, *stats)
	}

	c.JSON(http.StatusOK, gin.H{
		"timestamp":  time.Now(),
		"containers": result,
	})
}

// 设置资源限制
func setLimitsHandler(c *gin.Context) {
	containerID := c.Param("id")

	var limits ResourceLimits
	if err := c.ShouldBindJSON(&limits); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// 构建更新配置
	updateConfig := container.UpdateConfig{}

	if limits.CPUCores > 0 {
		updateConfig.Resources.NanoCPUs = int64(limits.CPUCores * 1e9)
	}

	if limits.MemoryMB > 0 {
		updateConfig.Resources.Memory = limits.MemoryMB * 1024 * 1024
		updateConfig.Resources.MemorySwap = limits.MemoryMB * 1024 * 1024
	}

	_, err := dockerClient.ContainerUpdate(ctx, containerID, updateConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"container_id": containerID,
		"limits":       limits,
	})
}

// 请求统计 - 解析 Traefik 日志
func requestStatsHandler(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain parameter required"})
		return
	}

	stats, err := countRequestsFromTraefikLog(domain)
	if err != nil {
		// 如果日志不存在，返回 0
		c.JSON(http.StatusOK, RequestStats{
			Domain: domain,
			Today:  0,
			Total:  0,
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Traefik JSON 日志结构
type TraefikLogEntry struct {
	RequestHost string `json:"RequestHost"`
	StartUTC    string `json:"StartUTC"`
	Time        string `json:"time"`
}

// 解析 Traefik 访问日志统计请求数
func countRequestsFromTraefikLog(domain string) (*RequestStats, error) {
	file, err := os.Open(traefikLogPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var total, today int64
	todayStr := time.Now().Format("2006-01-02")

	scanner := bufio.NewScanner(file)
	// 增加 buffer 大小以处理长行
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// 尝试解析 JSON 格式
		var entry TraefikLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			// JSON 格式日志
			if entry.RequestHost == domain {
				total++
				// 检查是否是今天 (StartUTC 格式: 2025-12-29T14:00:31.427569156Z)
				if strings.HasPrefix(entry.StartUTC, todayStr) || strings.HasPrefix(entry.Time, todayStr) {
					today++
				}
			}
		} else {
			// 回退到文本格式匹配
			if strings.Contains(line, domain) {
				total++
				if strings.Contains(line, todayStr) {
					today++
				}
			}
		}
	}

	return &RequestStats{
		Domain: domain,
		Today:  today,
		Total:  total,
	}, nil
}

// 获取容器统计数据（快速版本，使用缓存的 CPU 数据）
func getContainerStats(containerID string) (*ContainerStats, error) {
	ctx := context.Background()

	// 获取容器信息
	inspect, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// 使用非流式模式快速获取内存和网络数据
	statsResp, err := dockerClient.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	decoder := json.NewDecoder(statsResp.Body)
	var dockerStats types.StatsJSON
	if err := decoder.Decode(&dockerStats); err != nil {
		return nil, err
	}

	// 从缓存获取 CPU 数据，如果没有则使用当前计算值
	shortID := containerID
	if len(containerID) > 12 {
		shortID = containerID[:12]
	}

	cpuCacheMutex.RLock()
	cpuPercent, hasCached := cpuCache[shortID]
	cpuCacheMutex.RUnlock()

	if !hasCached {
		// 没有缓存，计算当前值（可能不准确）
		cpuPercent = calculateCPUPercent(&dockerStats)
	}

	// CPU 限制
	cpuLimit := float64(dockerStats.CPUStats.OnlineCPUs)
	if inspect.HostConfig.NanoCPUs > 0 {
		cpuLimit = float64(inspect.HostConfig.NanoCPUs) / 1e9
	}

	// 内存统计
	memUsage := dockerStats.MemoryStats.Usage
	memLimit := dockerStats.MemoryStats.Limit
	if inspect.HostConfig.Memory > 0 {
		memLimit = uint64(inspect.HostConfig.Memory)
	}
	memPercent := float64(0)
	if memLimit > 0 {
		memPercent = float64(memUsage) / float64(memLimit) * 100
	}

	// 网络统计
	var rxBytes, txBytes uint64
	for _, netStats := range dockerStats.Networks {
		rxBytes += netStats.RxBytes
		txBytes += netStats.TxBytes
	}

	containerName := strings.TrimPrefix(inspect.Name, "/")

	// 解析启动时间
	startedAt := time.Time{}
	if inspect.State != nil {
		startedAt, _ = time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
	}

	return &ContainerStats{
		ContainerID:   shortID,
		ContainerName: containerName,
		Timestamp:     time.Now(),
		StartedAt:     startedAt,
		CPU: CPUStats{
			UsagePercent: cpuPercent,
			Cores:        cpuPercent / 100 * cpuLimit,
			LimitCores:   cpuLimit,
		},
		Memory: MemoryStats{
			UsageBytes:   memUsage,
			UsageMB:      float64(memUsage) / 1024 / 1024,
			LimitBytes:   memLimit,
			LimitMB:      float64(memLimit) / 1024 / 1024,
			UsagePercent: memPercent,
		},
		Network: NetworkStats{
			RxBytes: rxBytes,
			RxMB:    float64(rxBytes) / 1024 / 1024,
			TxBytes: txBytes,
			TxMB:    float64(txBytes) / 1024 / 1024,
		},
	}, nil
}

// 组合响应结构
type ContainerAllResponse struct {
	ContainerID string              `json:"container_id"`
	Stats       *ContainerStats     `json:"stats"`
	History     *ContainerHistoryData `json:"history"`
	Requests    *RequestStats       `json:"requests,omitempty"`
}

type ContainerHistoryData struct {
	Network []NetworkHistoryPoint `json:"network"`
	CPU     []MetricHistoryPoint  `json:"cpu"`
	Memory  []MetricHistoryPoint  `json:"memory"`
}

// 获取容器所有数据（stats + history + requests）
func containerAllHandler(c *gin.Context) {
	containerID := c.Param("id")
	domain := c.Query("domain") // 可选，用于获取请求统计

	// 获取统计数据
	stats, err := getContainerStats(containerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取历史数据
	historyMutex.RLock()
	var matchedID string
	for id := range networkHistory {
		if strings.HasPrefix(id, containerID) || strings.HasPrefix(containerID, id) {
			matchedID = id
			break
		}
	}
	if matchedID == "" {
		matchedID = containerID
	}

	history := &ContainerHistoryData{
		Network: networkHistory[matchedID],
		CPU:     cpuHistory[matchedID],
		Memory:  memoryHistory[matchedID],
	}
	historyMutex.RUnlock()

	// 获取请求统计（如果提供了域名）
	var requestStats *RequestStats
	if domain != "" {
		rs, err := countRequestsFromTraefikLog(domain)
		if err == nil {
			requestStats = rs
		}
	}

	c.JSON(http.StatusOK, ContainerAllResponse{
		ContainerID: stats.ContainerID,
		Stats:       stats,
		History:     history,
		Requests:    requestStats,
	})
}

// 计算 CPU 使用率
func calculateCPUPercent(stats *types.StatsJSON) float64 {
	// 检查是否有有效的 PreCPUStats
	if stats.PreCPUStats.CPUUsage.TotalUsage == 0 || stats.PreCPUStats.SystemUsage == 0 {
		// 非流式模式下 PreCPUStats 可能为空，使用备用计算方法
		// 基于容器运行时间和总 CPU 时间估算
		if stats.CPUStats.OnlineCPUs > 0 && stats.CPUStats.CPUUsage.TotalUsage > 0 {
			// 简单估算：假设容器使用了一定比例的 CPU
			// 这不是精确值，但比 0 更有意义
			return 0.1 // 返回一个小的基准值表示容器在运行
		}
		return 0
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent := (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
		return float64(int(cpuPercent*100)) / 100
	}
	return 0
}
