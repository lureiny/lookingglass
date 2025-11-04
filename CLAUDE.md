# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LookingGlass is a distributed network diagnostic system built with Golang + gRPC + Protobuf. It uses a Master-Agent architecture where:
- **Master**: Central server that manages agents, receives frontend requests (WebSocket), and schedules tasks
- **Agent**: Deployed on VPS servers to execute diagnostic commands (ping, mtr, etc.)
- **Communication**: Master ↔ Agent via gRPC with streaming, Master ↔ Frontend via WebSocket

## Build and Development Commands

```bash
# Install dependencies
make deps

# Generate protobuf code (must run after modifying .proto files)
make proto

# Build both master and agent binaries
make build

# Run Master server
./bin/master -config master/config.yaml

# Run Agent (in separate terminal)
./bin/agent -config agent/config.yaml
```

## Architecture Patterns

### Task Identification System

The system uses **task names (strings)** instead of enum-based TaskType for maximum flexibility:

- **Task Name**: String identifier (e.g., "ping", "mtr", "curl_test")
- **Display Name**: Human-readable name shown in frontend (e.g., "Ping", "HTTP Check")
- **Requires Target**: Boolean flag indicating if task needs target parameter

This allows adding new tasks purely through configuration without modifying protobuf or code.

### Communication Flow
```
CLI/Frontend (WebSocket) → Master (gRPC streaming) → Agent (execute commands)
CLI/Frontend ← Master (WebSocket) ← Agent (stream output)
```

- **Master to Agent**: gRPC with server streaming for real-time command output
- **Master to CLI/Frontend**: WebSocket with **Protobuf binary messages** for real-time updates
- **Agent to Master**: Heartbeat mechanism via gRPC for health monitoring + task metadata (display_name, requires_target)
- **CLI to Agent**: Direct gRPC connection (bypassing Master) for agent testing

### Agent Registration and Authentication

1. Agent connects to Master gRPC server on startup
2. Agent calls `Register` RPC with:
   - API Key in gRPC metadata: `x-api-key` header
   - **Task Display Info**: Array of supported tasks with display_name and requires_target flags
3. Master validates API Key and optionally checks IP whitelist (configurable auth mode)
4. Master stores task metadata and forwards to frontend via WebSocket
5. Agent sends periodic heartbeats; Master marks as offline if heartbeat timeout exceeded

**Important**:
- Authentication mode cannot be `none` - must use either `api_key` or `ip_whitelist` mode
- Master acts as pure forwarder - does NOT validate task types or agent capabilities
- All business logic validation happens on Agent side

### Concurrency Control

**Master Side** - Two-tier control:
1. **Global limit**: Total concurrent tasks across all agents (`global_max` in master config)
2. **Per-agent limit**: Max concurrent tasks per agent (`agent_default_max` in master config, can be overridden per agent)

Before dispatching a task, Master checks both limits and rejects if either is exceeded.

**Agent Side** - Two-tier control (new):
1. **Global limit**: Total concurrent tasks on this agent (`global_concurrency` in agent config, default: 10)
2. **Per-task-type limit**: Max concurrent tasks per task type (configured per task in `tasks.<name>.concurrency.max`)

Agent uses semaphores to enforce both limits. Tasks acquire the global semaphore first, then the task-specific semaphore (if configured).

## Protobuf Design Patterns

### Task Identification (String-Based)

Tasks are identified by **task_name** (string) instead of enum:
```protobuf
message Task {
  string task_id = 1;
  string agent_id = 2;
  string task_name = 3;              // "ping", "mtr", "curl_test", etc.
  TaskType type = 4 [deprecated];    // Legacy field, use task_name

  oneof params {
    NetworkTestParams network_test = 10;
  }
}
```

### Task Metadata for Frontend

```protobuf
message TaskDisplayInfo {
  string task_name = 1;        // Internal identifier
  string display_name = 2;     // Frontend display name
  string description = 3;      // Optional description
  bool requires_target = 4;    // Whether task needs target parameter
}
```

### Enums for Status Only

Status fields use enums for type safety:
- `AgentStatus`: ONLINE, OFFLINE
- `TaskStatus`: PENDING, RUNNING, COMPLETED, FAILED, CANCELLED
- `AuthMode`: API_KEY, IP_WHITELIST

**TaskType enum is deprecated** - use task_name strings for maximum flexibility.

## Key Data Structures

