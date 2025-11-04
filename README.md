# LookingGlass

<div align="center">

![Docker Build](https://github.com/lureiny/lookingglass/workflows/Docker%20Image%20CI%2FCD/badge.svg)
![Go Tests](https://github.com/lureiny/lookingglass/workflows/Go%20Build%20and%20Test/badge.svg)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

一个基于 Golang + gRPC + Protobuf 实现的分布式网络诊断系统

[功能特性](#功能特性) •
[快速开始](#快速开始) •
[文档](#文档) •
[架构](#架构) •
[贡献](#贡献)

</div>

---

## ✨ 功能特性

- 🌐 **分布式架构** - Master-Agent 模式，支持全球多节点部署
- ⚡ **实时流式输出** - WebSocket + gRPC 实时传输命令执行结果
- 🔐 **安全认证** - 支持 API Key 和 IP 白名单双重认证
- 🎯 **并发控制** - 全局和单 Agent 两级并发限制
- 📊 **Web 界面** - 现代化单页面应用，无需安装任何工具
- 🐳 **容器化** - 完整的 Docker 支持，一键部署
- 🔧 **灵活任务系统** - 基于配置的任务管理，无需修改代码即可添加新工具
- 🎨 **自定义显示** - 支持自定义任务显示名称，多语言友好
- 🚀 **自动 IP 检测** - Agent 启动时自动检测公网 IP，无需手动配置
- 🛠️ **智能参数控制** - 根据任务类型自动显示/隐藏参数输入框

## 🎯 使用场景

- **多地网络质量监测** - 从全球不同位置测试服务可达性
- **CDN 节点选择** - 测试不同 CDN 节点的延迟和路由
- **网络故障排查** - 快速定位网络问题和路由异常
- **服务质量监控** - 监控关键服务的网络连通性

## 🚀 快速开始

### 方式一：自动安装脚本（推荐 - 生产环境）

使用自动化脚本部署，无需 Docker，自动生成配置：

```bash
# 1. 克隆仓库并构建
git clone https://github.com/lureiny/lookingglass.git
cd lookingglass
make build

# 2. 安装 Master（自动生成 API Key 和配置）
sudo ./scripts/install.sh master

# 3. 安装 Agent（在同一台或其他机器）
sudo ./scripts/install.sh agent

# 4. 查看状态
./scripts/manage.sh status

# 5. 访问 Web 界面
open http://localhost:8080
```

**特点**:
- ✅ 自动生成随机 API Key
- ✅ 自动检测公网 IP
- ✅ 使用 Supervisor 管理进程，自动重启
- ✅ 一键部署，无需手动配置

详见 [脚本部署指南](scripts/README.md)。

### 方式二：Docker 部署（推荐 - 测试环境）

```bash
# 1. 克隆仓库
git clone https://github.com/lureiny/lookingglass.git
cd lookingglass

# 2. 准备配置文件
cp master/config.yaml.example master/config.yaml
cp agent/config.yaml.example agent/config.yaml

# 3. 修改配置中的 API Key（重要！）
# 编辑 master/config.yaml 和 agent/config.yaml，修改 api_key

# 4. 启动服务
make docker-build
make docker-up

# 5. 访问 Web 界面
open http://localhost:8080
```

### 方式三：手动部署

```bash
# 1. 安装依赖
make deps

# 2. 构建
make build

# 3. 配置
cp master/config.yaml.example master/config.yaml
cp agent/config.yaml.example agent/config.yaml
# 编辑配置文件...

# 4. 运行 Master
./bin/master -config master/config.yaml

# 5. 运行 Agent（在另一个终端）
./bin/agent -config agent/config.yaml
```

完整部署文档请参阅 [部署指南](docs/DEPLOYMENT.md)。

## 📚 文档

### 部署相关
- [脚本部署指南](scripts/README.md) - 自动化脚本部署（推荐）
- [部署指南](docs/DEPLOYMENT.md) - 完整的生产环境部署文档
- [Docker 指南](docs/DOCKER.md) - Docker 容器化部署

### 配置和开发
- [任务配置指南](docs/TASK_CONFIG.md) - 如何添加和配置任务
- [开发指南](docs/DEVELOPMENT.md) - 开发环境配置和贡献指南
- [架构说明](CLAUDE.md) - 项目架构和设计模式

### 管理和运维
- [Supervisor 管理](docker/README.md) - Supervisor 进程管理指南
- [Supervisor 速查表](docker/SUPERVISOR_CHEATSHEET.md) - 常用命令快速参考

## 🏗️ 架构

```
┌─────────────────┐
│   浏览器/前端    │ ← WebSocket (Protobuf 二进制)
└────────┬────────┘
         │
┌────────▼────────┐
│  Master Server  │ ← gRPC 双向流
│                 │
│  - Agent 管理   │
│  - 任务调度     │
│  - 并发控制     │
│  - 认证授权     │
└────────┬────────┘
         │ gRPC Stream
         │
    ┌────┴─────┬───────────┐
    │          │           │
┌───▼───┐  ┌───▼───┐  ┌───▼───┐
│Agent 1│  │Agent 2│  │Agent 3│
│美国西 │  │香港   │  │德国   │
└───────┘  └───────┘  └───────┘
```

### 核心特性

- **Stream 架构**: Agent 主动连接 Master，通过单一双向 gRPC 流通信
- **NAT 穿透**: Agent 无需公网 IP，可部署在任何网络环境
- **自动重连**: Agent 断线自动重连，支持指数退避
- **零端口**: Agent 不监听任何端口，安全性更高
- **配置驱动**: 通过配置文件添加任务，无需修改代码
- **任务元数据**: 支持自定义显示名称和参数控制

## 🛠️ 技术栈

- **后端**: Go 1.21+, gRPC, Protobuf
- **前端**: 原生 JavaScript, WebSocket, Protobuf.js
- **容器**: Docker, Docker Compose
- **日志**: Zap (结构化日志)
- **诊断工具**: ping, mtr, nexttrace

## 📦 打包和发布

```bash
# 打包 Master
make package-master

# 打包 Agent
make package-agent

# 打包所有
make package-all
```

生成的压缩包位于 `dist/` 目录。

## 🤖 AI 辅助开发

本项目使用 Claude Code 进行 AI 辅助开发。项目包含：

- **[CLAUDE.md](CLAUDE.md)** - 给 AI 的项目说明文档，包含架构、设计模式、开发规范等
- 作为 AI + 人工协作的示例项目

## 🔒 安全建议

- ✅ **修改默认 API Key**: 使用强随机密钥（32+ 字符）
- ✅ **启用 IP 白名单**: 生产环境建议启用
- ✅ **使用 TLS**: 公网部署建议启用 gRPC TLS
- ✅ **定期更新**: 及时更新依赖和镜像
- ✅ **最小权限**: 使用专用用户运行服务

## 🤝 贡献

欢迎贡献代码、报告问题或提出改进建议！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

详见 [贡献指南](CONTRIBUTING.md)（如果有）。

## 📝 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- [gRPC](https://grpc.io/) - 高性能 RPC 框架
- [Protocol Buffers](https://protobuf.dev/) - 序列化协议
- [Zap](https://github.com/uber-go/zap) - 高性能日志库
- [Claude](https://www.anthropic.com/claude) - AI 辅助开发

## 📧 联系方式

- GitHub Issues: https://github.com/lureiny/lookingglass/issues
- 项目主页: https://github.com/lureiny/lookingglass

---

<div align="center">
Made with ❤️ and 🤖 (Claude Code)
</div>
