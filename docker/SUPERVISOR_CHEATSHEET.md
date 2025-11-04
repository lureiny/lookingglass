# Supervisor 快速参考

LookingGlass Docker 容器使用 Supervisor 管理进程的常用命令速查表。

## Make 命令（推荐）

### 查看进程状态
```bash
make docker-status
```

### 重启进程（不重启容器）
```bash
make docker-restart-master    # 重启 Master 进程
make docker-restart-agent     # 重启 Agent 进程
```

### 查看实时日志
```bash
make docker-logs-master       # Master 日志
make docker-logs-agent        # Agent 日志
```

### 进入容器 Shell
```bash
make docker-shell-master      # Master 容器
make docker-shell-agent       # Agent 容器
```

## Docker 原生命令

### 查看进程状态

```bash
# Master 容器
docker exec lookingglass-master supervisorctl status

# Agent 容器
docker exec lookingglass-agent supervisorctl status
```

**输出示例**:
```
master                           RUNNING   pid 12, uptime 1:23:45
```

**状态说明**:
- `RUNNING` - 正常运行
- `STOPPED` - 已停止
- `STARTING` - 正在启动
- `BACKOFF` - 启动失败，正在重试
- `FATAL` - 启动失败次数过多，放弃重试
- `EXITED` - 已退出

### 启动/停止/重启进程

```bash
# 启动进程
docker exec lookingglass-master supervisorctl start master

# 停止进程
docker exec lookingglass-master supervisorctl stop master

# 重启进程
docker exec lookingglass-master supervisorctl restart master

# 重启所有进程
docker exec lookingglass-master supervisorctl restart all
```

### 查看日志

#### 实时日志 (推荐)

```bash
# 查看标准输出
docker exec -it lookingglass-master supervisorctl tail -f master stdout

# 查看标准错误
docker exec -it lookingglass-master supervisorctl tail -f master stderr

# 查看最后 N 行
docker exec lookingglass-master supervisorctl tail master stdout 100
```

#### 直接查看日志文件

```bash
# Master 日志
docker exec lookingglass-master tail -f /app/logs/master-output.log
docker exec lookingglass-master tail -f /app/logs/master-error.log
docker exec lookingglass-master tail -f /app/logs/supervisord.log

# Agent 日志
docker exec lookingglass-agent tail -f /app/logs/agent-output.log
docker exec lookingglass-agent tail -f /app/logs/agent-error.log
```

### 重新加载配置

如果修改了 supervisord.conf：

```bash
# 1. 重新读取配置文件
docker exec lookingglass-master supervisorctl reread

# 2. 应用配置变更
docker exec lookingglass-master supervisorctl update

# 3. 重启受影响的进程
docker exec lookingglass-master supervisorctl restart master
```

### 清空日志

```bash
# 清空 Master 日志
docker exec lookingglass-master supervisorctl clear master

# 清空所有日志
docker exec lookingglass-master supervisorctl clear all
```

## Docker Compose 命令

如果使用 docker-compose：

```bash
# 查看状态
docker-compose exec master supervisorctl status
docker-compose exec agent supervisorctl status

# 重启进程
docker-compose exec master supervisorctl restart master
docker-compose exec agent supervisorctl restart agent

# 查看日志
docker-compose exec -T master supervisorctl tail master stdout
docker-compose exec -T agent supervisorctl tail agent stdout

# 进入容器
docker-compose exec master sh
docker-compose exec agent sh
```

## 常见任务

### 1. 修改配置后重启服务

```bash
# 方法 1: 只重启进程（推荐，快速）
docker exec lookingglass-master supervisorctl restart master

# 方法 2: 重启整个容器（较慢）
docker restart lookingglass-master
```

### 2. 查看进程运行了多久

```bash
docker exec lookingglass-master supervisorctl status
# 输出: master    RUNNING   pid 12, uptime 1:23:45
#                                         ^^^^^^^^ 运行时间
```

### 3. 检查进程是否异常退出

```bash
# 查看 Supervisor 日志
docker exec lookingglass-master cat /app/logs/supervisord.log | grep -i error

# 查看进程错误日志
docker exec lookingglass-master cat /app/logs/master-error.log
```

