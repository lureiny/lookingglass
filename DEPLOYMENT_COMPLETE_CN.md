# 部署实现完成

## 概述

本文档总结了 LookingGlass 项目已完成的部署基础设施改进。

## 已实现的功能

### 1. Docker Supervisor 集成

**创建的文件:**
- [docker/supervisord-master.conf](docker/supervisord-master.conf) - Master 的 Supervisor 配置
- [docker/supervisord-agent.conf](docker/supervisord-agent.conf) - Agent 的 Supervisor 配置
- [docker/SUPERVISOR_CHEATSHEET.md](docker/SUPERVISOR_CHEATSHEET.md) - 快速参考指南

**更新的文件:**
- [Dockerfile.master](Dockerfile.master) - 现在使用 supervisor
- [Dockerfile.agent](Dockerfile.agent) - 现在使用 supervisor
- [Makefile](Makefile) - 添加了 supervisor 管理命令
- [docker/README.md](docker/README.md) - 添加了 supervisor 使用文档
- [docs/DOCKER.md](docs/DOCKER.md) - 添加了 supervisor 章节

**优势:**
- 进程失败时自动重启
- 统一的日志管理和轮转
- 可以在不重启容器的情况下重启进程
- 通过 supervisorctl 更好地监控
- 生产环境就绪的进程管理

**使用方法:**
```bash
# 使用 supervisor 构建
make docker-build-master
make docker-build-agent

# 管理容器内的进程
make docker-status           # 查看进程状态
make docker-restart-master   # 重启 master 进程
make docker-logs-master      # 查看实时日志
```

### 2. 自动化部署脚本

**创建的文件:**
- [scripts/install.sh](scripts/install.sh) (~700 行) - 自动化安装
- [scripts/manage.sh](scripts/manage.sh) (~500 行) - 服务管理
- [scripts/uninstall.sh](scripts/uninstall.sh) (~250 行) - 清理卸载
- [scripts/README.md](scripts/README.md) (~600 行) - 完整指南

**核心特性:**

**install.sh:**
- 自动生成 32 字符随机 API 密钥
- 自动检测公网 IP 地址（多个 API 备用）
- 自动生成唯一的 Agent ID
- 创建完整的配置文件
- 安装系统依赖（supervisor、工具）
- 安装诊断工具（ping、mtr、nexttrace）
- 设置专用的 lookingglass 用户
- 配置 supervisor 服务
- 设置正确的文件权限

**使用方法:**
```bash
# 安装 Master
sudo ./scripts/install.sh master

# 安装 Agent（会提示输入 Master 地址和 API key）
sudo ./scripts/install.sh agent

# 同时安装两者（用于本地测试）
sudo ./scripts/install.sh all

# 高级选项
sudo ./scripts/install.sh master --skip-deps  # 跳过依赖安装
sudo ./scripts/install.sh master --no-start   # 安装但不启动
```

**manage.sh:**
- 查看服务状态（带颜色编码的输出）
- 启动/停止/重启服务
- 查看日志（普通和实时）
- 查看错误日志
- 编辑配置并自动提示重启
- 安全显示 API 密钥
- 全面的健康检查（进程、端口、连接性）

**使用方法:**
```bash
./scripts/manage.sh status          # 查看所有服务
./scripts/manage.sh start master    # 启动 Master
./scripts/manage.sh logs-f agent    # 实时查看 Agent 日志
./scripts/manage.sh health          # 健康检查
./scripts/manage.sh edit master     # 编辑配置
./scripts/manage.sh apikey          # 显示 API 密钥
```

**uninstall.sh:**
- 停止所有服务
- 可选的数据备份到 /tmp/lookingglass-backup-*
- 删除安装目录
- 可选的用户删除
- 清理 supervisor 配置

**使用方法:**
```bash
sudo ./scripts/uninstall.sh master  # 卸载 Master
sudo ./scripts/uninstall.sh agent   # 卸载 Agent
sudo ./scripts/uninstall.sh all     # 卸载所有组件
sudo ./scripts/uninstall.sh purge   # 完全清理，包括用户
```

### 3. 配置文件

**创建的文件:**
- [agent/config.yaml.example](agent/config.yaml.example) (~240 行) - 完整的 agent 配置模板

**更新的文件:**
- [master/config.yaml.example](master/config.yaml.example) (~200 行) - 修正为匹配实际代码

**重要修复:**
master 配置文件已修正，删除了实际代码中不存在的无效参数：
- ❌ 删除: `http_enabled`（不存在）
- ❌ 删除: `api_keys` 数组 → ✅ 改为: `api_key`（字符串）
- ❌ 删除: `web` 章节（不存在）
- ❌ 删除: `performance` 章节（不存在）
- ❌ 删除: 日志轮转字段（max_size、max_backups 等）
- ✅ 修复: `heartbeat` 移到 `agent` 章节下
- ❌ 删除: `task.max_timeout`、`task.default_nexttrace_hops`（不存在）

**验证过程:**
所有配置参数都已对照以下文件中的实际 Go 结构体定义进行验证：
- [master/config/config.go](master/config/config.go)
- [agent/config/config.go](agent/config/config.go)

### 4. 文档更新

**更新的文件:**
- [README.md](README.md) - 添加脚本部署为推荐方法
- [docs/DOCKER.md](docs/DOCKER.md) - 添加 supervisor 管理章节
- [docker/README.md](docker/README.md) - 完整的 Docker 指南，包含 supervisor

## 安装目录结构

安装后，会创建以下目录结构：

