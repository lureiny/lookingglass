# Architecture Documentation

本文档详细介绍 LookingGlass 的系统架构和设计理念。

## 系统概览

LookingGlass 是一个分布式网络诊断系统，采用 Master-Agent 架构：

```
┌──────────────────────────────────────────────────┐
│                    Frontend                       │
│              (Browser / Web App)                  │
└─────────────────┬────────────────────────────────┘
                  │ WebSocket (Protobuf Binary)
                  │
┌─────────────────▼────────────────────────────────┐
│              Master Server                        │
│  ┌──────────────────────────────────────────┐   │
│  │         WebSocket Server                  │   │
│  │  - 处理前端连接                          │   │
│  │  - 发送 Agent 列表和状态                 │   │
│  │  - 转发任务请求和结果                    │   │
│  └──────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────┐   │
│  │         gRPC Server                       │   │
│  │  - 接受 Agent 注册                       │   │
│  │  - 维护 Agent 连接                       │   │
│  │  - 心跳监控                              │   │
│  └──────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────┐   │
│  │         Agent Manager                     │   │
│  │  - Agent 状态管理                        │   │
│  │  - 任务元数据存储                        │   │
│  │  - 在线/离线状态跟踪                     │   │
│  └──────────────────────────────────────────┘   │
└─────────────────┬────────────────────────────────┘
                  │ gRPC Bidirectional Stream
                  │
    ┌─────────────┴──────────────┬─────────────┐
    │                            │             │
┌───▼────────┐          ┌────────▼───┐   ┌────▼───────┐
│  Agent 1   │          │  Agent 2   │   │  Agent 3   │
│            │          │            │   │            │
│  Tokyo     │          │  London    │   │  NYC       │
└────────────┘          └────────────┘   └────────────┘
```

## 核心设计理念

### 1. 任务名称系统（String-Based）

**设计决策**: 使用字符串标识任务，而非枚举类型

**优势**:
- ✅ 无需修改 protobuf 即可添加新任务
- ✅ 配置文件驱动，零代码改动
- ✅ 支持动态任务注册
- ✅ 简化扩展流程

**实现**:
```protobuf
message Task {
  string task_name = 3;        // "ping", "mtr", "curl_test"
  TaskType type = 4 [deprecated]; // 废弃的枚举字段
}

message TaskDisplayInfo {
  string task_name = 1;        // 内部标识符
  string display_name = 2;     // 前端显示名称
  bool requires_target = 4;    // 是否需要 target
}
```

### 2. Master 纯转发层

**设计决策**: Master 不做业务逻辑验证

**职责**:
- ✅ 转发 Agent 注册信息到前端
- ✅ 转发任务请求到 Agent
- ✅ 转发任务结果到前端
- ❌ ~~验证任务类型~~
- ❌ ~~验证 Agent 能力~~
- ❌ ~~修改任务参数~~

**优势**:
- Master 代码简单，易维护
- Agent 完全自治，独立扩展
- 减少 Master 和 Agent 的耦合

### 3. Agent 自治架构

**设计决策**: Agent 负责所有业务逻辑

**职责**:
- ✅ 任务注册和元数据管理
- ✅ 任务参数验证
- ✅ 并发控制
- ✅ 命令执行
- ✅ 输出流式传输

**配置示例**:
```yaml
executor:
  tasks:
    curl_test:
      enabled: true
      display_name: "HTTP Check"
      requires_target: true
      executor:
        path: "/usr/bin/curl"
        default_args: ["-I", "{target}"]
      concurrency:
        max: 2
```

### 4. 参数控制系统

**设计决策**: 通过 `requires_target` 标识参数需求

**工作流程**:

1. **Agent 配置**:
   ```yaml
   system_info:
     requires_target: false  # 不需要 target
   ```

2. **Agent 注册时发送元数据**:
   ```go
   TaskDisplayInfo{
     TaskName: "system_info",
     DisplayName: "System Info",
     RequiresTarget: false,
   }
   ```

3. **Master 转发到前端**:
   ```json
   {
     "taskDisplayInfo": [{
       "taskName": "system_info",
       "displayName": "System Info",
       "requiresTarget": false
     }]
   }
   ```

4. **前端动态控制**:
   ```javascript
   if (requiresTarget) {
     targetInput.disabled = false;  // 启用
     targetInput.required = true;
   } else {
     targetInput.disabled = true;   // 禁用
     targetInput.value = '';         // 清空
     targetInput.required = false;
   }
   ```

## 通信协议

### 1. Agent ↔ Master (gRPC Stream)

**协议**: gRPC 双向流

**消息类型**:

#### Agent → Master

```protobuf
message AgentMessage {
  enum Type {
    TYPE_REGISTER = 1;     // 注册
    TYPE_HEARTBEAT = 2;    // 心跳
    TYPE_TASK_OUTPUT = 3;  // 任务输出
  }
  
  oneof payload {
    RegisterRequest register = 2;
    HeartbeatRequest heartbeat = 3;
    TaskOutput task_output = 4;
  }
}
```

#### Master → Agent

```protobuf
message MasterMessage {
  enum Type {
    TYPE_REGISTER_ACK = 1;   // 注册确认
    TYPE_TASK_REQUEST = 2;   // 任务请求
    TYPE_TASK_CANCEL = 3;    // 取消任务
  }
  
  oneof payload {
    RegisterResponse register_ack = 2;
    Task task_request = 3;
    CancelTaskRequest cancel = 4;
  }
}
```

### 2. Frontend ↔ Master (WebSocket)

**协议**: WebSocket + Protobuf 二进制

**消息类型**:

#### Frontend → Master

