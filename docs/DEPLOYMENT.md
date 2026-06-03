# Deployment Guide

Production-ready deployment strategies for AgentForge, from single-machine to Kubernetes clusters.

---

## Prerequisites

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| **CPU** | 1 core | 4 cores |
| **RAM** | 512 MB | 4 GB |
| **Disk** | 100 MB | 10 GB |
| **OS** | Linux 3.10+ | Linux 5.0+ |
| **Go** | 1.24+ | 1.24+ |

### Dependencies

- Go 1.24+ (for building from source)
- Git (for MeMex memory versioning)
- Optional: Docker (for containerized deployment)
- Optional: PostgreSQL (for production memory store)

---

## Deployment Methods

### Method 1: Binary Installation

Fastest way to get started — download pre-compiled binary.

```bash
# Download latest release
curl -L https://github.com/agentforge/agentforge/releases/latest/download/agentforge-linux-amd64 \
  -o agentforge
chmod +x agentforge

# Or use Homebrew (macOS)
brew install agentforge/tap/agentforge

# Or use Go install
go install github.com/agentforge/agentforge/cmd/agentforge@latest

# Verify installation
agentforge version
```

---

### Method 2: Docker

Containerized deployment with all dependencies included.

```bash
# Pull latest image
docker pull agentforge/agentforge:latest

# Run with configuration
docker run -d \
  --name agentforge \
  -p 8080:8080 \
  -p 9090:9090 \
  -v ~/.agentforge:/root/.agentforge \
  -e OPENAI_API_KEY=sk-... \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  agentforge/agentforge:latest

# View logs
docker logs -f agentforge

# Stop
docker stop agentforge
```

#### Docker Compose

For local development with Postgres, Redis, etc.:

```yaml
# docker-compose.yaml
version: '3.8'
services:
  agentforge:
    image: agentforge/agentforge:latest
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ~/.agentforge:/root/.agentforge
      - ./config.yaml:/etc/agentforge/config.yaml:ro
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  # Optional: Postgres for production memory store
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: agentforge
      POSTGRES_USER: agentforge
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

Run:
```bash
docker-compose up -d
```

---

### Method 3: Kubernetes

Highly available, scalable deployment for large organizations.

#### Namespace and ConfigMap

```yaml
# agentforge-namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: agentforge
---
# agentforge-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: agentforge-config
  namespace: agentforge
data:
  config.yaml: |
    daemon:
      host: "0.0.0.0"
      port: 8080
      mcp_port: 9090
      log_level: "info"
    
    llm:
      default_provider: "openai"
      models:
        openai:
          model: "gpt-4.1"
          timeout: 30s
        anthropic:
          model: "claude-sonnet-4-20250514"
    
    memory:
      auto_commit: true
      compaction:
        enabled: true
```

#### Deployment

```yaml
# agentforge-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agentforge
  namespace: agentforge
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: agentforge
  template:
    metadata:
      labels:
        app: agentforge
    spec:
      serviceAccountName: agentforge
      containers:
      - name: agentforge
        image: agentforge/agentforge:latest
        imagePullPolicy: Always
        ports:
        - name: http
          containerPort: 8080
        - name: mcp
          containerPort: 9090
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: agentforge-secrets
              key: openai-api-key
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: agentforge-secrets
              key: anthropic-api-key
        volumeMounts:
        - name: config
          mountPath: /etc/agentforge
          readOnly: true
        - name: memory
          mountPath: /root/.agentforge/memory
        
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 2000m
            memory: 2Gi
        
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
          failureThreshold: 3
      
      volumes:
      - name: config
        configMap:
          name: agentforge-config
      - name: memory
        persistentVolumeClaim:
          claimName: agentforge-memory
```

#### Service and Ingress

```yaml
# agentforge-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: agentforge
  namespace: agentforge