### Agent Information
```go
type Agent struct {
    ID              string             // Unique identifier (e.g., "us-west-1")
    Name            string             // Display name (e.g., "美国西部-洛杉矶")
    Location        string             // Geographic location
    IPv4, IPv6      string             // IP addresses
    Host            string             // Agent gRPC address
    Status          AgentStatus        // Online/offline
    LastHeartbeat   time.Time          // Last heartbeat timestamp
    MaxConcurrent   int                // Max concurrent tasks for this agent
    CurrentTasks    int                // Currently running tasks
    TaskDisplayInfo []*TaskDisplayInfo // Supported tasks with metadata
}
```

**Key Fields**:
- `ID`: Internal identifier
- `Name`: User-facing display name
- `TaskDisplayInfo`: Array containing all supported tasks with display names and parameter requirements

### IP Address Auto-Detection

Agent supports automatic public IP address detection during startup. This feature is implemented in `pkg/netutil/ip.go`:

**How it works**:
1. If `ipv4` or `ipv6` fields are empty in config, auto-detection is triggered
2. IPv4 detection tries multiple public APIs (ipify.org, ifconfig.me, icanhazip.com, ident.me)
3. IPv6 detection attempts similar services, fails gracefully if unavailable
4. Falls back to local network interface detection if external services fail

**Usage**:
```yaml
agent:
  ipv4: ""  # Leave empty to auto-detect
  ipv6: ""  # Leave empty to auto-detect (optional)
```

**Implementation**:
- Location: `pkg/netutil/ip.go`
- Called from: `agent/config/config.go` in `autoDetectIPs()` method
- Multiple service fallback ensures high availability
- Timeout: 10 seconds total, 5 seconds per service
- Non-blocking: Failures don't prevent agent startup

**Benefits**:
- No manual IP configuration needed
- Works in NAT environments (detects public IP)
- Suitable for dynamic IP environments
- Docker-friendly

## Real-Time Streaming Implementation

### Agent Side
- Execute command using `exec.Command`
- Stream output line-by-line via `bufio.Scanner`
- Send each line immediately via gRPC stream (`ExecuteTask` returns `stream TaskOutput`)

### Master Side
- Receive gRPC stream from Agent
- Forward each output line to frontend via WebSocket in real-time
- Track task state and notify on completion/failure

## Configuration Files

- **Master**: `master/config.yaml` - contains auth settings, concurrency limits, heartbeat timeouts
- **Agent**: `agent/config.yaml` - contains agent ID/name, master connection details, API key, command paths

Critical settings:
- `auth.mode`: Must be `api_key` or `ip_whitelist`
- `concurrency.global_max`: System-wide concurrent task limit
- `concurrency.agent_default_max`: Default per-agent concurrent task limit
- `agent.heartbeat_timeout`: Seconds before marking agent offline

## Project Structure

```
proto/                    # Protobuf definitions (.proto files)
pb/                       # Generated Go code from protobuf
master/                   # Master server implementation
  ├── config/             # Configuration loading
  ├── server/             # gRPC server + agent gRPC client
  ├── agent/              # Agent manager and registry
  ├── task/               # Task scheduler and concurrency control
  ├── auth/               # Authentication (API key, IP whitelist)
  └── ws/                 # WebSocket server for frontend
agent/                    # Agent implementation
  ├── config/             # Configuration loading
  ├── server/             # AgentService gRPC server
  ├── executor/           # Command executors (ping, mtr, etc.)
  └── client/             # Master gRPC client (register, heartbeat)
web/                      # Frontend (to be implemented)
bin/                      # Compiled binaries
```

## Security Considerations

- **Input Validation**: Strictly validate target IPs/domains to prevent command injection
- **API Key**: Use strong random keys (32+ characters), rotate regularly
- **TLS**: Production must use TLS for gRPC connections
- **IP Whitelist**: Provides additional security layer when enabled
- **Parameter Limits**: Enforce reasonable limits on ping count, timeout values, etc.

## Adding New Diagnostic Commands

**No code changes required!** Add tasks purely through configuration:

1. **Edit `agent/config.yaml`**:
   ```yaml
   executor:
     tasks:
       my_new_task:
         enabled: true
         display_name: "My New Task"
         requires_target: true  # or false if no target needed
         executor:
           type: command
           path: "/usr/bin/my-command"
           default_args: ["-v", "{target}"]
         concurrency:
           max: 2
   ```

2. **Restart Agent** - new task automatically registers with Master

3. **Frontend automatically updates** - new task appears in dropdown with correct display name

**For builtin tasks** (ping, mtr, nexttrace):
- Still require executor implementation in `agent/executor/`
- But configuration follows same pattern as custom commands

## Custom Command Tasks

The system supports fully custom command execution without modifying code. All tasks (builtin and custom) are configured in `agent/config.yaml`.

### Features

