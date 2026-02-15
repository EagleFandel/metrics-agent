# metrics-agent

轻量级 Docker 容器监控代理，提供实时 CPU、内存、网络监控和请求统计。

## 线上地址

https://metrics.nomoo.top

## 功能特性

- 列出所有 Docker 容器
- 实时资源统计（CPU、内存、网络）
- 历史数据记录（24小时，每5分钟采样）
- 请求统计（解析 Traefik 访问日志）
- 设置容器资源限制
- Token 认证保护
- 一次性获取所有数据的聚合接口

## API

### 健康检查
```bash
GET /api/health
# 返回 {"status":"ok","version":"1.2.3","uptime":"..."}
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

### 获取容器历史数据
```bash
GET /api/containers/:id/history
Authorization: Bearer <token>
# 返回 CPU、内存、网络的历史数据点
```

### 获取容器所有数据（推荐）
```bash
GET /api/containers/:id/all?domain=xxx.nomoo.top
Authorization: Bearer <token>
# 一次返回 stats + history + requests
```

### 获取所有容器统计
```bash
GET /api/stats?filter=coolify
Authorization: Bearer <token>
```

### 获取请求统计
```bash
GET /api/requests?domain=xxx.nomoo.top
Authorization: Bearer <token>
# 返回 {"domain":"...","today":123,"total":456}
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

| 变量 | 说明 | 必填 |
|------|------|------|
| METRICS_AGENT_TOKEN | API 认证 Token | 是 |
| PORT / METRICS_AGENT_PORT | 服务端口 | 否（默认 3000） |
| TRAEFIK_LOG_PATH | Traefik 日志路径 | 否（默认 /var/log/traefik/access.log） |

### Docker Compose

```bash
docker-compose up -d
```

### 在 Coolify 中部署

1. 创建新的 Docker Compose 服务
2. 粘贴 docker-compose.yml 内容
3. 设置环境变量 `METRICS_AGENT_TOKEN`
4. 挂载 Docker Socket：`/var/run/docker.sock:/var/run/docker.sock`
5. 挂载 Traefik 日志：`/var/log/traefik:/var/log/traefik:ro`
6. 配置域名 `metrics.nomoo.top`
7. 部署

## 本地开发

```bash
# 安装依赖
go mod download

# 运行
METRICS_AGENT_TOKEN=test123 go run main.go

# 测试
curl http://localhost:3000/api/health
curl -H "Authorization: Bearer test123" http://localhost:3000/api/containers
```

## 请求统计说明

请求统计通过解析 Traefik JSON 格式的访问日志实现：
- 只统计 GET 请求
- 只统计 2xx 成功响应
- 排除静态资源（/_next/、/api/、/favicon 等）
- 排除扫描请求（/wp-admin、/.env 等）

## 安全注意事项

- 必须设置强密码作为 TOKEN
- Docker Socket 需要挂载才能获取容器信息
- 建议只允许特定 IP 访问

## Documentation

- [Coolify Integration Guide](docs/COOLIFY_INTEGRATION.md)

## 开源信息 / Open Source

- [License (MIT)](LICENSE)
- [Changelog](CHANGELOG.md)
- [Contributing Guide / 贡献指南](CONTRIBUTING.md)
- [Security Policy / 安全策略](SECURITY.md)
- [Code of Conduct / 行为准则](CODE_OF_CONDUCT.md)

## License

MIT