spec:
  type: LoadBalancer
  ports:
  - name: http
    port: 80
    targetPort: http
    protocol: TCP
  - name: mcp
    port: 9090
    targetPort: mcp
    protocol: TCP
  selector:
    app: agentforge
---
# agentforge-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: agentforge
  namespace: agentforge
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - agentforge.example.com
    secretName: agentforge-tls
  rules:
  - host: agentforge.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: agentforge
            port:
              name: http
```

#### Persistent Storage

```yaml
# agentforge-pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: agentforge-memory
  namespace: agentforge
spec:
  accessModes:
    - ReadWriteMany    # For shared memory store
  storageClassName: nfs-client  # NFS, EBS, etc.
  resources:
    requests:
      storage: 10Gi
```

#### Deploy to Kubernetes

```bash
# Create namespace and secrets
kubectl create namespace agentforge
kubectl create secret generic agentforge-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  --from-literal=anthropic-api-key=$ANTHROPIC_API_KEY \
  -n agentforge

# Apply manifests
kubectl apply -f agentforge-namespace.yaml
kubectl apply -f agentforge-config.yaml
kubectl apply -f agentforge-pvc.yaml
kubectl apply -f agentforge-deployment.yaml
kubectl apply -f agentforge-service.yaml
kubectl apply -f agentforge-ingress.yaml

# Check deployment
kubectl get pods -n agentforge
kubectl get svc -n agentforge
kubectl get ingress -n agentforge

# View logs
kubectl logs -f -n agentforge deployment/agentforge

# Port-forward for testing
kubectl port-forward -n agentforge svc/agentforge 8080:80 9090:9090
```

---

### Method 4: Systemd Service

For traditional Linux deployments.

```ini
# /etc/systemd/system/agentforge.service
[Unit]
Description=AgentForge Agent Orchestration Framework
After=network.target

[Service]
Type=simple
User=agentforge
WorkingDirectory=/opt/agentforge
ExecStart=/usr/local/bin/agentforge daemon --config /etc/agentforge/config.yaml
Restart=on-failure
RestartSec=10

# Environment variables
Environment="OPENAI_API_KEY=${OPENAI_API_KEY}"
Environment="ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}"

# Security hardening
PrivateTmp=true
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/home/agentforge/.agentforge

[Install]
WantedBy=multi-user.target
```

Setup:

```bash
# Create service user
sudo useradd -m -d /home/agentforge -s /bin/false agentforge

# Copy binary
sudo cp agentforge /usr/local/bin/
sudo chmod +x /usr/local/bin/agentforge

# Install service
sudo cp agentforge.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable agentforge
sudo systemctl start agentforge

# Monitor
sudo journalctl -u agentforge -f
```

---

## Configuration

### Environment Variables

```bash
# API Keys
export OPENAI_API_KEY=sk-proj-...
export ANTHROPIC_API_KEY=sk-ant-...

# Telegram/Discord
export TELEGRAM_BOT_TOKEN=123456:ABC...
export DISCORD_BOT_TOKEN=ODg...

# Config file location
export AGENTFORGE_CONFIG=/etc/agentforge/config.yaml

# Log level
export LOG_LEVEL=info  # debug, info, warn, error

# Port overrides
export DAEMON_PORT=8080
export MCP_PORT=9090
```

### Config File

Create `~/.agentforge/agentforge.yaml`:

```yaml
daemon:
  host: "0.0.0.0"
  port: 8080
  mcp_port: 9090
  log_level: "info"
  metrics: true

memory:
  root: "$HOME/.agentforge/memory"
  auto_commit: true
  commit_interval: 30s

security:
  capability_secret: "${AGENTFORGE_SECRET}"
  default_token_budget: 1000000
  default_timeout: 3600s

llm:
  default_provider: "openai"
  fallback_chain:
    - "openai"
    - "anthropic"
    - "ollama"

tools:
  registry:
    - file_io
    - web_fetch
    - shell_exec
    - memory_search
    # ... add more as needed