### 4. 临时关闭自动重启

```bash
# 停止进程并禁用自动重启
docker exec lookingglass-master supervisorctl stop master

# 再次启动
docker exec lookingglass-master supervisorctl start master
```

### 5. 监控进程资源使用

```bash
# 进入容器
docker exec -it lookingglass-master sh

# 查看进程资源使用
top
ps aux | grep master

# 或从外部查看
docker stats lookingglass-master
```

### 6. 导出日志

```bash
# 导出 Master 日志到本地
docker cp lookingglass-master:/app/logs/master-output.log ./master-$(date +%Y%m%d).log

# 导出所有日志
docker cp lookingglass-master:/app/logs/ ./logs-backup/
```

## 故障排查流程

### 问题: 容器启动但服务不可用

1. **检查容器状态**
   ```bash
   docker ps -a | grep lookingglass
   ```

2. **检查进程状态**
   ```bash
   docker exec lookingglass-master supervisorctl status
   ```

3. **查看错误日志**
   ```bash
   docker exec lookingglass-master cat /app/logs/master-error.log
   docker exec lookingglass-master cat /app/logs/supervisord.log
   ```

4. **检查配置文件**
   ```bash
   docker exec lookingglass-master cat /app/config.yaml
   ```

### 问题: 进程频繁重启 (BACKOFF)

1. **查看详细状态**
   ```bash
   docker exec lookingglass-master supervisorctl status
   # 如果看到 BACKOFF 或 FATAL
   ```

2. **查看启动日志**
   ```bash
   docker exec lookingglass-master supervisorctl tail master stderr 100
   ```

3. **手动尝试启动**
   ```bash
   docker exec -it lookingglass-master sh
   /app/master -config /app/config.yaml
   # 查看直接输出的错误信息
   ```

### 问题: 进程僵死 (STOPPED 但应该 RUNNING)

1. **尝试启动**
   ```bash
   docker exec lookingglass-master supervisorctl start master
   ```

2. **如果启动失败，查看原因**
   ```bash
   docker exec lookingglass-master supervisorctl tail master stderr
   ```

3. **强制重启**
   ```bash
   docker exec lookingglass-master supervisorctl restart master
   ```

## 高级操作

### 修改 Supervisor 配置

```bash
# 1. 进入容器
docker exec -it lookingglass-master sh

# 2. 编辑配置（需要安装 vi 或 nano）
vi /etc/supervisord.conf

# 3. 重新加载
supervisorctl reread
supervisorctl update

# 4. 重启进程
supervisorctl restart master
```

### 添加自定义监控脚本

在 supervisord.conf 中添加：

```ini
[program:healthcheck]
command=/app/scripts/healthcheck.sh
autostart=true
autorestart=true
priority=999
```

### 配置邮件告警

在 supervisord.conf 中添加：

```ini
[eventlistener:crashmail]
command=/usr/local/bin/crashmail -a -m your@email.com
events=PROCESS_STATE_EXITED
```

## 性能调优

### 日志轮转配置

在 supervisord.conf 中调整：

```ini
stdout_logfile_maxbytes=50MB    # 增加日志文件大小
stdout_logfile_backups=10       # 保留更多备份
```

### 进程优先级

```ini
priority=1      # 数字越小越先启动
```

### 资源限制 (需要 supervisor 3.4+)

```ini
[program:master]
rlimit_nofile=65536    # 最大打开文件数
```

## 参考资料

- Supervisor 官方文档: http://supervisord.org/
- Docker 文档: https://docs.docker.com/
- LookingGlass 文档: [../docs/](../docs/)

## 快速参考卡片

| 操作 | 命令 |
|------|------|
| 查看状态 | `supervisorctl status` |
| 启动进程 | `supervisorctl start <name>` |
| 停止进程 | `supervisorctl stop <name>` |
| 重启进程 | `supervisorctl restart <name>` |
| 查看日志 | `supervisorctl tail -f <name> stdout` |
| 清空日志 | `supervisorctl clear <name>` |
| 重载配置 | `supervisorctl reread && update` |
| 进入容器 | `docker exec -it <container> sh` |
