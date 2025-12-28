package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	version      = "1.0.0"
)

// 数据结构
type ContainerInfo struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
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
	CPU           CPUStats     `json:"cpu"`
	Memory        MemoryStats  `json:"memory"`
	Network       NetworkStats `json:"network"`
}

type ResourceLimits struct {
	CPUCores float64 `json:"cpu_cores"`
	MemoryMB int64   `json:"memory_mb"`
}

func main() {
	startTime = time.Now()

	// 获取配置
	authToken = os.Getenv("METRICS_AGENT_TOKEN")
	if authToken == "" {
		log.Fatal("METRICS_AGENT_TOKEN is required")
	}

	port := os.Getenv("METRICS_AGENT_PORT")
	if port == "" {
		port = "8080"
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
		api.POST("/containers/:id/limits", setLimitsHandler)
		api.GET("/stats", allStatsHandler)
	}

	log.Printf("Starting metrics-agent v%s on port %s", version, port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
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
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{All: true})
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

		result = append(result, ContainerInfo{
			ID:      ctr.ID[:12],
			Name:    name,
			Status:  ctr.State,
			Created: time.Unix(ctr.Created, 0),
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

// 获取所有容器统计
func allStatsHandler(c *gin.Context) {
	filter := c.Query("filter")

	ctx := context.Background()
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{})
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
		// CPU 限制：NanoCPUs = cores * 1e9
		updateConfig.Resources.NanoCPUs = int64(limits.CPUCores * 1e9)
	}

	if limits.MemoryMB > 0 {
		// 内存限制：bytes
		updateConfig.Resources.Memory = limits.MemoryMB * 1024 * 1024
		updateConfig.Resources.MemorySwap = limits.MemoryMB * 1024 * 1024 // 禁用 swap
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

// 获取容器统计数据
func getContainerStats(containerID string) (*ContainerStats, error) {
	ctx := context.Background()

	// 获取容器信息
	inspect, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// 获取统计数据（非流式）
	statsResp, err := dockerClient.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	body, err := io.ReadAll(statsResp.Body)
	if err != nil {
		return nil, err
	}

	var dockerStats types.StatsJSON
	if err := json.Unmarshal(body, &dockerStats); err != nil {
		return nil, err
	}

	// 计算 CPU 使用率
	cpuPercent := calculateCPUPercent(&dockerStats)

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

	return &ContainerStats{
		ContainerID:   containerID[:12],
		ContainerName: containerName,
		Timestamp:     time.Now(),
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

// 计算 CPU 使用率
func calculateCPUPercent(stats *types.StatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent := (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
		// 保留两位小数
		return float64(int(cpuPercent*100)) / 100
	}
	return 0
}
