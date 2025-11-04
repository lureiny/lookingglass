# LookingGlass Development Guide

## Quick Start

### Prerequisites

- Go 1.24+ installed
- Protocol Buffers compiler (`protoc`)
- `ping` and `mtr` commands available on agent machines

### Build

```bash
# Install dependencies
make deps

# Generate protobuf code
make proto

# Build both master and agent
make build

# Or build separately
make build-master
make build-agent
```

### Configuration

1. **Master Configuration** (`master/config.yaml`):
   - Update `auth.api_key` with a strong secret key
   - Configure `concurrency.global_max` based on your capacity
   - Adjust `auth.mode` to `ip_whitelist` for additional security

2. **Agent Configuration** (`agent/config.yaml`):
   - Set unique `agent.id` for each agent
   - Update `agent.name` and `agent.location` for display
   - Match `master.api_key` with master's configuration
   - Update `master.host` to point to your master server
   - Verify `executor.ping_path` and `executor.mtr_path` are correct

### Running Locally

Terminal 1 - Start Master:
```bash
./bin/master -config master/config.yaml
```

Terminal 2 - Start Agent:
```bash
./bin/agent -config agent/config.yaml
```

### Testing with WebSocket

You can test the WebSocket API using a simple HTML client or curl:

```bash
# Get agent list
curl http://localhost:8080/api/agents

# WebSocket connection (use a WebSocket client)
# Connect to: ws://localhost:8080/ws/task

# Send execute request:
{
  "action": "execute",
  "agent_id": "local-agent",
  "type": "ping",
  "params": {
    "target": "8.8.8.8",
    "count": 4,
    "timeout": 5
  }
}

# Send cancel request:
{
  "action": "cancel",
  "task_id": "task-uuid-here"
}
```

## Architecture Overview

### Master Server

- **gRPC Server** (Port 50051): Handles agent registration and heartbeat
- **WebSocket Server** (Port 8080): Serves frontend connections and task requests
- **HTTP API** (Port 8080): Provides REST endpoints for agent list, etc.

### Agent Server

- **gRPC Client**: Connects to master for registration and heartbeat
- **gRPC Server** (Port 50052): Receives task execution requests from master
- **Executor Manager**: Manages concurrent task execution

### Communication Flow

```
Frontend (WebSocket) → Master → Agent (gRPC) → Execute Command
Frontend ← Master ← Agent (gRPC Stream) ← Command Output
```

## Development Workflow

### Adding New Tasks

LookingGlass 使用**配置驱动**的任务系统，无需修改代码即可添加新任务。

#### 方法 1: 使用内置命令 (推荐)

1. 编辑 `agent/config.yaml` 添加新任务配置:
   ```yaml
   executor:
     tasks:
       # 添加自定义 HTTP 检查
       http_check:
         enabled: true
         display_name: "HTTP Check"
         requires_target: true
         executor:
           type: command
           path: "/usr/bin/curl"
           default_args: ["-I", "-m", "10", "{target}"]
         concurrency:
           max: 2
   ```

2. 重启 Agent:
   ```bash
   systemctl restart lookingglass-agent
   ```

3. 验证: 刷新前端，新任务会自动出现在任务列表中

**详细配置指南**: 查看 [docs/TASK_CONFIG.md](TASK_CONFIG.md)

#### 方法 2: 实现新的 Executor (高级)

如果需要复杂的命令逻辑（非标准命令行工具），可以实现自定义 Executor:

1. 在 `agent/executor/` 中创建新的 executor:
   ```go
   type CustomExecutor struct { ... }
   func (e *CustomExecutor) Execute(ctx context.Context, task *pb.Task, outputChan chan<- *pb.TaskOutput) error {
       // 自定义执行逻辑
   }
   ```

2. 在 `agent/main.go` 中注册:
   ```go
   customExecutor := executor.NewCustomExecutor(...)
   taskManager.RegisterExecutor("custom_task", customExecutor)
   ```

3. 在 `agent/config.yaml` 中启用:
   ```yaml
   executor:
     tasks:
       custom_task:
         enabled: true
         display_name: "Custom Task"
   ```

**注意**: Master 是纯转发层，无需修改任何 Master 代码

### Code Structure

```
├── proto/              # Protobuf definitions
├── pb/                 # Generated Go code
├── master/
│   ├── auth/          # Authentication logic
│   ├── agent/         # Agent manager
│   ├── task/          # Task scheduler
│   ├── server/        # gRPC server
│   ├── ws/            # WebSocket server
│   └── config/        # Configuration
├── agent/
│   ├── executor/      # Command executors
│   ├── server/        # gRPC server
│   ├── client/        # Master client
│   └── config/        # Configuration
└── bin/               # Compiled binaries
```

## Deployment

### Docker Deployment (Recommended)

Create `Dockerfile.master`:
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache make protobuf-dev
RUN make deps && make build-master

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bin/master .
COPY master/config.yaml .
CMD ["./master", "-config", "config.yaml"]
```

Create `Dockerfile.agent`:
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache make protobuf-dev
RUN make deps && make build-agent

FROM alpine:latest
RUN apk --no-cache add ca-certificates iputils mtr
WORKDIR /root/
COPY --from=builder /app/bin/agent .
COPY agent/config.yaml .
CMD ["./agent", "-config", "config.yaml"]
```

Build and run:
```bash
docker build -f Dockerfile.master -t lookingglass-master .
docker build -f Dockerfile.agent -t lookingglass-agent .

docker run -p 50051:50051 -p 8080:8080 lookingglass-master
docker run lookingglass-agent
```

## Optimization Recommendations

### Performance

1. **Connection Pooling**: Agent connections are maintained in `agent.Manager`
2. **Concurrent Task Limits**: Configurable at both global and per-agent levels
3. **Streaming Output**: Real-time streaming reduces memory usage
4. **Context Cancellation**: Proper cleanup of cancelled tasks

### Security

1. **API Key Authentication**: Required for all agent connections
2. **IP Whitelist**: Additional security layer for sensitive environments
3. **TLS Support**: Ready for implementation (commented in code)
4. **Input Validation**: Validate all task parameters to prevent command injection

### Monitoring

1. **Structured Logging**: Using zap with JSON output
2. **Task Tracking**: All tasks tracked with status and timestamps
3. **Agent Health**: Heartbeat mechanism with automatic offline detection
4. **Metrics**: Ready for Prometheus integration (add `/metrics` endpoint)

## Troubleshooting

### Agent Cannot Connect to Master

1. Check network connectivity: `telnet <master-host> 50051`
2. Verify API key matches in both configs
3. Check master logs for authentication errors
4. Ensure IP whitelist includes agent IP (if using `ip_whitelist` mode)

### Tasks Failing to Execute

1. Check agent logs for executor errors
2. Verify command paths in agent config (`ping_path`, `mtr_path`)
3. Ensure agent has permission to execute commands
4. Check task timeout settings

### WebSocket Connection Issues

1. Verify WebSocket port is accessible
2. Check browser console for connection errors
3. Ensure CORS settings if accessing from different origin
4. Review master logs for WebSocket upgrade errors

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Format code
make fmt
```

## Contributing

1. Follow Go coding standards
2. Add tests for new features
3. Update protobuf definitions for API changes
4. Document configuration changes
5. Test with multiple agents before PR