- **Template-based arguments**: Use placeholders like `{target}`, `{count}`, `{timeout}`, `{ipv6}` in command arguments
- **Per-task concurrency limits**: Configure how many instances of each task can run simultaneously
- **Display name customization**: Set user-friendly names for frontend display
- **Optional target parameter**: Tasks can declare they don't need target (e.g., system info commands)
- **No code changes required**: Add new diagnostic tools purely through configuration

### Configuration Example

```yaml
executor:
  global_concurrency: 10  # Global limit across all task types

  tasks:
    # Custom HTTP check using curl (requires target)
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

    # System info command (no target needed)
    system_info:
      enabled: true
      display_name: "System Info"
      requires_target: false  # Frontend disables target input
      executor:
        type: command
        path: "/usr/bin/uname"
        default_args: ["-a"]
      concurrency:
        max: 1

    # Builtin task with custom display name
    ping:
      enabled: true
      display_name: "网络延迟测试"  # Chinese display name
      requires_target: true
      concurrency:
        max: 3
```

### Template Placeholders

- `{target}`: Target IP/domain from frontend
- `{count}`: Count parameter (default: 4)
- `{timeout}`: Timeout in seconds
- `{ipv6}`: Boolean flag (true/false)

### Task Metadata Fields

- **display_name**: Name shown in frontend dropdown (required for good UX)
- **requires_target**: Boolean flag (default: true)
  - `true`: Frontend shows enabled target input
  - `false`: Frontend disables target input and clears value

### Implementation Details

**Location**: `agent/executor/command_builder.go`

- `BuildCustomCommandArgs()`: Performs template replacement
- `CreateCustomArgsBuilder()`: Returns a function closure with default args
- `NewCustomCommandExecutor()`: Factory function for custom command executors

### Frontend Behavior

When user selects a task:
1. Frontend reads `requires_target` from `TaskDisplayInfo`
2. If `false`: Target input is **disabled and cleared**
3. If `true`: Target input is **enabled and required**

### Use Cases

**Requires Target**:
- HTTP/HTTPS checks (curl, wget)
- DNS lookups (dig, nslookup)
- Network diagnostics (ping, mtr, traceroute)

**No Target Required**:
- System information (uname, hostname)
- Disk usage (df -h)
- Network interface info (ip addr)
- CPU/Memory stats (top, free)

## Implementation Details

### Executor Interface
All command executors must implement:
```go
type Executor interface {
    Execute(ctx context.Context, task *pb.Task, outputChan chan<- *pb.TaskOutput) error
    Cancel(taskID string) error
    Type() pb.TaskType
}
```

### WebSocket Protocol (CLI ↔ Master)

Uses **Protobuf binary format** for all WebSocket communication:

**Request Message** (`pb.WSRequest`):
```protobuf
message WSRequest {
  enum Action {
    ACTION_EXECUTE = 1;
    ACTION_CANCEL = 2;
  }
  Action action = 1;
  Task task = 2;        // For ACTION_EXECUTE
  string task_id = 3;   // For ACTION_CANCEL
}
```

**Response Message** (`pb.WSResponse`):
```protobuf
message WSResponse {
  enum Type {
    TYPE_OUTPUT = 1;
    TYPE_ERROR = 2;
    TYPE_COMPLETE = 3;
    TYPE_TASK_STARTED = 4;
  }
  Type type = 1;
  string task_id = 2;
  string output = 3;
  string message = 4;
}
```

**Why Protobuf binary over JSON?**
- 40-50% smaller message size
- 5-8x faster serialization/deserialization
- Type-safe at compile time
- Consistent with gRPC (entire system uses protobuf)

### Task Lifecycle
1. CLI/Frontend sends execute request via WebSocket (**Protobuf binary**)
2. Master's `task.Scheduler` validates concurrency limits
3. Master calls `agentManager.ExecuteTaskOnAgent()` which sends gRPC request
4. Agent's `executor.Manager` acquires semaphore and spawns command
5. Command output streams through: Agent → Master gRPC → Master WebSocket (**Protobuf binary**) → CLI/Frontend
6. On completion, counters are decremented and resources cleaned up

### Error Handling
- Context cancellation propagates through all layers
- Timeouts are enforced at both task level and executor level
- Agent failures are detected via heartbeat timeout
- Failed tasks send error message through output stream

### Logging
- Uses `zap` structured logging throughout
- Log levels: debug (heartbeats), info (tasks), warn (failures), error (critical)
- Logs include task_id, agent_id for correlation
- JSON format for production, console for development

## Web Frontend

### Overview

The web frontend is a modern, single-page application (SPA) built with vanilla JavaScript, HTML, and CSS. It provides a user-friendly interface for executing network diagnostic commands via the Master server.

**Location**: `web/` directory