channels:
  telegram:
    enabled: false
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    mode: "polling"
  
  discord:
    enabled: false
    bot_token: "${DISCORD_BOT_TOKEN}"
```

---

## Health Checks

### HTTP Health Endpoint

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "uptime_seconds": 3600,
  "agents_active": 5,
  "memory_usage_mb": 128,
  "last_check": "2026-06-03T15:30:45Z"
}
```

### Manual Health Verification

```bash
# Check dashboard accessibility
curl http://localhost:8080/

# Check MCP server
curl http://localhost:9090/mcp/info

# Check bus connectivity
agentforge status

# Check memory store
ls -la ~/.agentforge/memory/
```

---

## Scaling

### Horizontal Scaling (Kubernetes)

Scale to handle more agents:

```bash
kubectl scale deployment agentforge --replicas=10 -n agentforge
```

Each pod runs independently, sharing memory store via NFS/EBS.

### Vertical Scaling (Single Machine)

Increase resources for single instance:

```bash
# Edit systemd service
sudo systemctl edit agentforge

[Service]
# Increase max open files for more goroutines
LimitNOFILE=65536
```

Or in Docker:

```bash
docker run --memory=8g --cpus=4 agentforge/agentforge:latest
```

---

## Monitoring

### Prometheus Metrics

Enable metrics in config:

```yaml
daemon:
  metrics: true  # Expose /metrics endpoint
```

Scrape configuration:

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'agentforge'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Key Metrics

- `agentforge_agents_active` — Number of running agents
- `agentforge_bus_messages_total` — Total messages published
- `agentforge_tool_calls_total` — Total tool invocations
- `agentforge_tokens_consumed` — LLM tokens used
- `agentforge_cost_total_usd` — Total cost in USD
- `agentforge_memory_size_bytes` — Size of memory store

### Alerting Rules

```yaml
groups:
  - name: agentforge
    rules:
      - alert: HighAgentLoad
        expr: agentforge_agents_active > 100
        for: 5m
        annotations:
          summary: "High number of agents running"
      
      - alert: HighTokenConsumption
        expr: rate(agentforge_tokens_consumed[5m]) > 10000
        for: 5m
        annotations:
          summary: "High token consumption rate"
      
      - alert: DaemonDown
        expr: up{job="agentforge"} == 0
        for: 1m
        annotations:
          summary: "AgentForge daemon is down"
```

---

## Backup and Recovery

### Memory Store Backup

The memory store is a Git repository. Back it up like any Git repo:

```bash
# Local backup (cron job)
0 2 * * * rsync -av ~/.agentforge/memory /backups/agentforge-memory

# Remote backup (S3)
0 3 * * * cd ~/.agentforge/memory && git push origin main

# Or use Git hooks
git config hooks.postcommit "aws s3 sync . s3://backups/agentforge-memory"
```

### Session Recovery

Sessions are auto-archived on shutdown. Restore with:

```bash
agentforge restore --session SESSION_ID
```

### Database Recovery (PostgreSQL)

If using PostgreSQL for memory store:

```bash
# Backup
pg_dump agentforge > agentforge.sql

# Restore
psql agentforge < agentforge.sql
```

---

## Troubleshooting

### Daemon won't start

```bash
# Check logs
agentforge daemon 2>&1 | head -20

# Verify config
agentforge config list

# Check port availability
lsof -i :8080
```

**Common Issues:**
- Port already in use → Change `daemon.port` in config
- Config file not found → Set `AGENTFORGE_CONFIG` env var
- Missing API keys → Set env vars (OPENAI_API_KEY, etc.)

### High memory usage

```bash
# Check memory store size
du -sh ~/.agentforge/memory

# Trigger compaction
agentforge memory compact

# Archive old sessions
rm ~/.agentforge/sessions/*-2026-01-*.json
```

### Slow performance

