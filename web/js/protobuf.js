// Protobuf message definitions and handlers
const ProtoHandler = {
    // Message types will be loaded dynamically
    root: null,
    WSRequest: null,
    WSResponse: null,
    Task: null,
    NetworkTestParams: null,

    // Initialize protobuf from JSON schema
    async init() {
        // Define protobuf schema inline (subset of lookingglass.proto)
        // Note: We omit timestamp fields as they're not needed for client requests
        const proto = `
syntax = "proto3";

package lookingglass;

enum AgentStatus {
    AGENT_STATUS_UNSPECIFIED = 0;
    AGENT_STATUS_ONLINE = 1;
    AGENT_STATUS_OFFLINE = 2;
}

enum TaskType {
    TASK_TYPE_UNSPECIFIED = 0;
    TASK_TYPE_PING = 1;
    TASK_TYPE_MTR = 2;
    TASK_TYPE_TRACEROUTE = 3;
    TASK_TYPE_SYSBENCH = 4;
    TASK_TYPE_NEXTTRACE = 5;
    TASK_TYPE_CUSTOM_COMMAND = 6;
}

message NetworkTestParams {
    string target = 1;
    int32 count = 2;
    int32 timeout = 3;
    bool ipv6 = 4;
    map<string, string> extra_options = 5;
    string custom_task_name = 6;
}

message Task {
    string task_id = 1;
    string agent_id = 2;
    string task_name = 3;
    TaskType type = 4;
    int32 timeout = 6;

    oneof params {
        NetworkTestParams network_test = 10;
    }
}

message WSRequest {
    enum Action {
        ACTION_UNSPECIFIED = 0;
        ACTION_EXECUTE = 1;
        ACTION_CANCEL = 2;
        ACTION_LIST_AGENTS = 3;
    }

    Action action = 1;
    Task task = 2;
    string task_id = 3;
}

// Task metadata for frontend display (used for both builtin and custom tasks)
message TaskDisplayInfo {
    string task_name = 1;
    string display_name = 2;
    string description = 3;
    bool requires_target = 4;
}

// Deprecated: Use TaskDisplayInfo instead
message CustomCommandInfo {
    string task_name = 1;
    string display_name = 2;
    string description = 3;
}

message AgentStatusInfo {
    string id = 1;
    string name = 2;
    string location = 3;
    string ipv4 = 4;
    string ipv6 = 5;
    AgentStatus status = 6;
    repeated TaskType supported_tasks = 7;
    int32 current_tasks = 8;
    int32 max_concurrent = 9;
    string provider = 10;
    string idc = 11;
    string description = 12;
    repeated CustomCommandInfo custom_commands = 13;
    repeated string task_names = 14;
    repeated TaskDisplayInfo task_display_info = 15;
}

message WSResponse {
    enum Type {
        TYPE_UNSPECIFIED = 0;
        TYPE_OUTPUT = 1;
        TYPE_ERROR = 2;
        TYPE_COMPLETE = 3;
        TYPE_TASK_STARTED = 4;
        TYPE_AGENT_LIST = 5;
        TYPE_AGENT_STATUS_UPDATE = 6;
    }

    Type type = 1;
    string task_id = 2;
    string output = 3;
    string message = 4;
    repeated AgentStatusInfo agents = 5;
}
        `;

        try {
            this.root = protobuf.parse(proto).root;
            this.WSRequest = this.root.lookupType('lookingglass.WSRequest');
            this.WSResponse = this.root.lookupType('lookingglass.WSResponse');
            this.Task = this.root.lookupType('lookingglass.Task');
            this.NetworkTestParams = this.root.lookupType('lookingglass.NetworkTestParams');
            this.AgentStatusInfo = this.root.lookupType('lookingglass.AgentStatusInfo');

            console.log('Protobuf initialized successfully');
            return true;
        } catch (error) {
            console.error('Failed to initialize protobuf:', error);
            return false;
        }
    },

    // Encode WSRequest to binary
    encodeRequest(action, data = {}) {
        const message = {
            action: this.WSRequest.Action[action],
            ...data
        };

        console.log('Encoding request:', { action, message });

        const errMsg = this.WSRequest.verify(message);
        if (errMsg) {
            console.error('Verification error:', errMsg);
            throw Error(errMsg);
        }

        const req = this.WSRequest.create(message);
        const encoded = this.WSRequest.encode(req).finish();
        console.log('Encoded request:', encoded.length, 'bytes');
        return encoded;
    },

    // Decode WSResponse from binary
    decodeResponse(buffer) {
        return this.WSResponse.decode(new Uint8Array(buffer));
    },

    // Helper: Create LIST_AGENTS request
    createListAgentsRequest() {
        return this.encodeRequest('ACTION_LIST_AGENTS');
    },

    // Helper: Create EXECUTE request
    createExecuteRequest(agentId, taskName, target) {
        // Determine count based on task name
        let count = 4;  // Default for ping and mtr
        if (taskName === 'nexttrace') {
            count = 50;  // NextTrace uses max hops
        }

        const networkTest = {
            target: target,
            count: count,
            timeout: 0,
            ipv6: false,
            extraOptions: {}
        };

        const task = {
            taskId: this.generateUUID(),
            agentId: agentId,
            taskName: taskName,  // New: using task_name field
            type: 0,  // Deprecated, set to UNSPECIFIED
            timeout: 300,
            networkTest: networkTest
        };

        console.log('Creating execute request with task:', task);
        return this.encodeRequest('ACTION_EXECUTE', { task });
    },

    // Helper: Create CANCEL request
    createCancelRequest(taskId) {
        return this.encodeRequest('ACTION_CANCEL', { taskId });
    },

    // Generate UUID (simple version)
    generateUUID() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
            const r = Math.random() * 16 | 0;
            const v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    },

    // Get task type enum values
    getTaskTypes() {
        return {
            PING: 1,
            MTR: 2,
            TRACEROUTE: 3,
            SYSBENCH: 4,
            NEXTTRACE: 5,
            CUSTOM_COMMAND: 6
        };
    },

    // Get task type name from enum value
    getTaskTypeName(value) {
        const types = {
            1: 'PING',
            2: 'MTR',
            3: 'TRACEROUTE',
            4: 'SYSBENCH',
            5: 'NEXTTRACE',
            6: 'CUSTOM_COMMAND'
        };
        return types[value] || 'UNKNOWN';
    }
};
