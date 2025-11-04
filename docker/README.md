# Docker Supervisor 配置说明

本目录包含使用 Supervisor 管理 LookingGlass 容器内进程的配置文件。

## 为什么使用 Supervisor？

在 Docker 容器中使用 Supervisor 的优势：

1. **进程管理** - 自动重启失败的进程
2. **日志管理** - 统一的日志收集和轮转
3. **优雅关闭** - 正确处理进程终止信号
4. **可扩展性** - 可在同一容器中运行多个进程
5. **监控能力** - 通过 supervisorctl 管理和监控进程状态

## 配置文件

### supervisord-master.conf

Master 容器的 Supervisor 配置：

```ini
[program:master]
command=/app/master -config /app/config.yaml
user=lookingglass
autostart=true
autorestart=true
```

**关键配置项**:
- `autostart=true` - 容器启动时自动启动 master 进程
- `autorestart=true` - 进程异常退出时自动重启
- `startretries=3` - 最多重试 3 次
- `user=lookingglass` - 以非 root 用户运行（安全）

### supervisord-agent.conf

Agent 容器的 Supervisor 配置：

```ini
[program:agent]
command=/app/agent -config /app/config.yaml
user=lookingglass
autostart=true
autorestart=true
```

## 使用方法

### 构建镜像

```bash
# 构建 Master 镜像
docker build -f Dockerfile.master -t lookingglass-master:latest .

# 构建 Agent 镜像
docker build -f Dockerfile.agent -t lookingglass-agent:latest .
```

### 运行容器

```bash
# 运行 Master 容器
docker run -d \
  --name lookingglass-master \
  -p 50051:50051 \
  -p 8080:8080 \
  -v $(pwd)/master/config.yaml:/app/config.yaml:ro \
  lookingglass-master:latest

# 运行 Agent 容器
docker run -d \
  --name lookingglass-agent \
  --cap-add=NET_RAW \
  --cap-add=NET_ADMIN \
  -v $(pwd)/agent/config.yaml:/app/config.yaml:ro \
  lookingglass-agent:latest
```

## 进程管理

### 进入容器查看进程状态

```bash
# Master 容器
docker exec -it lookingglass-master supervisorctl status

# Agent 容器
docker exec -it lookingglass-agent supervisorctl status
```

预期输出：
```
master                           RUNNING   pid 12, uptime 0:05:23
```

### 手动重启进程

```bash
# 重启 Master 进程（不重启容器）
docker exec lookingglass-master supervisorctl restart master

# 重启 Agent 进程
docker exec lookingglass-agent supervisorctl restart agent
```

### 停止/启动进程

```bash
# 停止进程
docker exec lookingglass-master supervisorctl stop master

# 启动进程
docker exec lookingglass-master supervisorctl start master
```

### 查看进程日志

Supervisor 管理的日志文件：

```bash
# 查看 Master 进程日志
docker exec lookingglass-master tail -f /app/logs/master-output.log
docker exec lookingglass-master tail -f /app/logs/master-error.log

# 查看 Supervisor 自身日志
docker exec lookingglass-master tail -f /app/logs/supervisord.log
```

或使用 supervisorctl：

```bash
# 实时查看进程输出
docker exec -it lookingglass-master supervisorctl tail -f master stdout

# 查看进程错误输出
docker exec -it lookingglass-master supervisorctl tail -f master stderr
```

## 日志管理

### 日志文件位置

容器内的日志文件结构：

```
/app/logs/
├── supervisord.log        # Supervisor 自身日志
├── master-output.log      # Master 进程标准输出
├── master-error.log       # Master 进程标准错误
├── agent-output.log       # Agent 进程标准输出
└── agent-error.log        # Agent 进程标准错误
```

### 日志轮转

Supervisor 自动进行日志轮转：
- 每个日志文件最大 10MB
- 达到限制后自动轮转，保留旧日志

### 持久化日志

使用 Docker volume 持久化日志：

```bash
# 创建 volume
docker volume create master-logs
docker volume create agent-logs

# 运行时挂载
docker run -d \
  -v master-logs:/app/logs \
  lookingglass-master:latest

# 查看持久化日志
docker run --rm -v master-logs:/logs alpine cat /logs/master-output.log
```

