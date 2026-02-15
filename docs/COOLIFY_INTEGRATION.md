# Coolify Integration Guide

This guide explains how to integrate `metrics-agent` with [Coolify](https://coolify.io/) for Docker container resource monitoring.

## Overview

`metrics-agent` provides these API capabilities:

- Container list and runtime status
- Real-time CPU, memory, and network stats
- 24-hour historical metrics (5-minute interval)
- Request statistics by parsing Traefik access logs
- Resource limit update endpoint for containers

The service exposes HTTP endpoints under `/api/*`.

## Runtime Defaults

- Default app port: `3000`
- Compose deployment example in this document: `8080`
- Required token env: `METRICS_AGENT_TOKEN`

## Deployment via Coolify

### 1. Add a Docker Compose Service

In Coolify, create a new Docker Compose service and use:

```yaml
version: '3.8'

services:
  metrics-agent:
    build: https://github.com/EagleFandel/metrics-agent.git
    container_name: nomo-metrics-agent
    restart: unless-stopped
    environment:
      - METRICS_AGENT_TOKEN=${METRICS_AGENT_TOKEN}
      - METRICS_AGENT_PORT=8080
      - TRAEFIK_LOG_PATH=/var/log/traefik/access.log
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /var/log/traefik:/var/log/traefik:ro
    ports:
      - "8080:8080"
    networks:
      - coolify

networks:
  coolify:
    external: true
```

### 2. Required Environment Variables

Set in Coolify:

- `METRICS_AGENT_TOKEN`: strong random secret

Optional:

- `METRICS_AGENT_PORT`: service port inside container (example uses `8080`)
- `TRAEFIK_LOG_PATH`: default `/var/log/traefik/access.log`

### 3. Mount Volumes

- Docker socket: `/var/run/docker.sock:/var/run/docker.sock`
- Traefik logs: `/var/log/traefik:/var/log/traefik:ro`

### 4. Deploy and Verify

Health check:

```bash
curl http://your-server:8080/api/health
```

Expected response:

```json
{"status":"ok","version":"1.2.3","uptime":"..."}
```

## API Usage Examples

All authenticated API calls require:

```text
Authorization: Bearer YOUR_TOKEN
```

### Get All Running Container Stats

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://your-server:8080/api/stats?filter=coolify"
```

### Get a Specific Container's Current Metrics

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://your-server:8080/api/containers/CONTAINER_ID/stats"
```

### Get a Specific Container's History

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://your-server:8080/api/containers/CONTAINER_ID/history"
```

### Get Aggregated Data in One Request

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://your-server:8080/api/containers/CONTAINER_ID/all?domain=app.example.com"
```

### Get Request Statistics by Domain

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://your-server:8080/api/requests?domain=app.example.com"
```

## Integration Example (Next.js)

```typescript
const METRICS_AGENT_URL = process.env.METRICS_AGENT_URL
const METRICS_AGENT_TOKEN = process.env.METRICS_AGENT_TOKEN

export async function getContainerAllData(containerId: string, domain?: string) {
  const query = domain ? `?domain=${encodeURIComponent(domain)}` : ''
  const response = await fetch(
    `${METRICS_AGENT_URL}/api/containers/${containerId}/all${query}`,
    {
      headers: {
        Authorization: `Bearer ${METRICS_AGENT_TOKEN}`,
      },
      cache: 'no-store',
    }
  )

  if (!response.ok) {
    throw new Error(`Failed to fetch container data: ${response.status}`)
  }

  return response.json()
}
```

## Troubleshooting

### "Container not found"

1. Verify container exists: `docker ps -a`
2. Check whether Docker socket is mounted
3. Use full container ID if short ID matching fails

### "Unauthorized"

1. Verify `METRICS_AGENT_TOKEN` is configured in service runtime
2. Verify header format is exactly `Authorization: Bearer YOUR_TOKEN`

### Request Stats Always 0

1. Confirm Traefik access log is mounted read-only
2. Confirm log format is JSON
3. Confirm `domain` query matches `RequestHost` in logs

## Security Recommendations

1. Use a strong random token for `METRICS_AGENT_TOKEN`
2. Restrict API access by network policy or reverse proxy
3. Expose only trusted routes to the public internet
4. Rotate tokens periodically

## Support

- GitHub Issues: https://github.com/EagleFandel/metrics-agent/issues
- Coolify Community: https://discord.gg/coolify
