# LookingGlass 部署文档

本文档介绍如何部署 LookingGlass 的 Master 和 Agent 组件。

## 目录

- [系统要求](#系统要求)
- [部署 Master](#部署-master)
- [部署 Agent](#部署-agent)
- [配置说明](#配置说明)
- [Docker 部署](#docker-部署)
- [常见问题](#常见问题)

## 系统要求

### Master 服务器

- **操作系统**: Linux (推荐 Ubuntu 20.04+, Debian 11+, CentOS 8+)
- **CPU**: 1 核心 (推荐 2 核心)
- **内存**: 512MB (推荐 1GB+)
- **磁盘**: 1GB 可用空间
- **网络**:
  - 开放端口 50051 (gRPC, 供 Agent 连接)
  - 开放端口 8080 (HTTP/WebSocket, 供前端访问)

### Agent 服务器

- **操作系统**: Linux (推荐 Ubuntu 20.04+, Debian 11+, CentOS 8+)
- **CPU**: 1 核心
- **内存**: 256MB (推荐 512MB)
- **磁盘**: 100MB 可用空间
- **网络**: 能够访问 Master 服务器的 50051 端口
- **工具**: 需要安装 `ping`, `mtr`, `nexttrace` 等诊断工具

## 部署 Master

### 1. 下载安装包

```bash
# 从 GitHub Releases 下载最新版本
wget https://github.com/your-org/lookingglass/releases/download/v1.0.0/lookingglass-master-v1.0.0.tar.gz

# 或使用 make 构建
make package-master
```

### 2. 解压安装包

```bash
tar -xzf lookingglass-master-v1.0.0.tar.gz
cd master
```

### 3. 配置 Master

编辑 `config.yaml.example` 并保存为 `config.yaml`:

```bash
cp config.yaml.example config.yaml
vi config.yaml
```

关键配置项：

```yaml
server:
  grpc_port: 50051      # Agent 连接端口
  ws_port: 8080         # 前端访问端口

auth:
  mode: api_key         # 认证模式: api_key 或 ip_whitelist
  api_keys:
    - "your-secret-key-change-this"  # 修改为强随机密钥

concurrency:
  global_max: 100       # 全局最大并发任务数
  agent_default_max: 5  # 每个 Agent 默认最大并发数

heartbeat:
  interval: 30          # 心跳间隔（秒）
  timeout: 90           # 心跳超时（秒）
```

### 4. 启动 Master

```bash
# 直接运行
./start.sh

# 或使用 systemd (推荐生产环境)
sudo cp lookingglass-master.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable lookingglass-master
sudo systemctl start lookingglass-master
sudo systemctl status lookingglass-master
```

### 5. 验证部署

```bash
# 检查日志
tail -f logs/master.log

# 测试 Web 界面
curl http://localhost:8080
curl http://localhost:8080/api/agents
```

## 部署 Agent

### 1. 下载安装包

```bash
# 从 GitHub Releases 下载
wget https://github.com/your-org/lookingglass/releases/download/v1.0.0/lookingglass-agent-v1.0.0.tar.gz

# 或使用 make 构建
make package-agent
```

### 2. 解压安装包

```bash
tar -xzf lookingglass-agent-v1.0.0.tar.gz
cd agent
```

### 3. 安装诊断工具

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y iputils-ping mtr-tiny

# CentOS/RHEL
sudo yum install -y iputils mtr

# 安装 NextTrace (可选但推荐)
wget https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_amd64
chmod +x nexttrace_linux_amd64
sudo mv nexttrace_linux_amd64 /usr/bin/nexttrace
```

### 4. 配置 Agent

编辑 `config.yaml.example` 并保存为 `config.yaml`:

```bash
cp config.yaml.example config.yaml
vi config.yaml
```

关键配置项：

```yaml
agent:
  id: "us-west-1"               # Agent 唯一标识符
  name: "美国西部-洛杉矶"       # 显示名称
  ipv4: ""                       # 公网 IPv4 (留空自动检测)
  ipv6: ""                       # 公网 IPv6 (留空自动检测，可选)
  hide_ip: true                  # 是否隐藏 IP 后两位
  max_concurrent: 5              # 最大并发任务数

  # Agent 元数据
  metadata:
    location: "Los Angeles, USA" # 地理位置
    provider: "DigitalOcean"     # 服务商
    idc: "sfo3"                  # 数据中心
    description: "US West Coast" # 描述

master:
  host: "master.example.com:50051"  # Master 地址 (必须修改)
  api_key: "your-secret-key"        # 与 Master 配置一致
  tls_enabled: false                # 是否启用 TLS
  heartbeat_interval: 30

executor:
  global_concurrency: 10  # 全局并发限制

  tasks:
    # 内置任务配置
    ping:
      enabled: true
      display_name: "Ping"
      requires_target: true
      concurrency:
        max: 3

    mtr:
      enabled: true
      display_name: "MTR"
      requires_target: true
      concurrency:
        max: 2

    nexttrace:
      enabled: true
      display_name: "NextTrace"
      requires_target: true
      concurrency:
        max: 2

    # 自定义命令示例 (可选)
    curl_test:
      enabled: false
      display_name: "HTTP Check"
      requires_target: true
      executor:
        type: command
        path: "/usr/bin/curl"
        default_args: ["-I", "-m", "10", "{target}"]
      concurrency:
        max: 2

# 详细配置说明请参阅: docs/TASK_CONFIG.md
```

**IP 地址自动检测**:

Agent 支持自动检测公网 IP 地址。如果配置文件中的 `ipv4` 或 `ipv6` 字段为空，Agent 会在启动时自动检测：

- **IPv4**: 通过多个公共 API 服务检测（ipify.org, ifconfig.me 等）
- **IPv6**: 尝试检测，如果失败不会影响启动（IPv6 是可选的）

示例输出：
```
IPv4 not configured, attempting auto-detection...
Auto-detected IPv4: 1.2.3.4
IPv6 not configured, attempting auto-detection...
Failed to auto-detect IPv6 (this is normal if IPv6 is not available): <nil>
```

**优点**:
- 无需手动配置公网 IP
- 适用于动态 IP 环境
- 支持 NAT 穿透场景

**注意**:
- 如果你的服务器位于 NAT 后面，自动检测会获取公网 IP
- 如果需要使用内网 IP，请手动配置 `ipv4` 字段

### 5. 启动 Agent

```bash
# 直接运行
./start.sh

# 或使用 systemd (推荐生产环境)
sudo cp lookingglass-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable lookingglass-agent
sudo systemctl start lookingglass-agent
sudo systemctl status lookingglass-agent
```

### 6. 验证部署

```bash
# 检查日志
tail -f logs/agent.log

# 应该看到类似输出：
# 2025-11-02 12:00:00|INFO|client/stream_client.go:134|Agent registered successfully via stream [message=Registration successful, heartbeat_interval=30]
```

## systemd 服务文件

### Master 服务文件

创建 `/etc/systemd/system/lookingglass-master.service`:

```ini
[Unit]
Description=LookingGlass Master Server
After=network.target

[Service]
Type=simple
User=lookingglass
Group=lookingglass
WorkingDirectory=/opt/lookingglass/master
ExecStart=/opt/lookingglass/master/master -config /opt/lookingglass/master/config.yaml
Restart=on-failure
RestartSec=10

# 日志配置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=lookingglass-master

# 资源限制
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### Agent 服务文件

创建 `/etc/systemd/system/lookingglass-agent.service`:

```ini
[Unit]
Description=LookingGlass Agent
After=network.target

[Service]
Type=simple
User=lookingglass
Group=lookingglass
WorkingDirectory=/opt/lookingglass/agent
ExecStart=/opt/lookingglass/agent/agent -config /opt/lookingglass/agent/config.yaml
Restart=always
RestartSec=10

# 日志配置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=lookingglass-agent

# 资源限制
LimitNOFILE=4096

[Install]
WantedBy=multi-user.target
```

## 配置说明

### Master 认证模式

LookingGlass 支持两种认证模式：

#### 1. API Key 模式 (推荐)

```yaml
auth:
  mode: api_key
  api_keys:
    - "key1-for-production-agents"
    - "key2-for-testing-agents"
```

- Agent 通过 API Key 认证
- 支持多个 API Key，便于密钥轮换
- 推荐使用 32+ 字符的强随机密钥

#### 2. IP 白名单模式

```yaml
auth:
  mode: ip_whitelist
  allowed_ips:
    - "1.2.3.4"
    - "5.6.7.8/24"
```

- 仅允许指定 IP 的 Agent 连接
- 支持 CIDR 格式
- 适用于固定 IP 的内网环境

### 并发控制

LookingGlass 使用两级并发控制：

1. **全局并发限制** (`global_max`): 整个系统的最大并发任务数
2. **Agent 并发限制** (`agent_default_max`): 单个 Agent 的默认最大并发数

```yaml
concurrency:
  global_max: 100
  agent_default_max: 5
  agent_limits:
    us-west-1: 10  # 可为特定 Agent 设置不同的限制
```

### 日志配置

```yaml
log:
  level: info        # debug, info, warn, error
  file: logs/master.log
  console: true
```

## Docker 部署

Docker 是推荐的部署方式，特别适合快速部署和测试环境。

### 前提条件

- Docker 20.10+
- Docker Compose 2.0+

### 使用 Docker Compose (推荐)

#### 1. 准备配置文件

```bash
# 复制示例配置
cp master/config.yaml.example master/config.yaml
cp agent/config.yaml.example agent/config.yaml

# 编辑配置文件
vi master/config.yaml
vi agent/config.yaml
```

**重要配置项**:

Master (`master/config.yaml`):
```yaml
server:
  grpc_port: 50051
  ws_port: 8080

auth:
  mode: api_key
  api_keys:
    - "your-secret-key-change-this"
```

Agent (`agent/config.yaml`):
```yaml
master:
  host: "master:50051"  # 使用 Docker 服务名
  api_key: "your-secret-key-change-this"
```

#### 2. 启动服务

```bash
# 构建镜像
make docker-build

# 启动所有服务
make docker-up

# 或者直接使用 docker-compose
docker-compose up -d
```

#### 3. 验证部署

```bash
# 查看容器状态
docker-compose ps

# 查看日志
make docker-logs
# 或
docker-compose logs -f

# 检查 Master Web 界面
curl http://localhost:8080/api/agents
```

#### 4. 管理服务

```bash
# 停止服务
make docker-down

# 重启服务
make docker-restart

# 清理所有资源（包括卷）
make docker-clean
```

### 使用单独的 Docker 容器

#### 构建 Master 镜像

```bash
docker build -f Dockerfile.master -t lookingglass-master:latest .
```

#### 运行 Master 容器

```bash
docker run -d \
  --name lookingglass-master \
  -p 50051:50051 \
  -p 8080:8080 \
  -v $(pwd)/master/config.yaml:/app/config.yaml:ro \
  -v master-logs:/app/logs \
  lookingglass-master:latest
```

#### 构建 Agent 镜像

```bash
docker build -f Dockerfile.agent -t lookingglass-agent:latest .
```

#### 运行 Agent 容器

```bash
docker run -d \
  --name lookingglass-agent \
  --cap-add=NET_RAW \
  --cap-add=NET_ADMIN \
  -v $(pwd)/agent/config.yaml:/app/config.yaml:ro \
  -v agent-logs:/app/logs \
  lookingglass-agent:latest
```

### Docker 部署生产环境

#### 使用环境变量

创建 `.env` 文件：

```bash
# Master settings
MASTER_GRPC_PORT=50051
MASTER_WS_PORT=8080
MASTER_API_KEY=your-production-api-key

# Agent settings
AGENT_ID=prod-agent-1
AGENT_NAME=Production Agent 1
MASTER_HOST=master.example.com:50051
```

修改 `docker-compose.yml` 使用环境变量。

#### 使用外部网络

```yaml
networks:
  lookingglass:
    external: true
    name: production_network
```

#### 持久化日志

```yaml
volumes:
  master-logs:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /var/log/lookingglass/master
  agent-logs:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /var/log/lookingglass/agent
```

### Docker 网络配置

#### 跨主机部署

如果 Master 和 Agent 在不同的主机上：

1. Master 主机：
   ```bash
   # 只运行 Master
   docker-compose up -d master
   ```

2. Agent 主机：
   ```bash
   # 修改 agent/config.yaml 中的 master.host 为 Master 公网地址
   vi agent/config.yaml

   # 只运行 Agent
   docker-compose up -d agent
   ```

### 资源限制

在 `docker-compose.yml` 中添加资源限制：

```yaml
services:
  master:
    # ... 其他配置 ...
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M

  agent:
    # ... 其他配置 ...
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
```

### 健康检查

容器已配置健康检查：

```bash
# 查看健康状态
docker-compose ps

# 详细健康信息
docker inspect --format='{{.State.Health}}' lookingglass-master
```

### 日志管理

```bash
# 实时查看日志
docker-compose logs -f master
docker-compose logs -f agent

# 查看最近 100 行日志
docker-compose logs --tail=100 master

# 导出日志
docker-compose logs master > master.log
```

### 备份和恢复

```bash
# 备份配置
tar -czf backup-$(date +%Y%m%d).tar.gz \
  master/config.yaml \
  agent/config.yaml \
  docker-compose.yml

# 备份日志卷
docker run --rm \
  -v master-logs:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/master-logs-$(date +%Y%m%d).tar.gz -C /data .
```

### Docker 故障排查

#### 容器无法启动

```bash
# 查看详细日志
docker-compose logs master
docker-compose logs agent

# 检查容器状态
docker-compose ps

# 进入容器调试
docker-compose exec master sh
docker-compose exec agent sh
```

#### Agent 无法连接 Master

1. 检查网络连通性：
   ```bash
   docker-compose exec agent ping master
   docker-compose exec agent nc -zv master 50051
   ```

2. 检查配置文件中的 `master.host`：
   - Docker Compose: 使用服务名 `master:50051`
   - 跨主机: 使用公网 IP 或域名

#### 权限问题

```bash
# Agent 需要 NET_RAW 权限执行 ping
docker-compose exec agent ping -c 1 8.8.8.8

# 如果失败，检查 docker-compose.yml 中的 cap_add 配置
```

### 更新 Docker 镜像

```bash
# 拉取最新代码
git pull

# 重新构建镜像
make docker-build

# 重启服务
make docker-down
make docker-up
```

## 防火墙配置

### Master 服务器

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow 50051/tcp comment 'LookingGlass gRPC'
sudo ufw allow 8080/tcp comment 'LookingGlass Web'

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=50051/tcp
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

### Agent 服务器

Agent 不需要开放任何端口（使用出站连接到 Master）。

## 监控和维护

### 健康检查

```bash
# Master 健康检查
curl http://localhost:8080/api/agents

# 检查 Agent 状态
curl http://localhost:8080/api/agents | jq '.agents[] | select(.id=="us-west-1")'
```

### 日志查看

```bash
# 查看 systemd 日志
sudo journalctl -u lookingglass-master -f
sudo journalctl -u lookingglass-agent -f

# 查看文件日志
tail -f /opt/lookingglass/master/logs/master.log
tail -f /opt/lookingglass/agent/logs/agent.log
```

### 重启服务

```bash
# 重启 Master
sudo systemctl restart lookingglass-master

# 重启 Agent
sudo systemctl restart lookingglass-agent
```

## 常见问题

### Agent 无法连接到 Master

1. 检查网络连通性：
   ```bash
   telnet master.example.com 50051
   nc -zv master.example.com 50051
   ```

2. 检查防火墙设置

3. 检查 API Key 是否匹配

4. 查看 Agent 日志：
   ```bash
   tail -f logs/agent.log
   ```

### Web 界面无法访问

1. 检查 Master 是否正常运行：
   ```bash
   sudo systemctl status lookingglass-master
   ```

2. 检查端口是否监听：
   ```bash
   netstat -tlnp | grep 8080
   ```

3. 检查防火墙设置

### Agent 显示 Offline

1. 检查 Agent 是否运行：
   ```bash
   sudo systemctl status lookingglass-agent
   ```

2. 检查心跳超时配置（Master 的 `heartbeat.timeout` 应大于 Agent 的 `heartbeat_interval`）

3. 检查网络稳定性

### 任务执行失败

1. 检查诊断工具是否已安装：
   ```bash
   which ping mtr nexttrace
   ```

2. 检查工具路径配置是否正确（`executor.ping_path` 等）

3. 检查并发限制是否过低

4. 查看 Agent 日志排查具体错误

## 升级

### 升级 Master

```bash
# 备份配置
cp config.yaml config.yaml.bak

# 停止服务
sudo systemctl stop lookingglass-master

# 解压新版本
tar -xzf lookingglass-master-v1.1.0.tar.gz
cd master

# 恢复配置
cp ../master-old/config.yaml .

# 启动服务
sudo systemctl start lookingglass-master

# 检查状态
sudo systemctl status lookingglass-master
tail -f logs/master.log
```

### 升级 Agent

```bash
# 备份配置
cp config.yaml config.yaml.bak

# 停止服务
sudo systemctl stop lookingglass-agent

# 解压新版本
tar -xzf lookingglass-agent-v1.1.0.tar.gz
cd agent

# 恢复配置
cp ../agent-old/config.yaml .

# 启动服务
sudo systemctl start lookingglass-agent

# 检查状态
sudo systemctl status lookingglass-agent
tail -f logs/agent.log
```

## 安全建议

1. **使用强 API Key**: 32+ 字符随机密钥
2. **启用 TLS**: 生产环境建议启用 gRPC TLS
3. **定期轮换密钥**: 建议每 3-6 个月更换 API Key
4. **限制访问**: 使用防火墙限制 Master Web 界面访问
5. **最小权限运行**: 使用专用用户运行服务，避免 root
6. **监控日志**: 定期检查异常访问和错误

## 性能优化

1. **调整并发限制**: 根据服务器性能调整 `global_max` 和 `agent_default_max`
2. **使用 SSD**: Master 日志建议使用 SSD 存储
3. **网络优化**: 使用低延迟网络连接 Master 和 Agent
4. **资源监控**: 监控 CPU、内存、网络使用情况

## 技术支持

- GitHub Issues: https://github.com/your-org/lookingglass/issues
- 文档: https://docs.lookingglass.example.com
- Email: support@example.com
