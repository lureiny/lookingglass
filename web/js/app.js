// Main Application Logic
class LookingGlassApp {
    constructor() {
        this.client = null;
        this.agents = [];
        this.selectedAgent = null;
        this.selectedProvider = 'all';  // Filter state
        this.expandedAgents = new Set();  // Track expanded agent IDs
        this.history = [];
        this.isExecuting = false;
        this.lastSelectedTaskType = null;  // Track user's command preference

        // DOM elements
        this.elements = {
            connectionIndicator: document.getElementById('connection-indicator'),
            connectionText: document.getElementById('connection-text'),
            nodesTitle: document.getElementById('nodes-title'),
            agentList: document.getElementById('agent-list'),
            agentSelect: document.getElementById('agent-select'),
            providerFilter: document.getElementById('provider-filter'),
            toolSelect: document.getElementById('tool-select'),
            targetInput: document.getElementById('target-input'),
            executeBtn: document.getElementById('execute-btn'),
            cancelBtn: document.getElementById('cancel-btn'),
            commandForm: document.getElementById('command-form'),
            outputTerminal: document.getElementById('output-terminal'),
            outputTerminalWrapper: document.getElementById('output-terminal-wrapper'),
            outputHeader: document.getElementById('output-header'),
            clearOutputBtn: document.getElementById('clear-output-btn'),
            historyList: document.getElementById('history-list'),
            selectedNodeInfo: document.getElementById('selected-node-info'),
            selectedNodeText: document.getElementById('selected-node-text'),
            pageFooter: document.getElementById('page-footer'),
            footerContent: document.getElementById('footer-content')
        };

        this.init();
    }

    async init() {
        // Initialize protobuf
        console.log('Initializing protobuf...');
        const protoReady = await ProtoHandler.init();
        if (!protoReady) {
            this.showError('Failed to initialize protobuf');
            return;
        }

        // Load branding configuration
        await this.loadBranding();

        // Load history from localStorage
        this.loadHistory();

        // Setup event listeners
        this.setupEventListeners();

        // Connect to WebSocket
        await this.connect();
    }

    async loadBranding() {
        try {
            // Determine API URL
            const protocol = window.location.protocol;
            const host = window.location.hostname || 'localhost';
            const port = window.location.port || (window.location.protocol === 'https:' ? '443' : '80');
            const apiUrl = `${protocol}//${host}:${port}/api/branding`;

            console.log('Fetching branding from:', apiUrl);

            const response = await fetch(apiUrl);
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const branding = await response.json();
            console.log('Loaded branding:', branding);

            // Apply branding
            if (branding.site_title) {
                document.title = branding.site_title;
            }

            // Handle logo: support both image and text
            const logoElement = document.querySelector('.logo');
            if (logoElement) {
                // Clear existing content
                logoElement.innerHTML = '';

                // Add logo image if URL is provided
                if (branding.logo_url) {
                    const img = document.createElement('img');
                    img.src = branding.logo_url;
                    img.alt = 'Logo';
                    img.className = 'logo-image';
                    logoElement.appendChild(img);
                }

                // Add logo text - backend handles default values
                if (branding.logo_text) {
                    const textSpan = document.createElement('span');
                    textSpan.className = 'logo-text';
                    textSpan.textContent = branding.logo_text;
                    logoElement.appendChild(textSpan);
                }
            }

            // Handle subtitle - backend handles default values
            const subtitleElement = document.querySelector('.subtitle');
            if (subtitleElement && branding.subtitle) {
                subtitleElement.textContent = branding.subtitle;
            } else if (subtitleElement && !branding.subtitle) {
                // Hide subtitle if not configured
                subtitleElement.style.display = 'none';
            }

            if (branding.footer_text) {
                // Render footer
                this.elements.footerContent.innerHTML = branding.footer_text;
                this.elements.pageFooter.style.display = 'block';
            }
        } catch (error) {
            console.error('Failed to load branding:', error);
            // Continue with default branding if loading fails
        }
    }

