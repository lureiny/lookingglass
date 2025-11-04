# GitHub Actions 工作流

本目录包含 LookingGlass 项目的 CI/CD 工作流配置。

## 工作流列表

### 1. Docker Image CI/CD (`docker-publish.yml`)

**触发条件**:
- Push 到 `main` 或 `master` 分支
- 创建 tag（格式: `v*`）
- Pull Request 到 `main` 或 `master` 分支
- 手动触发

**功能**:
- 自动构建 Master 和 Agent Docker 镜像
- 推送到 GitHub Container Registry (ghcr.io)
- 支持多架构构建（amd64, arm64）
- 自动标记（latest, 版本号, 分支名等）

**使用镜像**:

```bash
# 拉取最新版本
docker pull ghcr.io/lureiny/lookingglass/master:latest
docker pull ghcr.io/lureiny/lookingglass/agent:latest

# 拉取特定版本
docker pull ghcr.io/lureiny/lookingglass/master:v1.0.0
docker pull ghcr.io/lureiny/lookingglass/agent:v1.0.0
```

### 2. Go Build and Test (`go-test.yml`)

**触发条件**:
- Push 到 `main` 或 `master` 分支
- Pull Request 到 `main` 或 `master` 分支
- 手动触发

**功能**:
- 在多个 Go 版本下测试（1.21, 1.22）
- 运行所有单元测试
- 生成代码覆盖率报告
- 运行 golangci-lint 代码检查
- 上传覆盖率到 Codecov（可选）

### 3. Release (`release.yml`)

**触发条件**:
- 创建 tag（格式: `v*`）
- 手动触发

**功能**:
- 自动构建 Master 和 Agent 二进制文件
- 打包成 `.tar.gz` 格式
- 创建 GitHub Release
- 生成 Release Notes
- 附加下载链接和 Docker 镜像信息

**创建 Release**:

```bash
# 1. 创建并推送 tag
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# 2. GitHub Actions 自动构建并创建 Release
# 3. 在 GitHub Releases 页面查看结果
```

## 配置说明

### 必需的 GitHub 设置

1. **启用 GitHub Packages**
   - 项目设置 → Packages → 确保已启用

2. **配置包可见性**
   - 默认为私有，可在包设置中改为公开
   - 路径: Package settings → Change visibility → Public

3. **GitHub Token**
   - 工作流使用 `GITHUB_TOKEN` 自动提供
   - 无需额外配置

### 可选配置

#### Codecov 集成

如需启用代码覆盖率报告：

1. 在 [codecov.io](https://codecov.io) 注册并添加仓库
2. 获取 `CODECOV_TOKEN`
3. 添加到 GitHub Secrets: Settings → Secrets → New repository secret

#### 自定义镜像仓库

如需使用 Docker Hub 或其他仓库，修改 `docker-publish.yml`:

```yaml
env:
  REGISTRY: docker.io  # 或其他仓库
  IMAGE_NAME_MASTER: username/lookingglass-master
  IMAGE_NAME_AGENT: username/lookingglass-agent
```

## 开发流程

### 日常开发

```bash
# 1. 创建功能分支
git checkout -b feature/my-feature

# 2. 开发并提交
git add .
git commit -m "feat: add new feature"
git push origin feature/my-feature

# 3. 创建 Pull Request
# GitHub Actions 会自动运行测试

# 4. 合并到主分支后，自动构建 Docker 镜像
```

### 发布新版本

```bash
# 1. 更新版本号（如有需要）
# 2. 提交所有更改
git add .
git commit -m "chore: prepare release v1.0.0"
git push origin main

# 3. 创建 tag 并推送
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# 4. GitHub Actions 自动:
#    - 运行测试
#    - 构建二进制文件
#    - 打包
#    - 创建 GitHub Release
#    - 构建并推送 Docker 镜像
```

## 故障排查

### Docker 构建失败

1. 检查 Dockerfile 语法
2. 确保所有依赖文件存在
3. 查看 Actions 日志中的具体错误

### 镜像推送失败

1. 确认 `GITHUB_TOKEN` 有正确权限
2. 检查包设置是否正确
3. 确认分支/tag 名称匹配触发条件

### 测试失败

1. 本地运行 `make test` 重现问题
2. 检查 Go 版本兼容性
3. 查看详细测试日志

## Badge 状态

在 README.md 中添加 Badge:

```markdown
![Docker Build](https://github.com/lureiny/lookingglass/workflows/Docker%20Image%20CI%2FCD/badge.svg)
![Go Tests](https://github.com/lureiny/lookingglass/workflows/Go%20Build%20and%20Test/badge.svg)
[![codecov](https://codecov.io/gh/lureiny/lookingglass/branch/main/graph/badge.svg)](https://codecov.io/gh/lureiny/lookingglass)
```

## 参考资料

- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [GitHub Packages 文档](https://docs.github.com/en/packages)
- [Docker Buildx](https://docs.docker.com/buildx/working-with-buildx/)
