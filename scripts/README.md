# LookingGlass 自动化脚本

本目录包含用于自动化部署和管理 LookingGlass 的脚本。

## 脚本列表

### 1. install.sh - 自动安装脚本

无需 Docker，自动部署 Master 和 Agent，使用 Supervisor 管理进程。

**功能特性**:
- ✅ 自动生成随机 API Key（32 字符）
- ✅ 自动检测公网 IP 地址
- ✅ 自动生成唯一 Agent ID
- ✅ 自动安装系统依赖（supervisor, curl, wget）
- ✅ 自动安装诊断工具（ping, mtr, nexttrace）
- ✅ 生成完整的配置文件
- ✅ 配置 Supervisor 自动重启
- ✅ 创建专用用户运行服务
- ✅ 设置正确的文件权限

**使用方法**:

```bash
# 安装 Master
sudo ./scripts/install.sh master

# 安装 Agent（会提示输入 Master 地址和 API Key）
sudo ./scripts/install.sh agent

# 同时安装 Master 和 Agent（本地测试）
sudo ./scripts/install.sh all

# 跳过依赖安装（已安装过）
sudo ./scripts/install.sh master --skip-deps

# 只安装但不启动
sudo ./scripts/install.sh master --no-start
```

**安装后的目录结构**:

```
/opt/lookingglass/
├── master/
│   ├── master              # 二进制文件
│   ├── config.yaml         # 配置文件
│   ├── .api_key            # API Key (保密)
│   ├── logs/               # 日志目录
│   │   ├── master-output.log
│   │   ├── master-error.log
│   │   └── supervisord.log
│   └── web/                # 前端文件
└── agent/
    ├── agent               # 二进制文件
    ├── config.yaml         # 配置文件
    └── logs/               # 日志目录
        ├── agent-output.log
        ├── agent-error.log
        └── supervisord.log
```

**Supervisor 配置位置**:
- Master: `/etc/supervisor/conf.d/lookingglass-master.conf`
- Agent: `/etc/supervisor/conf.d/lookingglass-agent.conf`

---

### 2. manage.sh - 日常管理脚本

用于日常维护和管理服务。

**功能特性**:
- 📊 查看服务状态
- 🚀 启动/停止/重启服务
- 📝 查看日志（普通/实时）
- ⚙️ 查看/编辑配置
- 🔑 显示 API Key
- 💊 健康检查

**使用方法**:

```bash
# 查看所有服务状态
./scripts/manage.sh status

# 启动服务
./scripts/manage.sh start master      # 启动 Master
./scripts/manage.sh start agent       # 启动 Agent
./scripts/manage.sh start             # 启动所有

# 停止服务
./scripts/manage.sh stop master
./scripts/manage.sh stop agent
./scripts/manage.sh stop              # 停止所有

# 重启服务
./scripts/manage.sh restart master
./scripts/manage.sh restart agent
./scripts/manage.sh restart           # 重启所有

# 查看日志
./scripts/manage.sh logs master       # 最近 50 行
./scripts/manage.sh logs-f master     # 实时日志
./scripts/manage.sh error master      # 错误日志
./scripts/manage.sh error-f master    # 实时错误日志

# 配置管理
./scripts/manage.sh config master     # 查看配置
./scripts/manage.sh edit master       # 编辑配置（会提示是否重启）

# 显示 API Key
./scripts/manage.sh apikey

# 健康检查
./scripts/manage.sh health
```

**输出示例**:

```bash
$ ./scripts/manage.sh status

==========================================
LookingGlass 服务状态
==========================================

[Master]
  ● lookingglass-master    RUNNING   pid 1234, uptime 1:23:45

  端口监听:
    0.0.0.0:50051 -> 1234/master
    0.0.0.0:8080 -> 1234/master

[Agent]
  ● lookingglass-agent     RUNNING   pid 5678, uptime 1:23:40

==========================================
```

---

### 3. uninstall.sh - 卸载脚本

完全卸载 LookingGlass。

**功能特性**:
- 🗑️ 卸载指定组件或全部
- 💾 可选择保留配置和日志
- 🧹 完全清理模式
- 👤 可选择删除用户

**使用方法**:

```bash
# 卸载 Master（会询问是否保留数据）
sudo ./scripts/uninstall.sh master

# 卸载 Agent
sudo ./scripts/uninstall.sh agent

# 卸载所有组件
sudo ./scripts/uninstall.sh all

# 完全清理（包括用户）
sudo ./scripts/uninstall.sh purge
```