    async connect() {
        try {
            // Determine WebSocket URL
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const host = window.location.hostname || 'localhost';
            const port = window.location.port || (window.location.protocol === 'https:' ? '443' : '80');
            const wsUrl = `${protocol}//${host}:${port}/ws`;

            console.log('Connecting to:', wsUrl);

            this.client = new LookingGlassClient(wsUrl);

            // Setup client event handlers
            this.client.onConnectionChange = (connected) => {
                this.updateConnectionStatus(connected);
                if (connected) {
                    // Request agent list when connected
                    setTimeout(() => this.client.requestAgentList(), 500);
                }
            };

            this.client.onAgentList = (agents) => this.handleAgentList(agents);
            this.client.onAgentStatusUpdate = (agents) => this.handleAgentStatusUpdate(agents);
            this.client.onTaskStarted = (taskId) => this.handleTaskStarted(taskId);
            this.client.onOutput = (output, error) => this.handleOutput(output, error);
            this.client.onComplete = (message) => this.handleComplete(message);
            this.client.onError = (error) => this.handleError(error);

            await this.client.connect();
        } catch (error) {
            console.error('Connection failed:', error);
            this.showError('Failed to connect to server: ' + error.message);
        }
    }

    setupEventListeners() {
        // Provider filter
        this.elements.providerFilter.addEventListener('change', (e) => {
            this.selectedProvider = e.target.value;
            this.handleProviderChange();
        });

        // Agent selection
        this.elements.agentSelect.addEventListener('change', () => {
            this.handleAgentSelection();
        });

        // Task selection - track user's task preference across node changes
        this.elements.toolSelect.addEventListener('change', (e) => {
            this.lastSelectedTaskType = e.target.value;  // Store task name (string)
            this.updateToolSelect();
        });

        // Form submission
        this.elements.commandForm.addEventListener('submit', (e) => {
            e.preventDefault();
            this.executeCommand();
        });

        // Cancel button
        this.elements.cancelBtn.addEventListener('click', () => {
            this.cancelExecution();
        });

        // Clear output
        this.elements.clearOutputBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            this.clearOutput();
        });
    }

    updateConnectionStatus(connected) {
        if (connected) {
            this.elements.connectionIndicator.className = 'indicator online';
            this.elements.connectionText.textContent = 'Connected';
        } else {
            this.elements.connectionIndicator.className = 'indicator offline';
            this.elements.connectionText.textContent = 'Disconnected';
        }
    }

    handleAgentList(agents) {
        console.log('Received agent list:', agents);
        this.agents = agents;
        this.updateProviderFilter();
        this.renderAgentList();
        this.updateAgentSelect();

        // Auto-select first online agent if none selected
        if (!this.selectedAgent && agents.length > 0) {
            const firstOnlineAgent = agents.find(a => a.status === 1);
            if (firstOnlineAgent) {
                this.selectAgent(firstOnlineAgent);
            }
        }
    }

    handleAgentStatusUpdate(agents) {
        console.log('Received agent status update:', agents);
        this.agents = agents;
        this.updateProviderFilter();
        this.renderAgentList();
        this.updateAgentSelect();

        // If selected agent went offline, clear selection
        if (this.selectedAgent) {
            const updatedAgent = agents.find(a => a.id === this.selectedAgent.id);
            if (updatedAgent && updatedAgent.status !== 1) {
                console.log('Selected agent went offline, clearing selection');
                this.selectedAgent = null;
                this.elements.toolSelect.disabled = true;
                this.elements.targetInput.disabled = true;
                this.elements.executeBtn.disabled = true;
                this.updateSelectedNodeInfo();
            } else if (updatedAgent) {
                // Update selected agent info
                this.selectedAgent = updatedAgent;
                this.updateSelectedNodeInfo();
            }
        }
    }

    updateProviderFilter() {
        // Extract unique providers from agents
        const providers = new Set();
        this.agents.forEach(agent => {
            if (agent.provider) {
                providers.add(agent.provider);
            }
        });

        // Sort providers alphabetically
        const sortedProviders = Array.from(providers).sort();

        // Update dropdown
        const select = this.elements.providerFilter;
        const currentValue = select.value;
        select.innerHTML = '<option value="all">All</option>';

        sortedProviders.forEach(provider => {
            const option = document.createElement('option');
            option.value = provider;
            option.textContent = provider;
            select.appendChild(option);
        });

        // Restore previous selection if it still exists
        if (currentValue !== 'all' && sortedProviders.includes(currentValue)) {
            select.value = currentValue;
        }
    }

    handleProviderChange() {
        // Get filtered agents for new provider
        let filteredAgents = this.agents;
        if (this.selectedProvider !== 'all') {
            filteredAgents = this.agents.filter(agent => agent.provider === this.selectedProvider);
        }

        // Get online agents only
        const onlineFilteredAgents = filteredAgents.filter(a => a.status === 1);

        // Sort agents using the same logic as renderAgentList
        const sortedOnlineAgents = [...onlineFilteredAgents].sort((a, b) => {
            // First by provider
            const providerA = a.provider || '';
            const providerB = b.provider || '';
            if (providerA !== providerB) {
                return providerA.localeCompare(providerB);
            }

            // Then by location
            const locationA = a.location || '';
            const locationB = b.location || '';
            if (locationA !== locationB) {
                return locationA.localeCompare(locationB);
            }

            // Finally by IDC
            const idcA = a.idc || '';
            const idcB = b.idc || '';
            return idcA.localeCompare(idcB);
        });

        // Check if currently selected agent is in the new filtered list
        if (this.selectedAgent) {
            const isSelectedAgentInList = sortedOnlineAgents.some(a => a.id === this.selectedAgent.id);

            if (!isSelectedAgentInList) {
                // Current agent not in new provider list, select first online agent from new list
                if (sortedOnlineAgents.length > 0) {
                    this.selectAgent(sortedOnlineAgents[0]);
                } else {
                    // No online agents in new provider, clear selection
                    this.selectedAgent = null;
                    this.elements.toolSelect.disabled = true;
                    this.elements.targetInput.disabled = true;
                    this.elements.executeBtn.disabled = true;
                    this.updateSelectedNodeInfo();
                }
            }
            // If selected agent is still in the list, keep the selection
        }

        // Re-render the agent list
        this.renderAgentList();
    }

    renderAgentList() {
        const agentListDiv = this.elements.agentList;
        agentListDiv.innerHTML = '';

        if (this.agents.length === 0) {
            agentListDiv.innerHTML = '<div class="loading">No agents available</div>';
            this.updateNodesTitle(0, 0);
            return;
        }

        // Filter agents by provider
        let filteredAgents = this.agents;
        if (this.selectedProvider !== 'all') {
            filteredAgents = this.agents.filter(agent => agent.provider === this.selectedProvider);
        }

        if (filteredAgents.length === 0) {
            agentListDiv.innerHTML = '<div class="loading">No agents match the filter</div>';
            this.updateNodesTitle(0, 0);
            return;
        }

        // Count online and total agents
        const onlineCount = filteredAgents.filter(a => a.status === 1).length;
        const totalCount = filteredAgents.length;
        this.updateNodesTitle(onlineCount, totalCount);

        // Separate online and offline agents
        const onlineAgents = filteredAgents.filter(a => a.status === 1);
        const offlineAgents = filteredAgents.filter(a => a.status !== 1);

        // Sort function
        const sortAgents = (agents) => {
            return [...agents].sort((a, b) => {
                // First by provider
                const providerA = a.provider || '';
                const providerB = b.provider || '';
                if (providerA !== providerB) {
                    return providerA.localeCompare(providerB);
                }

                // Then by location
                const locationA = a.location || '';
                const locationB = b.location || '';
                if (locationA !== locationB) {
                    return locationA.localeCompare(locationB);
                }

                // Finally by IDC
                const idcA = a.idc || '';
                const idcB = b.idc || '';
                return idcA.localeCompare(idcB);
            });
        };

        const sortedOnlineAgents = sortAgents(onlineAgents);
        const sortedOfflineAgents = sortAgents(offlineAgents);

        // Render online agents section
        if (sortedOnlineAgents.length > 0) {
            const onlineSection = document.createElement('div');
            onlineSection.className = 'agent-list-section';
            onlineSection.innerHTML = '<div class="agent-list-section-title">Online</div>';
            sortedOnlineAgents.forEach(agent => {
                onlineSection.appendChild(this.createAgentElement(agent));
            });
            agentListDiv.appendChild(onlineSection);
        }

        // Render offline agents section
        if (sortedOfflineAgents.length > 0) {
            const offlineSection = document.createElement('div');
            offlineSection.className = 'agent-list-section';
            offlineSection.innerHTML = '<div class="agent-list-section-title">Offline</div>';
            sortedOfflineAgents.forEach(agent => {
                offlineSection.appendChild(this.createAgentElement(agent));
            });
            agentListDiv.appendChild(offlineSection);
        }
    }

    createAgentElement(agent) {
        // Debug: log agent IP info
        console.log(`Agent ${agent.name}: IPv4=${agent.ipv4}, IPv6=${agent.ipv6}`);

        const agentItem = document.createElement('div');
        agentItem.className = 'agent-item';
        if (agent.status !== 1) {  // Not ONLINE
            agentItem.classList.add('offline');
        }
        if (this.selectedAgent && this.selectedAgent.id === agent.id) {
            agentItem.classList.add('selected');
        }
        if (this.expandedAgents.has(agent.id)) {
            agentItem.classList.add('expanded');
        }

        const statusText = agent.status === 1 ? 'online' : 'offline';
        const statusClass = agent.status === 1 ? 'online' : 'offline';

        // Compact info line: IDC • Provider • Location
        const infoLine = [
            agent.idc || 'N/A',
            agent.provider || 'N/A',
            agent.location || 'N/A'
        ];

        const isExpanded = this.expandedAgents.has(agent.id);
        const hasDescription = agent.description && agent.description.trim();

        agentItem.innerHTML = `
            <div class="agent-header">
                <div class="agent-header-row">
                    <div class="agent-name">${agent.name}</div>
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <div class="agent-status ${statusClass}">${statusText}</div>
                        ${hasDescription ? '<span class="agent-expand-arrow">▼</span>' : ''}
                    </div>
                </div>
                <div class="agent-info-compact">
                    ${infoLine.map(info => `<span>${info}</span>`).join('')}
                </div>
                ${agent.ipv4 || agent.ipv6 ? `<div class="agent-ip-row">
                    ${agent.ipv4 ? `<div class="agent-ip">IPv4: ${agent.ipv4}</div>` : ''}
                    ${agent.ipv6 ? `<div class="agent-ip">IPv6: ${agent.ipv6}</div>` : ''}
                </div>` : ''}
            </div>
            ${hasDescription ? `<div class="agent-description ${isExpanded ? 'expanded' : ''}">${agent.description}</div>` : ''}
        `;

        const header = agentItem.querySelector('.agent-header');
        const arrow = agentItem.querySelector('.agent-expand-arrow');

        // Click handler for selection (online agents only)
        if (agent.status === 1) {
            header.addEventListener('click', (e) => {
                // If clicking on arrow, toggle expand instead
                if (hasDescription && (e.target === arrow || arrow?.contains(e.target))) {
                    e.stopPropagation();
                    this.toggleAgentExpand(agent.id);
                } else {
                    this.selectAgent(agent);
                }
            });
        }

        // Arrow click handler for expansion
        if (arrow) {
            arrow.addEventListener('click', (e) => {
                e.stopPropagation();
                this.toggleAgentExpand(agent.id);
            });
        }

        return agentItem;
    }

    updateNodesTitle(onlineCount, totalCount) {
        this.elements.nodesTitle.textContent = `Nodes (${onlineCount}/${totalCount})`;
    }

    selectAgent(agent) {
        this.selectedAgent = agent;
        this.elements.agentSelect.value = agent.id;
        this.renderAgentList();
        this.updateToolSelect();
        this.updateSelectedNodeInfo();
    }

    updateSelectedNodeInfo() {
        if (this.selectedAgent) {
            // Format: Name (Location) • Provider • IDC
            const parts = [this.selectedAgent.name];
            if (this.selectedAgent.location) {
                parts[0] += ` (${this.selectedAgent.location})`;
            }
            if (this.selectedAgent.provider) {
                parts.push(this.selectedAgent.provider);
            }
            if (this.selectedAgent.idc) {
                parts.push(this.selectedAgent.idc);
            }

            this.elements.selectedNodeText.textContent = parts.join(' • ');
            this.elements.selectedNodeInfo.style.display = 'flex';
        } else {
            this.elements.selectedNodeInfo.style.display = 'none';
        }
    }

    toggleAgentExpand(agentId) {
        if (this.expandedAgents.has(agentId)) {
            this.expandedAgents.delete(agentId);
        } else {
            this.expandedAgents.add(agentId);
        }
        this.renderAgentList();
    }

    updateAgentSelect() {
        const select = this.elements.agentSelect;
        // select.innerHTML = '<option value="">Select an agent...</option>';

        this.agents.filter(a => a.status === 1).forEach(agent => {
            const option = document.createElement('option');
            option.value = agent.id;
            option.textContent = `${agent.name} (${agent.location})`;
            select.appendChild(option);
        });

        select.disabled = this.agents.length === 0;
    }

    handleAgentSelection() {
        const agentId = this.elements.agentSelect.value;
        if (!agentId) {
            this.selectedAgent = null;
            this.elements.toolSelect.disabled = true;
            this.elements.targetInput.disabled = true;
            this.elements.executeBtn.disabled = true;
            return;
        }

        this.selectedAgent = this.agents.find(a => a.id === agentId);
        this.renderAgentList();
        this.updateToolSelect();
    }

    updateToolSelect() {
        const select = this.elements.toolSelect;
        select.replaceChildren();
        // select.innerHTML = '<option value="">Select a tool...</option>';

        if (!this.selectedAgent) {
            select.disabled = true;
            this.elements.targetInput.disabled = true;
            this.elements.executeBtn.disabled = true;
            return;
        }

        // Check if agent has task_display_info field (new architecture)
        if (!this.selectedAgent.taskDisplayInfo || this.selectedAgent.taskDisplayInfo.length === 0) {
            console.warn('Agent has no taskDisplayInfo:', this.selectedAgent);
            select.disabled = true;
            this.elements.targetInput.disabled = true;
            this.elements.executeBtn.disabled = true;
            return;
        }

        let hasLastSelected = false;

        // Add all tasks from task_display_info array
        this.selectedAgent.taskDisplayInfo.forEach(taskInfo => {
            const option = document.createElement('option');
            option.value = taskInfo.taskName;

            // Use display_name from agent configuration
            const displayName = taskInfo.displayName || taskInfo.taskName;
            option.textContent = displayName;

            // Store requires_target info in option dataset for later use
            option.dataset.requiresTarget = taskInfo.requiresTarget !== false; // Default true

            select.appendChild(option);

            // Check if this agent supports the user's last selected task
            if (this.lastSelectedTaskType && taskInfo.taskName === this.lastSelectedTaskType) {
                hasLastSelected = true;
            }
        });

        // Selection priority:
        // 1. If new agent supports the previously selected task, keep it selected
        // 2. Otherwise, select the first task in the list
        if (hasLastSelected) {
            select.value = this.lastSelectedTaskType;
        } else if (select.options.length > 0) {
            // Select first task if previous selection is not supported
            select.selectedIndex = 0;
        }

        select.disabled = false;

        // Update target input based on selected task
        this.updateTargetInputState();

        this.elements.executeBtn.disabled = false;
    }

    updateTargetInputState() {
        const select = this.elements.toolSelect;
        const selectedOption = select.options[select.selectedIndex];

        if (!selectedOption) {
            this.elements.targetInput.disabled = true;
            this.elements.targetInput.required = false;
            return;
        }

        const requiresTarget = selectedOption.dataset.requiresTarget === 'true';

        if (requiresTarget) {
            // Task requires target
            this.elements.targetInput.disabled = false;
            this.elements.targetInput.required = true;
            this.elements.targetInput.placeholder = 'e.g., 8.8.8.8 or google.com';
        } else {
            // Task does not require target
            this.elements.targetInput.value = '';
            this.elements.targetInput.disabled = true;
            this.elements.targetInput.required = false;
            this.elements.targetInput.placeholder = 'No target required';
        }
    }

    executeCommand() {
        if (this.isExecuting) {
            return;
        }

        const agentId = this.elements.agentSelect.value;
        const taskName = this.elements.toolSelect.value;  // Now this is the task name directly
        const target = this.elements.targetInput.value.trim();

        var requiresTarget = true;
        const select = this.elements.toolSelect;
        const selectedOption = select.options[select.selectedIndex];

        if (selectedOption) {
            requiresTarget = selectedOption.dataset.requiresTarget === 'true';
        }

        if (!agentId || !taskName || (requiresTarget && !target)) {
            this.showError('Please fill in all fields');
            return;
        }

        this.isExecuting = true;
        this.elements.executeBtn.disabled = true;
        this.elements.cancelBtn.style.display = 'inline-block';

        // Clear output
        this.clearOutput();

        try {
            // Get display name for terminal
            const builtinTaskDisplay = {
                'ping': 'Ping',
                'mtr': 'MTR',
                'nexttrace': 'NextTrace'
            };
            const displayName = builtinTaskDisplay[taskName] || taskName;

            // Show command in terminal
            this.appendToTerminal(`$ ${displayName} ${target}`, 'terminal-prompt');

            // Send execute request with task name
            console.log('Executing task:', { agentId, taskName, target });
            this.client.executeTask(agentId, taskName, target);

            // Add to history
            this.addToHistory(agentId, displayName, target);
        } catch (error) {
            console.error('Failed to execute command:', error);
            this.handleError(error.message);
        }
    }

    handleTaskStarted(taskId) {
        console.log('Task started:', taskId);
        this.client.currentTaskId = taskId;
    }

    handleOutput(output, error) {
        if (output) {
            this.appendToTerminal(output, 'terminal-output');
            this.appendToHistory(output);
        }
        if (error) {
            this.appendToTerminal(error, 'terminal-error');
            this.appendToHistory(error);
        }
    }

    // Convert ANSI escape codes to HTML with colors
    ansiToHtml(text) {
        // ANSI color mapping
        const colors = {
            // Regular colors
            '30': '#000000', '31': '#cd3131', '32': '#0dbc79', '33': '#e5e510',
            '34': '#2472c8', '35': '#bc3fbc', '36': '#11a8cd', '37': '#e5e5e5',
            // Bright colors
            '90': '#666666', '91': '#f14c4c', '92': '#23d18b', '93': '#f5f543',
            '94': '#3b8eea', '95': '#d670d6', '96': '#29b8db', '97': '#ffffff',
            // Background colors (optional, not commonly used in terminal output)
            '40': '#000000', '41': '#cd3131', '42': '#0dbc79', '43': '#e5e510',
            '44': '#2472c8', '45': '#bc3fbc', '46': '#11a8cd', '47': '#e5e5e5'
        };

        let result = '';
        let currentStyle = '';
        let buffer = '';

        // Split text by ESC sequences
        const parts = text.split(/(\x1b\[[0-9;]*m)/g);

        for (let i = 0; i < parts.length; i++) {
            const part = parts[i];

            // Check if this is an ANSI escape code
            const match = part.match(/\x1b\[([0-9;]*)m/);
            if (match) {
                // Close previous span if exists
                if (currentStyle) {
                    result += `<span style="${currentStyle}">${this.escapeHtml(buffer)}</span>`;
                    buffer = '';
                }

                const codes = match[1].split(';').filter(c => c !== '');

                if (codes.length === 0 || codes[0] === '0') {
                    // Reset
                    currentStyle = '';
                } else {
                    // Build style from codes
                    let styles = [];
                    for (const code of codes) {
                        if (code === '1') {
                            styles.push('font-weight: bold');
                        } else if (code === '3') {
                            styles.push('font-style: italic');
                        } else if (code === '4') {
                            styles.push('text-decoration: underline');
                        } else if (colors[code]) {
                            if (parseInt(code) >= 40 && parseInt(code) < 50) {
                                styles.push(`background-color: ${colors[code]}`);
                            } else {
                                styles.push(`color: ${colors[code]}`);
                            }
                        }
                    }
                    currentStyle = styles.join('; ');
                }
            } else {
                // Regular text
                buffer += part;
            }
        }

        // Flush remaining buffer
        if (buffer) {
            if (currentStyle) {
                result += `<span style="${currentStyle}">${this.escapeHtml(buffer)}</span>`;
            } else {
                result += this.escapeHtml(buffer);
            }
        }

        return result;
    }

    // Escape HTML special characters
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    handleComplete(message) {
        this.isExecuting = false;
        this.elements.executeBtn.disabled = false;
        this.elements.cancelBtn.style.display = 'none';

        if (message) {
            this.appendToTerminal(message, 'terminal-success');
        }
        // this.appendToTerminal('\n$ Command completed', 'terminal-prompt');

        // Mark history item as successful
        if (this.currentHistoryItem) {
            this.currentHistoryItem.success = true;
        }

        // Finalize history
        this.finalizeHistory();
    }

    handleError(error) {
        this.isExecuting = false;
        this.elements.executeBtn.disabled = false;
        this.elements.cancelBtn.style.display = 'none';

        this.appendToTerminal(`Error: ${error}`, 'terminal-error');
        this.appendToTerminal('\n$ Command failed', 'terminal-prompt');

        // Mark history item as failed
        if (this.currentHistoryItem) {
            this.currentHistoryItem.success = false;
        }

        // Finalize history
        this.finalizeHistory();
    }

    cancelExecution() {
        if (!this.isExecuting) {
            return;
        }

        this.client.cancelTask();
        this.appendToTerminal('\n^C (Cancelled)', 'terminal-error');
        this.handleComplete('Task cancelled by user');
    }

    appendToTerminal(text, className = '') {
        const terminal = this.elements.outputTerminal;
        const line = document.createElement('div');
        line.className = className;

        // Convert ANSI codes to HTML with colors
        const htmlText = this.ansiToHtml(text);
        line.innerHTML = htmlText;

        terminal.appendChild(line);

        // Auto-scroll to bottom
        terminal.scrollTop = terminal.scrollHeight;
    }

    clearOutput() {
        this.elements.outputTerminal.innerHTML = '';
    }

    showError(message) {
        this.appendToTerminal(`Error: ${message}`, 'terminal-error');
    }

    // History functions
    loadHistory() {
        const stored = localStorage.getItem('lookingglass_history');
        if (stored) {
            try {
                this.history = JSON.parse(stored);
            } catch (e) {
                this.history = [];
            }
        }
        this.renderHistory();
    }

    saveHistory() {
        localStorage.setItem('lookingglass_history', JSON.stringify(this.history));
    }

    addToHistory(agentId, tool, target) {
        const item = {
            timestamp: Date.now(),
            agentId,
            tool,
            target,
            output: '',  // Will be filled during execution
            expanded: false,
            success: null  // null = running, true = success, false = error
        };

        this.history.unshift(item);

        // Keep only last 50 items
        if (this.history.length > 50) {
            this.history = this.history.slice(0, 50);
        }

        // Store reference to current history item for output capture
        this.currentHistoryItem = this.history[0];

        this.saveHistory();
        this.renderHistory();
    }

    // Append output to current history item
    appendToHistory(text) {
        if (this.currentHistoryItem) {
            // Only add newline if text doesn't already end with one
            // This handles different executors: ping (no \n) vs nexttrace (has \n)
            if (text && !text.endsWith('\n')) {
                this.currentHistoryItem.output += text + '\n';
            } else {
                this.currentHistoryItem.output += text;
            }
        }
    }

    // Finalize current history item
    finalizeHistory() {
        if (this.currentHistoryItem) {
            this.saveHistory();
            this.renderHistory();
            this.currentHistoryItem = null;
        }
    }

    renderHistory() {
        const listDiv = this.elements.historyList;

        if (this.history.length === 0) {
            listDiv.innerHTML = '<p class="no-history">No history yet</p>';
            return;
        }

        listDiv.innerHTML = '';

        this.history.forEach((item, index) => {
            const historyItem = document.createElement('div');
            historyItem.className = 'history-item';

            const date = new Date(item.timestamp);
            const timeStr = date.toLocaleTimeString();

            // Status icon based on success/failure/running
            let statusIcon = '⏳';  // running
            if (item.success === true) {
                statusIcon = '✓';  // success
            } else if (item.success === false) {
                statusIcon = '✗';  // error
            }

            historyItem.innerHTML = `
                <div class="history-header">
                    <span class="history-status-icon">${statusIcon}</span>
                    <span class="history-tool-badge ${item.tool}">${item.tool}</span>
                    <span class="history-target">${item.target}</span>
                    <span class="history-time">${timeStr}</span>
                </div>
                <div class="history-output ${item.expanded ? '' : 'collapsed'}"></div>
            `;

            // Add click handler to header for toggling
            const header = historyItem.querySelector('.history-header');
            header.addEventListener('click', () => {
                this.toggleHistoryItem(index);
            });

            // Render output if expanded
            if (item.expanded && item.output) {
                const outputDiv = historyItem.querySelector('.history-output');
                const outputHtml = this.ansiToHtml(item.output);
                outputDiv.innerHTML = `<pre>${outputHtml}</pre>`;
            }

            listDiv.appendChild(historyItem);
        });
    }

    toggleHistoryItem(index) {
        this.history[index].expanded = !this.history[index].expanded;
        this.saveHistory();
        this.renderHistory();
    }

    replayFromHistory(item) {
        // Set form values from history item
        this.elements.agentSelect.value = item.agentId;
        this.handleAgentSelection();

        // Find task type ID from tool name
        const toolTypes = {
            'ping': '1',
            'mtr': '2',
            'nexttrace': '5'
        };
        const taskType = toolTypes[item.tool.toLowerCase()];
        if (taskType) {
            this.elements.toolSelect.value = taskType;
        }

        this.elements.targetInput.value = item.target;

        // Scroll to top
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }

}

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.app = new LookingGlassApp();
});