```
/opt/lookingglass/
├── master/
│   ├── master                  # 二进制文件
│   ├── config.yaml             # 配置文件
│   ├── .api_key                # API Key（600 权限）
│   ├── logs/                   # 日志目录
│   │   ├── master-output.log
│   │   ├── master-error.log
│   │   └── supervisord.log
│   └── web/                    # 前端文件
└── agent/
    ├── agent                   # 二进制文件
    ├── config.yaml             # 配置文件
    └── logs/                   # 日志目录
        ├── agent-output.log
        ├── agent-error.log
        └── supervisord.log
```

Supervisor 配置文件位置：
- Master: `/etc/supervisor/conf.d/lookingglass-master.conf`
- Agent: `/etc/supervisor/conf.d/lookingglass-agent.conf`

## 安全特性

1. **专用用户**: 服务以 `lookingglass` 用户运行（非 root）
2. **文件权限**: 配置文件 644，API 密钥文件 600
3. **随机 API 密钥**: 通过 openssl 生成 32 字符十六进制密钥
4. **IP 白名单支持**: 可选的额外安全层
5. **进程隔离**: Supervisor 以 root 运行，但以 lookingglass 用户身份启动服务

## 系统要求

### Master 服务器
- Linux（Ubuntu 20.04+、CentOS 8+、Debian 11+）
- CPU: 1 核心（推荐 2 核心）
- 内存: 512MB（推荐 1GB+）
- 磁盘: 1GB 可用空间
- 端口: 50051（gRPC）、8080（HTTP/WebSocket）

### Agent 服务器
- Linux（Ubuntu 20.04+、CentOS 8+、Debian 11+）
- CPU: 1 核心
- 内存: 256MB（推荐 512MB）
- 磁盘: 100MB 可用空间
- 网络: 能访问 Master 的 50051 端口

## 部署场景

### 场景 1: 单机部署（测试环境）
```bash
make build
sudo ./scripts/install.sh all
./scripts/manage.sh status
# 访问: http://localhost:8080
```

### 场景 2: 分布式部署（生产环境）

**Master 服务器:**
```bash
make build-master
sudo ./scripts/install.sh master
./scripts/manage.sh apikey  # 保存这个密钥
./scripts/manage.sh health
```

**Agent 服务器:**
```bash
make build-agent
sudo ./scripts/install.sh agent
# 提示时输入 Master 地址和 API 密钥
./scripts/manage.sh status
```

### 场景 3: Docker + Supervisor
```bash
make docker-build-master
make docker-build-agent
make docker-up
make docker-status
```

## 常见任务

### 查看实时日志
```bash
./scripts/manage.sh logs-f master
./scripts/manage.sh logs-f agent
```

### 更新二进制文件
```bash
make build
./scripts/manage.sh stop master
sudo cp bin/master /opt/lookingglass/master/
./scripts/manage.sh start master
```

### 修改配置
```bash
./scripts/manage.sh edit master
# 脚本会提示是否重启
```

### 故障排查
```bash
./scripts/manage.sh health           # 全面的健康检查
./scripts/manage.sh error master     # 查看错误日志
sudo supervisorctl status            # 查看所有 supervisor 服务
```

## 日志

**位置:**
- Master 输出: `/opt/lookingglass/master/logs/master-output.log`
- Master 错误: `/opt/lookingglass/master/logs/master-error.log`
- Agent 输出: `/opt/lookingglass/agent/logs/agent-output.log`
- Agent 错误: `/opt/lookingglass/agent/logs/agent-error.log`

**轮转:**
- 通过 supervisor 自动轮转
- 最大大小: 每个文件 10MB
- 备份: 保留 5 个文件

## 关键技术决策

1. **Supervisor 而非 systemd**: 更好的跨平台支持，Docker 和裸机部署统一管理
2. **自动配置生成**: 降低部署复杂度和人为错误
3. **多 API 备用**: IP 检测在不同网络环境下更可靠
4. **专用用户**: 安全最佳实践，进程隔离
5. **配置验证**: 所有示例配置都已对照实际 Go 结构体验证
6. **全面的脚本**: 覆盖完整生命周期（安装、管理、卸载）

## 测试清单

- [x] Docker 使用 supervisor 成功构建
- [x] Supervisor 启动并管理进程
- [x] 安装脚本生成有效配置
- [x] API 密钥生成正常工作
- [x] IP 自动检测工作（已测试多个 API）
- [x] Agent 配置示例已创建
- [x] Master 配置示例已对照代码验证
- [x] 所有无效配置参数已删除
- [x] Supervisor 日志轮转正常工作
- [x] 健康检查脚本正常工作
- [x] 卸载脚本正确清理

## 用户后续步骤

1. **快速开始:**
   ```bash
   make build
   sudo ./scripts/install.sh all
   ./scripts/manage.sh status
   ```

2. **生产环境部署:**
   - 查看 [scripts/README.md](scripts/README.md)
   - 按照分布式部署场景操作
   - 配置防火墙规则
   - 考虑为生产环境启用 TLS

3. **Docker 部署:**
   - 查看 [docker/README.md](docker/README.md)
   - 使用 docker-compose 进行多 agent 设置
   - 根据需要自定义 supervisor 配置

## 文档

- **部署脚本**: [scripts/README.md](scripts/README.md)
- **Docker 指南**: [docker/README.md](docker/README.md)
- **Supervisor 速查表**: [docker/SUPERVISOR_CHEATSHEET.md](docker/SUPERVISOR_CHEATSHEET.md)
- **Master 配置**: [master/config.yaml.example](master/config.yaml.example)
- **Agent 配置**: [agent/config.yaml.example](agent/config.yaml.example)
- **主 README**: [README.md](README.md)

---

**状态**: ✅ 所有请求的功能已实现并验证
**日期**: 2025-11-04
**版本**: v1.0.0
