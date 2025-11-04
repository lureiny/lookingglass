// WebSocket Client for LookingGlass
class LookingGlassClient {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.connected = false;
        this.currentTaskId = null;

        // Event handlers
        this.onConnectionChange = null;
        this.onAgentList = null;
        this.onAgentStatusUpdate = null;  // New handler for status updates
        this.onTaskStarted = null;
        this.onOutput = null;
        this.onComplete = null;
        this.onError = null;
    }

    // Connect to WebSocket server
    async connect() {
        return new Promise((resolve, reject) => {
            try {
                this.ws = new WebSocket(this.url);
                this.ws.binaryType = 'arraybuffer';

                this.ws.onopen = () => {
                    console.log('WebSocket connected');
                    this.connected = true;
                    if (this.onConnectionChange) {
                        this.onConnectionChange(true);
                    }
                    resolve();
                };

                this.ws.onclose = () => {
                    console.log('WebSocket disconnected');
                    this.connected = false;
                    if (this.onConnectionChange) {
                        this.onConnectionChange(false);
                    }
                };

                this.ws.onerror = (error) => {
                    console.error('WebSocket error:', error);
                    reject(error);
                };

                this.ws.onmessage = (event) => {
                    this.handleMessage(event.data);
                };
            } catch (error) {
                reject(error);
            }
        });
    }

    // Handle incoming messages
    handleMessage(data) {
        try {
            const response = ProtoHandler.decodeResponse(data);
            // console.log('Received message:', response);

            switch (response.type) {
                case 5: // TYPE_AGENT_LIST
                    if (this.onAgentList) {
                        this.onAgentList(response.agents);
                    }
                    break;

                case 6: // TYPE_AGENT_STATUS_UPDATE
                    console.log('Received agent status update');
                    if (this.onAgentStatusUpdate) {
                        this.onAgentStatusUpdate(response.agents);
                    }
                    break;

                case 4: // TYPE_TASK_STARTED
                    if (this.onTaskStarted) {
                        this.onTaskStarted(response.taskId);
                    }
                    break;

                case 1: // TYPE_OUTPUT
                    if (this.onOutput) {
                        this.onOutput(response.output, response.message);
                    }
                    break;

                case 3: // TYPE_COMPLETE
                    this.currentTaskId = null;
                    if (this.onComplete) {
                        this.onComplete(response.message);
                    }
                    break;

                case 2: // TYPE_ERROR
                    this.currentTaskId = null;
                    if (this.onError) {
                        this.onError(response.message);
                    }
                    break;

                default:
                    console.warn('Unknown message type:', response.type);
            }
        } catch (error) {
            console.error('Failed to handle message:', error);
        }
    }

    // Send binary message
    send(data) {
        if (!this.connected || !this.ws) {
            throw new Error('Not connected to WebSocket');
        }
        console.log('Sending WebSocket message:', data.length, 'bytes');
        this.ws.send(data);
    }

    // Request agent list
    requestAgentList() {
        const request = ProtoHandler.createListAgentsRequest();
        this.send(request);
    }

    // Execute task
    executeTask(agentId, taskName, target) {
        const request = ProtoHandler.createExecuteRequest(agentId, taskName, target);
        this.send(request);

        // Extract task ID from the request for tracking
        // (In a real implementation, you'd parse the request to get the task ID)
        this.currentTaskId = 'task-' + Date.now();
        return this.currentTaskId;
    }

    // Cancel current task
    cancelTask() {
        if (!this.currentTaskId) {
            return;
        }
        const request = ProtoHandler.createCancelRequest(this.currentTaskId);
        this.send(request);
    }

    // Close connection
    close() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.connected = false;
    }

    // Check connection status
    isConnected() {
        return this.connected && this.ws && this.ws.readyState === WebSocket.OPEN;
    }
}
