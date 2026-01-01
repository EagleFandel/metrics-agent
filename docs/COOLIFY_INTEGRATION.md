# Coolify Integration Guide

This guide explains how to integrate metrics-agent with [Coolify](https://coolify.io/) to monitor your deployed applications.

## Overview

metrics-agent provides real-time CPU, memory, and network metrics for Docker containers. When integrated with Coolify, you can monitor all your deployed applications from a single dashboard.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Coolify Server                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   App 1     │  │   App 2     │  │   App 3     │     │
│  │ (container) │  │ (container) │  │ (container) │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│                         │                               │
│                         ▼                               │
│              ┌─────────────────────┐                   │
│              │   metrics-agent     │                   │
│              │   (port 9100)       │                   │
│              └─────────────────────┘                   │
│                         │                               │
└─────────────────────────│───────────────────────────────┘
                          │
                          ▼
              ┌─────────────────────┐
              │   Your Dashboard    │
              │   (API calls)       │
              └─────────────────────┘
```

## Deployment Options

### Option 1: Deploy via Coolify (Recommended)

1. In Coolify, create a new project for infrastructure services
2. Add a new application with Docker Compose
3. Use this `docker-compose.yml`:

```yaml
version: "3"

services:
  metrics-agent:
    image: ghcr.io/eaglefandel/metrics-agent:latest
    # Or build from source:
    # build: https://github.com/EagleFandel/metrics-agent.git
    container_name: metrics-agent
    restart: unless-stopped
    environment:
      - API_TOKEN=${API_TOKEN}
      - PORT=9100
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    ports:
      - "9100:9100"
    networks:
      - coolify
```

4. Set environment variable `API_TOKEN` to a secure random string
5. Deploy

### Option 2: Deploy via SSH

```bash
# On your Coolify server
cd /opt
git clone https://github.com/EagleFandel/metrics-agent.git
cd metrics-agent

# Create .env file
echo "API_TOKEN=your-secure-token" > .env

# Start with Docker Compose
docker-compose up -d
```

### Option 3: Run as Systemd Service

```bash
# Build binary
go build -o /usr/local/bin/metrics-agent

# Create systemd service
cat > /etc/systemd/system/metrics-agent.service << EOF
[Unit]
Description=Metrics Agent
After=docker.service

[Service]
Environment=API_TOKEN=your-secure-token
Environment=PORT=9100
ExecStart=/usr/local/bin/metrics-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF

systemctl enable metrics-agent
systemctl start metrics-agent
```

## Network Configuration

### Coolify Network

metrics-agent must be on the same Docker network as your Coolify applications to access their metrics. The default Coolify network is `coolify`.

```yaml
networks:
  coolify:
    external: true
```

### Firewall Rules

If accessing metrics-agent from outside the server:

```bash
# Allow port 9100 (adjust for your firewall)
ufw allow 9100/tcp
```

For production, consider:
- Using a reverse proxy (Caddy/Nginx) with HTTPS
- Restricting access to specific IPs
- Using Coolify's built-in proxy

## API Usage with Coolify

### Get All Container Metrics

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://your-server:9100/metrics
```

### Get Specific Application Metrics

Coolify containers are named with a pattern. To get metrics for a specific app:

```bash
# Get metrics for container by ID or name
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://your-server:9100/metrics/CONTAINER_ID"
```

### Finding Coolify Container IDs

Coolify stores the container ID in the application settings. You can also find it via:

```bash
# List all containers with Coolify labels
docker ps --filter "label=coolify.managed=true" --format "{{.ID}} {{.Names}}"
```

## Integration Example (Next.js)

```typescript
// lib/metrics-agent/client.ts
const METRICS_AGENT_URL = process.env.METRICS_AGENT_URL
const METRICS_AGENT_TOKEN = process.env.METRICS_AGENT_TOKEN

export async function getContainerMetrics(containerId: string) {
  const response = await fetch(
    `${METRICS_AGENT_URL}/metrics/${containerId}`,
    {
      headers: {
        'Authorization': `Bearer ${METRICS_AGENT_TOKEN}`,
      },
    }
  )
  
  if (!response.ok) {
    throw new Error('Failed to fetch metrics')
  }
  
  return response.json()
}

// Usage in API route
export async function GET(request: Request) {
  const { searchParams } = new URL(request.url)
  const containerId = searchParams.get('containerId')
  
  const metrics = await getContainerMetrics(containerId)
  
  return Response.json({
    cpu_percent: metrics.cpu_percent,
    memory_mb: metrics.memory_usage / 1024 / 1024,
    memory_percent: metrics.memory_percent,
    network_rx_mb: metrics.network_rx / 1024 / 1024,
    network_tx_mb: metrics.network_tx / 1024 / 1024,
  })
}
```

## Storing Container IDs

To link Coolify applications with metrics-agent, store the container ID in your database:

```sql
-- Add container_id column to projects table
ALTER TABLE projects ADD COLUMN container_id TEXT;
```

After deployment, update the container ID:

```typescript
// After Coolify deployment
const app = await coolifyClient.getApplication(appUuid)
const containerId = app.container_id // or fetch via Docker API

await supabase
  .from('projects')
  .update({ container_id: containerId })
  .eq('coolify_app_id', appUuid)
```

## Troubleshooting

### "Container not found" Error

1. Verify the container is running: `docker ps`
2. Check if metrics-agent can access Docker socket
3. Ensure container ID is correct (not truncated)

### "Unauthorized" Error

1. Verify API_TOKEN matches in both services
2. Check Authorization header format: `Bearer TOKEN`

### No Network Metrics

Network metrics require the container to have network activity. New containers may show 0 until traffic occurs.

### High Memory Usage

metrics-agent caches container stats. If memory grows:
1. Check for container churn (many start/stop cycles)
2. Restart metrics-agent periodically via cron

## Security Recommendations

1. **Use HTTPS**: Put metrics-agent behind a reverse proxy with TLS
2. **Rotate Tokens**: Change API_TOKEN periodically
3. **Network Isolation**: Only expose to trusted networks
4. **Read-only Socket**: Mount Docker socket as read-only (`:ro`)

## Prometheus Integration (Optional)

metrics-agent can export Prometheus-compatible metrics:

```bash
curl http://your-server:9100/metrics/prometheus
```

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'coolify-containers'
    static_configs:
      - targets: ['your-server:9100']
    bearer_token: 'YOUR_TOKEN'
```

## Support

- GitHub Issues: https://github.com/EagleFandel/metrics-agent/issues
- Coolify Discord: https://discord.gg/coolify