## 故障排查

### 进程无法启动

1. 查看 Supervisor 日志：
   ```bash
   docker exec lookingglass-master cat /app/logs/supervisord.log
   ```

2. 查看进程错误日志：
   ```bash
   docker exec lookingglass-master cat /app/logs/master-error.log
   ```

3. 检查配置文件：
   ```bash
   docker exec lookingglass-master cat /app/config.yaml
   ```

### 进程频繁重启

查看重启次数和原因：

```bash
docker exec lookingglass-master supervisorctl status
```

如果看到 `BACKOFF` 或 `FATAL` 状态，检查错误日志。

### 修改 Supervisor 配置

如果需要临时修改配置（不推荐生产环境）：

```bash
# 进入容器
docker exec -it lookingglass-master sh

# 编辑配置
vi /etc/supervisord.conf

# 重新加载配置
supervisorctl reread
supervisorctl update
```

## 高级配置

### 添加额外进程

如果需要在同一容器中运行额外的辅助进程（如监控脚本），可以在 supervisord.conf 中添加：

```ini
[program:monitor]
command=/app/scripts/monitor.sh
directory=/app
user=lookingglass
autostart=true
autorestart=true
priority=10
```

### 进程依赖关系

如果某个进程依赖于另一个进程：

```ini
[program:dependent]
command=/app/dependent-process
autostart=true
autorestart=true
priority=999  # 较高的优先级，后启动
```

`priority` 值越小越先启动。

### 环境变量

在 Supervisor 配置中传递环境变量：

```ini
[program:master]
command=/app/master -config /app/config.yaml
environment=LOG_LEVEL="debug",TIMEZONE="Asia/Shanghai"
```

## Docker Compose 集成

在 `docker-compose.yml` 中使用 Supervisor：

```yaml
version: '3.8'

services:
  master:
    build:
      context: .
      dockerfile: Dockerfile.master
    ports:
      - "50051:50051"
      - "8080:8080"
    volumes:
      - ./master/config.yaml:/app/config.yaml:ro
      - master-logs:/app/logs
    restart: unless-stopped

  agent:
    build:
      context: .
      dockerfile: Dockerfile.agent
    cap_add:
      - NET_RAW
      - NET_ADMIN
    volumes:
      - ./agent/config.yaml:/app/config.yaml:ro
      - agent-logs:/app/logs
    restart: unless-stopped

volumes:
  master-logs:
  agent-logs:
```

使用 supervisorctl 查看状态：

```bash
docker-compose exec master supervisorctl status
docker-compose exec agent supervisorctl status
```

## 与原始 Dockerfile 的区别

### 原始方式（直接运行进程）

```dockerfile
ENTRYPOINT ["/app/master"]
CMD ["-config", "/app/config.yaml"]
```

**优点**:
- 简单直接
- 容器 PID 1 即应用进程

**缺点**:
- 进程崩溃后容器退出
- 无法在同一容器运行多个进程
- 日志管理不够灵活

### Supervisor 方式（推荐）

```dockerfile
ENTRYPOINT ["/usr/bin/supervisord"]
CMD ["-c", "/etc/supervisord.conf"]
```

**优点**:
- 进程自动重启（不重启容器）
- 统一的日志管理和轮转
- 可运行多个进程
- 更好的监控和控制能力

**缺点**:
- 稍微增加镜像大小（约 10MB）
- 多一层抽象

## 生产环境建议

1. **使用 Volume 持久化日志**：
   ```bash
   docker run -v /var/log/lookingglass/master:/app/logs ...
   ```

2. **配置日志收集**：集成 ELK、Loki 等日志系统

3. **监控 Supervisor 状态**：
   ```bash
   # 定期检查
   docker exec lookingglass-master supervisorctl status
   ```

4. **告警配置**：监控进程 `FATAL` 状态并发送告警

5. **资源限制**：在 supervisord.conf 中配置资源限制（需要 supervisor 3.4+）

## 参考资料

- [Supervisor 官方文档](http://supervisord.org/)
- [Docker 最佳实践](https://docs.docker.com/develop/dev-best-practices/)
- [LookingGlass 部署指南](../docs/DEPLOYMENT.md)