**数据备份**:

如果选择保留数据，会自动备份到：
```
/tmp/lookingglass-backup-YYYYMMDD_HHMMSS/
```

---

## 完整部署流程

### 场景 1: 单机部署（测试环境）

在一台机器上同时运行 Master 和 Agent：

```bash
# 1. 编译项目
make build

# 2. 安装 Master 和 Agent
sudo ./scripts/install.sh all

# 3. 查看状态
./scripts/manage.sh status

# 4. 访问 Web 界面
# 打开浏览器: http://localhost:8080

# 5. 查看日志
./scripts/manage.sh logs-f master
```

### 场景 2: 分布式部署（生产环境）

**在 Master 服务器上**:

```bash
# 1. 编译 Master
make build-master

# 2. 安装 Master
sudo ./scripts/install.sh master

# 3. 获取 API Key
./scripts/manage.sh apikey
# 输出: abc123...xyz (保存此 Key)

# 4. 查看状态
./scripts/manage.sh status

# 5. 确认服务正常
./scripts/manage.sh health
```

**在 Agent 服务器上**:

```bash
# 1. 编译 Agent
make build-agent

# 2. 安装 Agent
sudo ./scripts/install.sh agent

# 输入提示信息:
# Master 地址: 192.168.1.100:50051
# API Key: abc123...xyz (来自 Master)

# 3. 查看状态
./scripts/manage.sh status

# 4. 查看日志确认连接
./scripts/manage.sh logs agent
```

### 场景 3: 多 Agent 部署

在多台服务器上部署 Agent：

```bash
# 在每台 Agent 服务器上执行
sudo ./scripts/install.sh agent

# 使用相同的 Master 地址和 API Key
# 每个 Agent 会自动生成唯一的 Agent ID
```

---

## 常见任务

### 修改配置

```bash
# 1. 编辑配置
./scripts/manage.sh edit master

# 2. 自动提示重启
# 或手动重启
./scripts/manage.sh restart master
```

### 查看实时日志

```bash
# Master 日志
./scripts/manage.sh logs-f master

# Agent 日志
./scripts/manage.sh logs-f agent

# 按 Ctrl+C 退出
```

### 更新二进制文件

```bash
# 1. 编译新版本
make build

# 2. 停止服务
./scripts/manage.sh stop master

# 3. 备份旧版本（可选）
sudo cp /opt/lookingglass/master/master /opt/lookingglass/master/master.bak

# 4. 复制新版本
sudo cp bin/master /opt/lookingglass/master/

# 5. 启动服务
./scripts/manage.sh start master

# 6. 检查状态
./scripts/manage.sh health
```

### 迁移到其他服务器

**导出配置**:

```bash
# 备份配置和 API Key
sudo tar czf lookingglass-backup.tar.gz \
  /opt/lookingglass/master/config.yaml \
  /opt/lookingglass/master/.api_key \
  /opt/lookingglass/agent/config.yaml
```

**导入配置**:

```bash
# 1. 安装（不启动）
sudo ./scripts/install.sh master --no-start

# 2. 恢复配置
sudo tar xzf lookingglass-backup.tar.gz -C /

# 3. 启动服务
./scripts/manage.sh start master
```

### 故障排查

```bash
# 1. 查看服务状态
./scripts/manage.sh status

# 2. 健康检查
./scripts/manage.sh health

# 3. 查看错误日志
./scripts/manage.sh error master
./scripts/manage.sh error agent

# 4. 查看 Supervisor 日志
sudo tail -f /opt/lookingglass/master/logs/supervisord.log

# 5. 手动测试连接
# Master 到 Agent
telnet <master_ip> 50051

# Agent 到 Master
telnet <master_ip> 50051
```

---

## 系统要求

### Master 服务器

- **操作系统**: Linux (Ubuntu 20.04+, CentOS 8+, Debian 11+)
- **CPU**: 1 核心（推荐 2 核心）
- **内存**: 512MB（推荐 1GB+）
- **磁盘**: 1GB 可用空间
- **端口**: 50051 (gRPC), 8080 (HTTP/WebSocket)

### Agent 服务器

- **操作系统**: Linux (Ubuntu 20.04+, CentOS 8+, Debian 11+)
- **CPU**: 1 核心
- **内存**: 256MB（推荐 512MB）
- **磁盘**: 100MB 可用空间
- **网络**: 能够访问 Master 的 50051 端口

### 依赖软件

脚本会自动安装以下依赖：

**必需**:
- supervisor
- curl
- wget

