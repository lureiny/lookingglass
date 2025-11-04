# Docker 快速开始

本文档提供使用 Docker 部署 LookingGlass 的快速指南。

## 快速开始（5 分钟）

### 1. 克隆仓库

```bash
git clone https://github.com/your-org/lookingglass.git
cd lookingglass
```

### 2. 准备配置文件

```bash
# 复制示例配置
cp master/config.yaml.example master/config.yaml
cp agent/config.yaml.example agent/config.yaml

# 生成强随机 API Key
API_KEY=$(openssl rand -hex 32)
echo "Your API Key: $API_KEY"

# 自动更新配置文件中的 API Key
sed -i "s/your-secret-key-change-this/$API_KEY/g" master/config.yaml
sed -i "s/your-secret-key-change-this/$API_KEY/g" agent/config.yaml
```

### 3. 启动服务

```bash
# 构建并启动
make docker-build
make docker-up

# 或者一步到位
docker-compose up -d --build
```

### 4. 访问 Web 界面

打开浏览器访问: http://localhost:8080

### 5. 查看日志

```bash
# 查看所有服务日志
make docker-logs

# 或分别查看
docker-compose logs -f master
docker-compose logs -f agent
```

## 常用命令

```bash
# 查看容器状态
docker-compose ps

# 停止服务
make docker-down

# 重启服务
make docker-restart

# 清理所有资源
make docker-clean
```

## 自定义配置

### Master 配置

编辑 `master/config.yaml`:

```yaml
server:
  grpc_port: 50051      # gRPC 端口
  ws_port: 8080         # Web 界面端口

auth:
  mode: api_key         # 认证模式
  api_keys:
    - "your-api-key"    # 使用强随机密钥

concurrency:
  global_max: 100       # 全局最大并发数
  agent_default_max: 5  # 每个 Agent 默认并发数
```

### Agent 配置

编辑 `agent/config.yaml`:

```yaml
agent:
  id: "docker-agent"              # Agent ID（唯一）
  name: "Docker Agent"            # 显示名称
  ipv4: "127.0.0.1"              # 公网 IPv4
  max_concurrent: 5

  metadata:
    location: "Docker Container"  # 位置
    provider: "Local"             # 服务商
    idc: "localhost"              # 数据中心
    description: "Local Docker Agent"

master:
  host: "master:50051"            # Master 地址（Docker 服务名）
  api_key: "your-api-key"         # 与 Master 一致
```

## 生产部署

### 跨主机部署

如果需要在不同服务器上运行 Master 和 Agent：

**Master 服务器**:

```bash
# 只启动 Master
docker-compose up -d master
```

**Agent 服务器**:

1. 修改 `agent/config.yaml`:
   ```yaml
   master:
     host: "master.example.com:50051"  # 改为 Master 的公网地址
   ```

2. 启动 Agent:
   ```bash
   docker-compose up -d agent
   ```

### 使用 Docker Swarm

```bash
# 初始化 Swarm
docker swarm init

# 部署服务栈
docker stack deploy -c docker-compose.yml lookingglass

# 查看服务
docker stack services lookingglass

# 查看日志
docker service logs -f lookingglass_master
docker service logs -f lookingglass_agent
```

### 使用 Kubernetes

详见 `k8s/` 目录中的配置文件（待添加）。

## 故障排查

### Agent 无法连接 Master

```bash
# 进入 Agent 容器测试连接
docker-compose exec agent ping master
docker-compose exec agent nc -zv master 50051

# 检查 Master 日志
docker-compose logs master | grep -i error
```

### Ping 命令失败

Agent 需要 `NET_RAW` 权限：

```bash
# 检查权限
docker-compose exec agent ping -c 1 8.8.8.8

# 如果失败，确保 docker-compose.yml 中有:
# cap_add:
#   - NET_RAW
#   - NET_ADMIN
```

### 端口冲突

如果 8080 或 50051 端口被占用：

1. 修改 `docker-compose.yml`:
   ```yaml
   ports:
     - "8081:8080"   # 使用不同的主机端口
     - "50052:50051"
   ```

2. 重启服务:
   ```bash
   docker-compose down
   docker-compose up -d
   ```

## 性能优化

### 资源限制

在 `docker-compose.yml` 中添加:

```yaml
services:
  master:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G

  agent:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
```

### 日志管理

限制日志大小:

```yaml
services:
  master:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## 进程管理 (Supervisor)

容器内使用 Supervisor 管理进程，提供自动重启、日志管理等功能。

### 查看进程状态

```bash
# Master 容器
docker exec lookingglass-master supervisorctl status

# Agent 容器
docker exec lookingglass-agent supervisorctl status
```

### 重启进程（不重启容器）

```bash
# 重启 Master 进程
docker exec lookingglass-master supervisorctl restart master

# 重启 Agent 进程
docker exec lookingglass-agent supervisorctl restart agent
```

### 查看进程日志

```bash
# 实时查看 Master 输出
docker exec -it lookingglass-master supervisorctl tail -f master stdout

# 查看错误日志
docker exec -it lookingglass-master supervisorctl tail -f master stderr

# 或者直接查看日志文件
docker exec lookingglass-master tail -f /app/logs/master-output.log
docker exec lookingglass-master tail -f /app/logs/master-error.log
```

### 日志文件位置

容器内日志结构：
```
/app/logs/
├── supervisord.log        # Supervisor 自身日志
├── master-output.log      # Master 标准输出
├── master-error.log       # Master 标准错误
├── agent-output.log       # Agent 标准输出
└── agent-error.log        # Agent 标准错误
```

**详细说明**: 查看 [docker/README.md](../docker/README.md)

## 安全建议

1. **更改默认 API Key**: 使用强随机密钥
2. **限制网络访问**: 只暴露必要端口
3. **使用 TLS**: 生产环境启用 gRPC TLS
4. **定期更新**: 及时更新 Docker 镜像
5. **资源限制**: 防止资源耗尽

## 下一步

- 阅读完整部署文档: [DEPLOYMENT.md](DEPLOYMENT.md)
- 了解架构设计: [CLAUDE.md](../CLAUDE.md)
- 查看 Supervisor 配置: [docker/README.md](../docker/README.md)
- 查看 Web 前端说明: [../web/README.md](../web/README.md)

## 获取帮助

- GitHub Issues: https://github.com/your-org/lookingglass/issues
- 文档: https://docs.lookingglass.example.com
