# Task Configuration Guide

本文档介绍如何通过配置文件添加和管理任务，无需修改代码。

## 任务配置架构

LookingGlass 使用 **基于字符串的任务名称系统**，完全通过配置文件管理任务。

### 核心概念

- **Task Name** (task_name): 内部标识符，如 `ping`, `curl_test`
- **Display Name** (display_name): 前端显示名称，如 `Ping`, `HTTP 检查`
- **Requires Target** (requires_target): 是否需要 target 参数

## 配置示例

### 基本任务配置

在 `agent/config.yaml` 中配置任务：

```yaml
executor:
  global_concurrency: 10  # 全局并发限制

  tasks:
    # 内置任务 - Ping
    ping:
      enabled: true
      display_name: "Ping"
      requires_target: true
      concurrency:
        max: 3

    # 内置任务 - MTR
    mtr:
      enabled: true
      display_name: "MTR"
      requires_target: true
      concurrency:
        max: 2

    # 内置任务 - NextTrace
    nexttrace:
      enabled: true
      display_name: "NextTrace"
      requires_target: true
      concurrency:
        max: 2
```

### 自定义命令任务

#### 需要 Target 参数的任务

```yaml
executor:
  tasks:
    # HTTP 检查
    curl_test:
      enabled: true
      display_name: "HTTP Check"
      requires_target: true
      executor:
        type: command
        path: "/usr/bin/curl"
        default_args: ["-I", "-m", "10", "{target}"]
        line_formatter: "none"
      concurrency:
        max: 2

    # DNS 查询
    dns_query:
      enabled: true
      display_name: "DNS Lookup"
      requires_target: true
      executor:
        type: command
        path: "/usr/bin/dig"
        default_args: ["+short", "{target}"]
      concurrency:
        max: 3
```

#### 不需要 Target 参数的任务

```yaml
executor:
  tasks:
    # 系统信息
    system_info:
      enabled: true
      display_name: "System Info"
      requires_target: false  # 前端将禁用 target 输入框
      executor:
        type: command
        path: "/usr/bin/uname"
        default_args: ["-a"]
      concurrency:
        max: 1

    # 网络接口信息
    network_interfaces:
      enabled: true
      display_name: "Network Interfaces"
      requires_target: false
      executor:
        type: command
        path: "/usr/bin/ip"
        default_args: ["addr", "show"]
      concurrency:
        max: 1

    # 磁盘使用情况
    disk_usage:
      enabled: true
      display_name: "Disk Usage"
      requires_target: false
      executor:
        type: command
        path: "/usr/bin/df"
        default_args: ["-h"]
      concurrency:
        max: 1
```

## 配置字段说明

### 必填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用该任务 |
| `display_name` | string | 前端显示名称 |

### 可选字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `requires_target` | bool | `true` | 是否需要 target 参数 |
| `executor.type` | string | - | 执行器类型（目前仅支持 `command`）|
| `executor.path` | string | - | 命令路径 |
| `executor.default_args` | []string | - | 默认参数列表 |
| `executor.line_formatter` | string | `"none"` | 输出格式化器 |
| `concurrency.max` | int | 无限制 | 该任务最大并发数 |

### 模板占位符

在 `default_args` 中可以使用以下占位符：

- `{target}` - 前端传入的目标地址
- `{count}` - 次数参数（默认 4）
- `{timeout}` - 超时时间（秒）
- `{ipv6}` - 是否使用 IPv6（true/false）

## 前端行为

### requires_target = true（默认）

- Target 输入框：**启用**
- 输入框状态：可编辑
- 表单验证：必填
- Placeholder：`e.g., 8.8.8.8 or google.com`

### requires_target = false

- Target 输入框：**禁用**
- 输入框状态：灰色不可编辑
- 输入框内容：自动清空
- 表单验证：不检查
- Placeholder：`No target required`

## 国际化支持

Display Name 支持任意 Unicode 字符，可以使用不同语言：

```yaml
executor:
  tasks:
    ping:
      enabled: true
      display_name: "网络延迟测试"  # 中文
      requires_target: true

    mtr:
      enabled: true
      display_name: "ルート追跡"  # 日语
      requires_target: true

    nexttrace:
      enabled: true
      display_name: "Трассировка"  # 俄语
      requires_target: true
```