**诊断工具**（仅 Agent）:
- ping (iputils)
- mtr
- nexttrace（自动下载安装）

---

## 安全建议

1. **API Key 安全**:
   - 生成的 API Key 保存在 `/opt/lookingglass/master/.api_key`
   - 文件权限为 600，仅 lookingglass 用户可读
   - 定期轮换 API Key

2. **网络安全**:
   - 使用防火墙限制端口访问
   - 生产环境建议启用 TLS
   - 限制 Web 界面访问（如使用 nginx 反向代理 + 认证）

3. **系统安全**:
   - 使用专用的 lookingglass 用户运行服务
   - 定期更新系统和依赖包
   - 监控日志文件

4. **备份**:
   - 定期备份配置文件
   - 保存 API Key
   - 备份重要日志

---

## 故障排查

### 服务无法启动

**问题**: `supervisorctl status` 显示 FATAL 或 BACKOFF

**排查步骤**:

1. 查看错误日志:
   ```bash
   ./scripts/manage.sh error master
   ```

2. 检查配置文件:
   ```bash
   ./scripts/manage.sh config master
   ```

3. 手动运行查看详细错误:
   ```bash
   sudo -u lookingglass /opt/lookingglass/master/master -config /opt/lookingglass/master/config.yaml
   ```

4. 检查端口占用:
   ```bash
   sudo netstat -tlnp | grep -E ":(50051|8080)"
   ```

### Agent 无法连接 Master

**排查步骤**:

1. 检查网络连通性:
   ```bash
   telnet <master_ip> 50051
   # 或
   nc -zv <master_ip> 50051
   ```

2. 检查 API Key 是否正确:
   ```bash
   # Master 上
   ./scripts/manage.sh apikey

   # Agent 上
   grep api_key /opt/lookingglass/agent/config.yaml
   ```

3. 查看 Agent 日志:
   ```bash
   ./scripts/manage.sh logs agent | grep -i error
   ```

4. 检查防火墙:
   ```bash
   # Master 上
   sudo ufw status
   sudo iptables -L -n | grep 50051
   ```

### Web 界面无法访问

**排查步骤**:

1. 检查 Master 是否运行:
   ```bash
   ./scripts/manage.sh status
   ```

2. 检查端口监听:
   ```bash
   sudo netstat -tlnp | grep 8080
   ```

3. 测试 API:
   ```bash
   curl http://localhost:8080/api/agents
   ```

4. 查看 Master 错误日志:
   ```bash
   ./scripts/manage.sh error master
   ```

---

## 日志管理

### 日志位置

```
/opt/lookingglass/
├── master/logs/
│   ├── master-output.log      # 标准输出
│   ├── master-error.log       # 标准错误
│   └── supervisord.log        # Supervisor 日志
└── agent/logs/
    ├── agent-output.log       # 标准输出
    ├── agent-error.log        # 标准错误
    └── supervisord.log        # Supervisor 日志
```

### 日志轮转

Supervisor 自动进行日志轮转：
- 单个日志文件最大 10MB
- 保留 5 个备份文件

### 查看日志

```bash
# 使用管理脚本（推荐）
./scripts/manage.sh logs master
./scripts/manage.sh logs-f master

# 直接查看文件
sudo tail -f /opt/lookingglass/master/logs/master-output.log

# 使用 supervisorctl
sudo supervisorctl tail -f lookingglass-master stdout
```

---

## 性能优化

### 并发配置

编辑配置文件调整并发限制：

```yaml
# Master: config.yaml
concurrency:
  global_max: 100              # 全局最大并发
  agent_default_max: 10        # 单 Agent 默认并发

# Agent: config.yaml
executor:
  global_concurrency: 10       # Agent 全局并发
  tasks:
    ping:
      concurrency:
        max: 5                 # Ping 任务并发
```

### 资源监控

```bash
# CPU 和内存使用
top -p $(pgrep -f lookingglass)

# 网络连接
sudo netstat -antp | grep -E "(master|agent)"

# 磁盘使用
du -sh /opt/lookingglass/*/logs/
```

---

## 相关文档

- [完整部署指南](../docs/DEPLOYMENT.md)
- [任务配置指南](../docs/TASK_CONFIG.md)
- [Docker 部署](../docs/DOCKER.md)
- [架构说明](../CLAUDE.md)

---

## 获取帮助

- GitHub Issues: https://github.com/lureiny/lookingglass/issues
- 项目主页: https://github.com/lureiny/lookingglass