**Tech Stack**:
- Pure HTML + CSS + JavaScript (no framework dependencies)
- Protobuf.js for binary message serialization
- WebSocket for real-time communication with Master

### Architecture

```
Browser (web/index.html)
  ├── Protobuf.js (CDN) - Binary serialization
  ├── js/protobuf.js - Message encoding/decoding
  ├── js/websocket.js - WebSocket client wrapper
  └── js/app.js - Main application logic
      │
      ├─> WebSocket → ws://master:8080/ws (protobuf binary)
      └─> LocalStorage → History persistence
```

### Key Features

1. **Real-time Agent List**:
   - Automatically fetches and displays all registered agents
   - Shows online/offline status, location, IP, current tasks
   - Click to select agent for command execution

2. **Command Execution**:
   - Only shows tools supported by selected agent
   - Simple input: target IP/domain only
   - Count parameters fixed in Master config (default: 4)
   - Real-time streaming output display

3. **Terminal Output**:
   - Live command output with auto-scroll
   - Color-coded: prompts (green), output (white), errors (red)
   - Clear output button

4. **History**:
   - Stores last 50 commands in browser LocalStorage
   - Collapsible panel (default collapsed)
   - Click to quickly re-run previous commands

5. **Single Task Execution**:
   - Only one command can run at a time per user
   - Cancel button appears during execution
   - Form disabled while task is running

### WebSocket Protocol

Frontend uses the same Protobuf binary protocol as CLI:

**Request** (`WSRequest`):
```javascript
// List agents
{ action: ACTION_LIST_AGENTS }

// Execute task
{
  action: ACTION_EXECUTE,
  task: {
    taskId: "uuid",
    agentId: "us-west-1",
    type: TASK_TYPE_PING,
    networkTest: { target: "8.8.8.8", count: 4 }
  }
}

// Cancel task
{ action: ACTION_CANCEL, taskId: "uuid" }
```

**Response** (`WSResponse`):
```javascript
// Agent list
{ type: TYPE_AGENT_LIST, agents: [...] }

// Task started
{ type: TYPE_TASK_STARTED, taskId: "uuid" }

// Output line
{ type: TYPE_OUTPUT, output: "PING 8.8.8.8...\n" }

// Complete
{ type: TYPE_COMPLETE, message: "success" }

// Error
{ type: TYPE_ERROR, message: "error details" }
```

### Master Configuration for Frontend

```yaml
server:
  ws_port: 8080   # WebSocket endpoint for frontend
  http_port: 8081  # Static file serving (serves web/ directory)

task:
  default_ping_count: 4  # Used by frontend (user doesn't input count)
  default_mtr_count: 4
```

### Deployment

**Development**:
```bash
# Start Master (serves static files automatically)
./bin/master -config master/config.yaml

# Access frontend
open http://localhost:8081
```

**Production**:
- Serve `web/` directory via nginx/Apache
- Configure CORS if frontend and Master on different domains
- Use HTTPS/WSS for secure connections
- Update WebSocket URL in `js/app.js` if needed

### File Structure

```
web/
├── index.html        # Main page (SPA)
├── css/
│   └── style.css     # Modern responsive styling
├── js/
│   ├── protobuf.js   # Protobuf schema + encode/decode
│   ├── websocket.js  # WebSocket client class
│   └── app.js        # Application logic + DOM manipulation
└── README.md         # Frontend-specific documentation
```

### Adding New Task Types

When adding a new executor (e.g., `TASK_TYPE_SPEEDTEST`):

1. Update `proto/lookingglass.proto` and run `make proto`
2. Update `web/js/protobuf.js`:
   - Add to `TaskType` enum
   - Add to `getTaskTypes()` and `getTaskTypeName()`
3. Update `web/js/app.js`:
   - Add to `toolNames` mapping in `updateToolSelect()`

No need to modify HTML or CSS for basic task types.

### Browser Requirements

- WebSocket support
- ES6+ JavaScript (classes, arrow functions, async/await)
- CSS Grid and Flexbox
- LocalStorage API

Tested on: Chrome 90+, Firefox 88+, Safari 14+, Edge 90+

### Security Considerations

1. **No Authentication**: Frontend currently has no auth. For production:
   - Add API key or JWT token authentication
   - Implement user sessions
   - Use Master's auth system

2. **XSS Protection**: All user input is sanitized before display

3. **CORS**: Master must allow frontend origin if on different domain

4. **Data Privacy**: History stored locally, contains no credentials

### Future Enhancements

- Multi-task execution (task queue)
- Real-time agent status updates (WebSocket push)
- Export results (JSON/CSV)
- User authentication
- Dark/light theme toggle
- Internationalization (i18n)