## 最佳实践

### 1. 任务命名

- **task_name**: 使用小写字母和下划线，如 `curl_test`, `system_info`
- **display_name**: 使用用户友好的名称，如 `HTTP Check`, `系统信息`

### 2. 并发控制

```yaml
# 高频任务 - 较高并发
ping:
  concurrency:
    max: 5

# 资源密集型任务 - 较低并发
nexttrace:
  concurrency:
    max: 2

# 系统查询任务 - 限制为 1
system_info:
  concurrency:
    max: 1
```

### 3. 安全考虑

```yaml
# ✅ 好的做法 - 使用绝对路径
executor:
  path: "/usr/bin/curl"

# ❌ 避免 - 使用相对路径或环境变量
executor:
  path: "curl"  # 可能被劫持
```

### 4. 参数安全

```yaml
# ✅ 好的做法 - 限制参数
default_args: ["-I", "-m", "10", "{target}"]

# ❌ 避免 - 允许任意参数注入
default_args: ["{target}"]  # 用户可以传入 "example.com; rm -rf /"
```

## 添加新任务步骤

1. **编辑配置文件** (`agent/config.yaml`)
   ```yaml
   executor:
     tasks:
       my_new_task:
         enabled: true
         display_name: "My New Task"
         requires_target: true
         executor:
           type: command
           path: "/usr/bin/my-command"
           default_args: ["{target}"]
         concurrency:
           max: 2
   ```

2. **重启 Agent**
   ```bash
   systemctl restart lookingglass-agent
   # 或
   docker restart lookingglass-agent
   ```

3. **验证注册**

   检查 Master 日志：
   ```
   Agent registered successfully [task_names=[ping, mtr, my_new_task]]
   ```

4. **前端验证**

   刷新浏览器，新任务应该出现在任务下拉列表中。

## 故障排除

### 任务没有出现在前端

**检查点**:
1. Agent 配置文件中 `enabled: true`
2. Agent 已重启
3. Agent 成功连接到 Master（检查日志）
4. 前端已刷新页面

### 执行任务时报错

**常见原因**:
1. 命令路径不正确（使用 `which <command>` 查看路径）
2. 命令未安装
3. 权限不足（命令需要 root 权限）
4. 参数格式错误

**调试方法**:
```bash
# 在 Agent 机器上手动测试命令
/usr/bin/curl -I -m 10 example.com

# 查看 Agent 日志
tail -f /var/log/lookingglass-agent.log
```

## 示例：完整配置

```yaml
executor:
  global_concurrency: 20

  tasks:
    # === 网络诊断工具 ===
    ping:
      enabled: true
      display_name: "Ping"
      requires_target: true
      concurrency:
        max: 5

    mtr:
      enabled: true
      display_name: "MTR"
      requires_target: true
      concurrency:
        max: 3

    nexttrace:
      enabled: true
      display_name: "NextTrace"
      requires_target: true
      concurrency:
        max: 2

    # === HTTP 工具 ===
    curl_test:
      enabled: true
      display_name: "HTTP Check"
      requires_target: true
      executor:
        type: command
        path: "/usr/bin/curl"
        default_args: ["-I", "-L", "-m", "10", "{target}"]
      concurrency:
        max: 3

    # === DNS 工具 ===
    dns_query:
      enabled: true
      display_name: "DNS Query"
      requires_target: true
      executor:
        type: command
        path: "/usr/bin/dig"
        default_args: ["+short", "{target}"]
      concurrency:
        max: 5

    # === 系统信息（无需 target）===
    system_info:
      enabled: true
      display_name: "System Info"
      requires_target: false
      executor:
        type: command
        path: "/usr/bin/uname"
        default_args: ["-a"]
      concurrency:
        max: 1

    network_info:
      enabled: true
      display_name: "Network Info"
      requires_target: false
      executor:
        type: command
        path: "/usr/bin/ip"
        default_args: ["addr", "show"]
      concurrency:
        max: 1
```

## 相关文档

- [CLAUDE.md](../CLAUDE.md) - 完整架构文档
- [DEPLOYMENT.md](DEPLOYMENT.md) - 部署指南
- [agent/config.yaml.example](../agent/config.yaml.example) - 配置示例