```protobuf
message WSRequest {
  enum Action {
    ACTION_EXECUTE = 1;      // 执行任务
    ACTION_CANCEL = 2;       // 取消任务
    ACTION_LIST_AGENTS = 3;  // 获取 Agent 列表
  }
  
  Action action = 1;
  Task task = 2;          // 任务详情
  string task_id = 3;     // 任务 ID
}
```

#### Master → Frontend

```protobuf
message WSResponse {
  enum Type {
    TYPE_OUTPUT = 1;              // 任务输出
    TYPE_ERROR = 2;               // 错误
    TYPE_COMPLETE = 3;            // 完成
    TYPE_TASK_STARTED = 4;        // 任务已启动
    TYPE_AGENT_LIST = 5;          // Agent 列表
    TYPE_AGENT_STATUS_UPDATE = 6; // Agent 状态更新
  }
  
  Type type = 1;
  string task_id = 2;
  string output = 3;
  repeated AgentStatusInfo agents = 5;
}
```

## 数据流

### 任务执行流程

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│Frontend │     │ Master  │     │  Agent  │
└────┬────┘     └────┬────┘     └────┬────┘
     │               │               │
     │ 1. Execute    │               │
     │  task_name=   │               │
     │  "ping"       │               │
     ├──────────────>│               │
     │               │ 2. Forward    │
     │               │   Task        │
     │               ├──────────────>│
     │               │               │
     │               │               │ 3. Validate
     │               │               │    task_name
     │               │               │
     │               │               │ 4. Execute
     │               │               │    command
     │               │               │
     │               │ 5. Stream     │
     │ 6. Forward    │   Output      │
     │   Output      │<──────────────┤
     │<──────────────┤               │
     │               │               │
     │ 7. Complete   │ 8. Complete   │
     │<──────────────┤<──────────────┤
     │               │               │
```

### Agent 注册流程

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│  Agent  │     │ Master  │     │Frontend │
└────┬────┘     └────┬────┘     └────┬────┘
     │               │               │
     │ 1. Connect    │               │
     │   gRPC Stream │               │
     ├──────────────>│               │
     │               │               │
     │ 2. Register   │               │
     │   + API Key   │               │
     │   + TaskInfo[]│               │
     ├──────────────>│               │
     │               │               │
     │               │ 3. Validate   │
     │               │    API Key    │
     │               │               │
     │ 4. Ack        │               │
     │<──────────────┤               │
     │               │               │
     │               │ 5. Broadcast  │
     │               │   Agent List  │
     │               ├──────────────>│
     │               │               │
     │ 6. Heartbeat  │               │
     │──────────────>│               │
     │               │               │
```

## 并发控制

### 三级并发限制

1. **Master 全局限制** (`master/config.yaml`)
   ```yaml
   concurrency:
     global_max: 100  # 整个系统最大并发
   ```

2. **Agent 全局限制** (`agent/config.yaml`)
   ```yaml
   executor:
     global_concurrency: 10  # 该 Agent 最大并发
   ```

3. **任务级别限制** (`agent/config.yaml`)
   ```yaml
   executor:
     tasks:
       ping:
         concurrency:
           max: 5  # ping 任务最大并发
   ```

### 实现机制

```go
// Agent 端使用 Semaphore 控制
type Manager struct {
    globalSem *semaphore.Weighted      // 全局信号量
    taskSems  map[string]*semaphore.Weighted // 任务级信号量
}

func (m *Manager) ExecuteTask(task *Task) error {
    // 1. 获取全局信号量
    m.globalSem.Acquire(ctx, 1)
    defer m.globalSem.Release(1)
    
    // 2. 获取任务信号量
    if taskSem, ok := m.taskSems[task.Name]; ok {
        taskSem.Acquire(ctx, 1)
        defer taskSem.Release(1)
    }
    
    // 3. 执行任务
    return m.executor.Execute(ctx, task)
}
```

## 扩展性设计

### 添加新任务（零代码）

**步骤**:

1. 编辑 `agent/config.yaml`:
   ```yaml
   executor:
     tasks:
       my_new_tool:
         enabled: true
         display_name: "My Tool"
         requires_target: true
         executor:
           path: "/usr/bin/my-tool"
           default_args: ["{target}"]
   ```

2. 重启 Agent

3. 前端自动更新

**无需修改**:
- ❌ Protobuf 定义
- ❌ Go 代码
- ❌ JavaScript 代码
- ❌ Master 配置

### 添加新 Executor（需要代码）

仅在内置任务（ping, mtr, nexttrace）需要特殊逻辑时才需要实现新 Executor。

**步骤**:

1. 实现 Executor 接口:
   ```go
   type Executor interface {
       Execute(ctx context.Context, task *Task, 
               outputChan chan<- *TaskOutput) error
       Cancel(taskID string) error
       Type() string
   }
   ```

2. 注册到 Registry:
   ```go
   registry.RegisterExecutor("my_type", 
       NewMyExecutor())
   ```

## 安全设计

### 1. 认证机制

- **API Key**: gRPC metadata 中传输
- **IP 白名单**: 可选的额外验证层

### 2. 命令注入防护

- 使用绝对路径
- 参数模板化
- 不允许任意命令

### 3. 最小权限

- Agent 以非特权用户运行
- 限制命令执行范围

## 性能优化

### 1. 连接复用

- Agent 使用单一 gRPC 流
- 减少连接开销

### 2. 二进制协议

- Protobuf 比 JSON 小 40-50%
- 序列化速度提升 5-8x

### 3. 流式传输

- 实时输出，无缓冲延迟
- 降低内存占用

## 相关文档

- [CLAUDE.md](../CLAUDE.md) - 详细技术文档
- [TASK_CONFIG.md](TASK_CONFIG.md) - 任务配置指南
- [README.md](../README.md) - 项目概览