```bash
# Check active agent count
curl http://localhost:8080/health | jq '.agents_active'

# Profile CPU usage
go tool pprof http://localhost:8080/debug/pprof/profile

# Check memory store query performance
cd ~/.agentforge/memory && sqlite3 index.db
sqlite> .timer on
sqlite> SELECT * FROM documents WHERE content LIKE '%keyword%';
```

### Network issues

```bash
# Test connectivity to LLM provider
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/models

# Test Telegram bot token
curl https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe

# Test Discord token
curl -H "Authorization: Bot ${DISCORD_BOT_TOKEN}" \
  https://discord.com/api/v10/users/@me
```

---

## Security Hardening

### TLS/HTTPS

Enable TLS for encrypted communication:

```yaml
daemon:
  tls:
    enabled: true
    cert_file: /etc/agentforge/cert.pem
    key_file: /etc/agentforge/key.pem
```

Generate certificate:

```bash
openssl req -x509 -newkey rsa:4096 \
  -keyout key.pem -out cert.pem \
  -days 365 -nodes
```

### Authentication

Configure strong authentication:

```yaml
auth:
  admin_password: "${ADMIN_PASSWORD}"  # Strong password
  jwt_expiry_mins: 15                  # Short-lived tokens
  refresh_expiry_days: 7
  allow_registration: false            # Disable self-signup
```

### Network Segmentation

Restrict daemon to internal network:

```yaml
daemon:
  host: "127.0.0.1"  # Only localhost (use proxy)
  # or
  host: "10.0.0.5"   # Only internal network
```

### Capability Tokens

Issue minimal tokens:

```bash
# Instead of:
agentforge spawn worker --fs-allow "/" --domain-allow "*"

# Do this:
agentforge spawn worker \
  --fs-allow "/home/worker/jobs/**" \
  --domain-allow "api.openai.com" \
  --token-budget 100000 \
  --timeout 300
```

---

## Upgrade Procedure

### Binary Upgrade

```bash
# Download new version
curl -L https://github.com/agentforge/agentforge/releases/download/v0.3.1/agentforge-linux-amd64 \
  -o agentforge.new
chmod +x agentforge.new

# Backup old binary
cp agentforge agentforge.bak

# Replace binary
mv agentforge.new agentforge

# Restart daemon
systemctl restart agentforge

# Verify
agentforge version
```

### Docker Upgrade

```bash
# Pull new image
docker pull agentforge/agentforge:latest

# Stop old container
docker stop agentforge

# Remove old container
docker rm agentforge

# Start new container
docker run -d \
  --name agentforge \
  -p 8080:8080 -p 9090:9090 \
  -v ~/.agentforge:/root/.agentforge \
  agentforge/agentforge:latest

# Check logs
docker logs agentforge
```

### Kubernetes Upgrade

```bash
# Update image in deployment
kubectl set image deployment/agentforge \
  agentforge=agentforge/agentforge:latest \
  -n agentforge

# Monitor rollout
kubectl rollout status deployment/agentforge -n agentforge

# Rollback if needed
kubectl rollout undo deployment/agentforge -n agentforge
```

---

## Performance Optimization

### Memory Store Indexing

Rebuild search index for better query performance:

```bash
cd ~/.agentforge/memory
sqlite3 index.db "REINDEX documents;"
```

### Connection Pooling

Configure connection limits in config:

```yaml
llm:
  openai:
    max_connections: 10
    connection_timeout: 5s
```

### Caching

Cache frequently accessed resources:

```yaml
memory:
  cache:
    enabled: true
    ttl: 300s
    max_size: 1000
```

---

## Support and Resources

- **Documentation:** https://docs.agentforge.dev
- **Discord Community:** https://discord.gg/agentforge
- **GitHub Issues:** https://github.com/agentforge/agentforge/issues
- **Security:** security@agentforge.dev

---

**Status:** 🟢 Production Ready — All security issues fixed, comprehensive test coverage, ready for enterprise deployment.
