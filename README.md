# metrics-agent

Lightweight Docker container metrics agent. Provides real-time CPU, memory, and network monitoring for Docker containers via a simple HTTP API.

Perfect for PaaS platforms like [Coolify](https://coolify.io/), custom dashboards, or any application that needs container metrics.

## Features

- 列出所有 Docker 容器
- 获取容器实时资源统计（CPU、内存、网络）
- 设置容器资源限制（CPU、内存）
- Token 认证保护

## API

### 健康检查
```bash
GET /api/health
```

### 列出容器
```bash
GET /api/containers?filter=coolify
Authorization: Bearer <token>
```

### 获取容器统计
```bash
GET /api/containers/:id/stats
Authorization: Bearer <token>
```

### 获取所有容器统计
```bash
GET /api/stats?filter=coolify
Authorization: Bearer <token>
```

### 设置资源限制
```bash
POST /api/containers/:id/limits
Authorization: Bearer <token>
Content-Type: application/json

{
  "cpu_cores": 0.5,
  "memory_mb": 512
}
```

## 部署

### 环境变量
```bash
METRICS_AGENT_TOKEN=your-secret-token  # 必需
METRICS_AGENT_PORT=8080                # 可选，默认 8080
```

### Docker Compose
```bash
# 设置 token
export METRICS_AGENT_TOKEN=your-secret-token

# 启动
docker-compose up -d
```

### 在 Coolify 中部署

1. 创建新的 Docker Compose 服务
2. 粘贴 docker-compose.yml 内容
3. 设置环境变量 `METRICS_AGENT_TOKEN`
4. 配置域名 `metrics.nomoo.top`
5. 部署

## 本地开发

```bash
# 安装依赖
go mod download

# 运行
METRICS_AGENT_TOKEN=test123 go run main.go

# 测试
curl http://localhost:8080/api/health
curl -H "Authorization: Bearer test123" http://localhost:8080/api/containers
```

## 安全注意事项

- 必须设置强密码作为 TOKEN
- Docker Socket 以只读方式挂载（除了 limits 功能）
- 建议只允许特定 IP 访问

## Documentation

- [Coolify Integration Guide](docs/COOLIFY_INTEGRATION.md) - Deploy and integrate with Coolify

## License

MIT
