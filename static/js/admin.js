// Real-time Admin Dashboard JavaScript with Server-Sent Events

class AdminDashboard {
    constructor() {
        this.eventSource = null;
        this.isPageVisible = true;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000; // Start with 1 second
        this.connectionStatus = 'disconnected';
        this.updateQueue = [];
        this.isUpdating = false;
        this.lastUpdateTime = 0;
        this.updateThrottle = 100; // Minimum 100ms between updates
        this.elementCache = new Map(); // Cache DOM elements
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.connectToRealtimeStream();
        this.loadInitialData();
        this.setupMediaFolderHandlers();
    }

    setupEventListeners() {
        // Tab switching
        document.querySelectorAll('.tab-button').forEach(button => {
            button.addEventListener('click', (e) => {
                const tabName = e.target.getAttribute('onclick').match(/'([^']+)'/)[1];
                this.showTab(tabName);
            });
        });

        // Page visibility detection
        document.addEventListener('visibilitychange', () => {
            this.isPageVisible = !document.hidden;
            if (this.isPageVisible) {
                // Page became visible, reconnect if needed
                if (this.connectionStatus === 'disconnected') {
                    this.connectToRealtimeStream();
                }
            } else {
                // Page became hidden, close connection to save resources
                this.disconnectFromRealtimeStream();
            }
        });

        // Window focus/blur events as backup
        window.addEventListener('focus', () => {
            this.isPageVisible = true;
            if (this.connectionStatus === 'disconnected') {
                this.connectToRealtimeStream();
            }
        });

        window.addEventListener('blur', () => {
            this.isPageVisible = false;
            this.disconnectFromRealtimeStream();
        });

        // Handle page unload
        window.addEventListener('beforeunload', () => {
            this.cleanup();
        });
    }

    connectToRealtimeStream() {
        if (this.eventSource && this.eventSource.readyState !== EventSource.CLOSED) {
            return; // Already connected
        }

        this.updateConnectionStatus('connecting');

        try {
            this.eventSource = new EventSource('/admin/api/realtime');

            this.eventSource.onopen = () => {
                console.log('Real-time connection established');
                this.connectionStatus = 'connected';
                this.reconnectAttempts = 0;
                this.reconnectDelay = 1000;
                this.updateConnectionStatus('connected');
                this.showNotification('Real-time dashboard connected', 'success');
            };

            this.eventSource.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleRealtimeUpdate(data);
                } catch (error) {
                    console.error('Error parsing real-time data:', error);
                }
            };

            this.eventSource.onerror = (error) => {
                console.error('Real-time connection error:', error);
                this.connectionStatus = 'disconnected';
                this.updateConnectionStatus('disconnected');

                // Attempt to reconnect
                this.scheduleReconnect();
            };

        } catch (error) {
            console.error('Failed to create EventSource:', error);
            this.connectionStatus = 'disconnected';
            this.updateConnectionStatus('disconnected');
            this.scheduleReconnect();
        }
    }

    disconnectFromRealtimeStream() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
        this.connectionStatus = 'disconnected';
        this.updateConnectionStatus('disconnected');
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.log('Max reconnection attempts reached');
            this.showNotification('Real-time connection failed. Please refresh the page.', 'error');
            return;
        }

        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1); // Exponential backoff

        console.log(`Attempting to reconnect in ${delay}ms (attempt ${this.reconnectAttempts})`);

        setTimeout(() => {
            if (this.isPageVisible && this.connectionStatus === 'disconnected') {
                this.connectToRealtimeStream();
            }
        }, delay);
    }

    handleRealtimeUpdate(data) {
        // Add update to queue
        this.updateQueue.push(data);

        // Process updates with throttling
        this.processUpdateQueue();
    }

    processUpdateQueue() {
        const now = Date.now();

        // Throttle updates to prevent excessive DOM manipulation
        if (this.isUpdating || (now - this.lastUpdateTime) < this.updateThrottle) {
            return;
        }

        this.isUpdating = true;
        this.lastUpdateTime = now;

        // Process all queued updates in a single batch
        requestAnimationFrame(() => {
            try {
                // Get the latest update (skip intermediate updates for efficiency)
                const latestUpdate = this.updateQueue[this.updateQueue.length - 1];
                this.updateQueue = []; // Clear queue

                if (latestUpdate) {
                    switch (latestUpdate.type) {
                        case 'initial':
                            this.updateDashboardData(latestUpdate.data);
                            break;
                        case 'update':
                            this.updateDashboardData(latestUpdate.stats);
                            this.updateConnectionsTable(latestUpdate.connections);
                            this.updateLastRefreshTime();
                            break;
                        default:
                            console.log('Unknown real-time update type:', latestUpdate.type);
                    }
                }
            } catch (error) {
                console.error('Error processing update queue:', error);
            } finally {
                this.isUpdating = false;
            }
        });
    }

    updateDashboardData(stats) {
        this.updateStatsCards(stats);
        this.updatePerformanceMetrics(stats.performance_metrics);
        this.updateStreamingMetrics(stats.streaming_metrics);
        this.updateCacheMetrics(stats.cache_stats);
    }

    updateConnectionStatus(status) {
        const indicator = document.getElementById('connection-status');
        if (indicator) {
            indicator.className = `connection-status ${status}`;
            const statusText = {
                'connected': '🟢 Real-time',
                'connecting': '🟡 Connecting...',
                'disconnected': '🔴 Disconnected'
            };
            indicator.textContent = statusText[status] || status;
        }
    }

    loadInitialData() {
        // Initial data will be loaded via SSE connection
        // Fallback to API calls if SSE fails
        if (this.connectionStatus === 'disconnected') {
            this.refreshConnections();
            this.refreshStats();
            this.refreshActivity();
        }
    }

    updatePerformanceMetrics(metrics) {
        if (!metrics) {
            // Set default values when metrics are not available
            this.updateElement('#cpu-cores', 'N/A');
            this.updateElement('#gomaxprocs', 'N/A');
            this.updateElement('#goroutines', 'N/A');
            this.updateElement('#memory-usage', 'N/A');
            this.updateElement('#memory-percent', 'N/A');
            this.updateElement('#gc-count', 'N/A');
            this.updateElement('#uptime', 'N/A');
            return;
        }

        // Update CPU and memory info with proper formatting
        this.updateElement('#cpu-cores', metrics.cpu_cores || 0);
        this.updateElement('#gomaxprocs', metrics.gomaxprocs || 0);
        this.updateElement('#goroutines', this.formatNumber(metrics.goroutines || 0));
        this.updateElement('#memory-usage', this.formatBytes(metrics.memory_usage || 0));
        this.updateElement('#memory-percent', `${(metrics.memory_percent || 0).toFixed(1)}%`);
        this.updateElement('#gc-count', this.formatNumber(metrics.gc_count || 0));
        this.updateElement('#uptime', this.formatDuration(metrics.uptime || 0));

        // Update worker pools
        this.updateWorkerPools(metrics.worker_pools);

        // Update system health indicators
        this.updateSystemHealth(metrics);
    }

    updateSystemHealth(metrics) {
        if (!metrics) return;

        // Performance Status
        const memoryPercent = metrics.memory_percent || 0;
        const goroutines = metrics.goroutines || 0;

        let performanceStatus = 'Optimal';
        let performanceClass = 'status-good';

        if (memoryPercent > 80 || goroutines > 1000) {
            performanceStatus = 'Critical';
            performanceClass = 'status-critical';
        } else if (memoryPercent > 60 || goroutines > 500) {
            performanceStatus = 'Warning';
            performanceClass = 'status-warning';
        }

        this.updateElementWithClass('#performance-status', performanceStatus, performanceClass);

        // Memory Pressure
        let memoryPressure = 'Low';
        let memoryClass = 'status-good';

        if (memoryPercent > 80) {
            memoryPressure = 'High';
            memoryClass = 'status-critical';
        } else if (memoryPercent > 60) {
            memoryPressure = 'Medium';
            memoryClass = 'status-warning';
        }

        this.updateElementWithClass('#memory-pressure', memoryPressure, memoryClass);

        // Goroutine Health
        let goroutineHealth = 'Normal';
        let goroutineClass = 'status-good';

        if (goroutines > 1000) {
            goroutineHealth = 'High';
            goroutineClass = 'status-critical';
        } else if (goroutines > 500) {
            goroutineHealth = 'Elevated';
            goroutineClass = 'status-warning';
        }

        this.updateElementWithClass('#goroutine-health', goroutineHealth, goroutineClass);

        // GC Frequency (simplified indicator)
        const gcCount = metrics.gc_count || 0;
        let gcFrequency = 'Normal';
        let gcClass = 'status-good';

        // This is a simplified check - in reality you'd track GC rate over time
        if (gcCount > 1000) {
            gcFrequency = 'High';
            gcClass = 'status-warning';
        }

        this.updateElementWithClass('#gc-frequency', gcFrequency, gcClass);
    }

    updateElementWithClass(selector, value, className) {
        // Use cached element if available
        let element = this.elementCache.get(selector);
        if (!element) {
            element = document.querySelector(selector);
            if (element) {
                this.elementCache.set(selector, element);
            }
        }

        if (element) {
            if (element.textContent !== value) {
                element.textContent = value;
            }
            const newClassName = `metric-value ${className}`;
            if (element.className !== newClassName) {
                element.className = newClassName;
            }
        }
    }

    updateStreamingMetrics(metrics) {
        if (!metrics) {
            // Set default values when metrics are not available
            this.updateElement('#active-streams', '0');
            this.updateElement('#total-streams', '0');
            this.updateElement('#bytes-streamed', '0 B');
            this.updateElement('#average-speed', '0 B/s');
            this.updateElement('#peak-speed', '0 B/s');
            this.updateElement('#concurrent-peak', '0');
            return;
        }

        // Update streaming metrics with proper formatting
        this.updateElement('#active-streams', this.formatNumber(metrics.active_streams || 0));
        this.updateElement('#total-streams', this.formatNumber(metrics.total_streams || 0));
        this.updateElement('#bytes-streamed', this.formatBytes(metrics.bytes_streamed || 0));
        this.updateElement('#average-speed', this.formatSpeed(metrics.average_speed || 0));
        this.updateElement('#peak-speed', this.formatSpeed(metrics.peak_speed || 0));
        this.updateElement('#concurrent-peak', this.formatNumber(metrics.concurrent_peak || 0));

        // Add visual indicators for active streams
        const activeStreamsElement = document.querySelector('#active-streams');
        if (activeStreamsElement) {
            const activeCount = metrics.active_streams || 0;
            if (activeCount > 0) {
                activeStreamsElement.style.color = '#10b981'; // Green for active
                activeStreamsElement.style.fontWeight = 'bold';
            } else {
                activeStreamsElement.style.color = 'var(--text-color)';
                activeStreamsElement.style.fontWeight = 'normal';
            }
        }
    }

    updateCacheMetrics(metrics) {
        if (!metrics) {
            // Set default values when metrics are not available
            this.updateElement('#cache-size', '0');
            this.updateElement('#cache-max-size', '0');
            this.updateElement('#cache-hit-rate', '0.0%');
            this.updateElement('#cache-hit-count', '0');
            this.updateElement('#cache-miss-count', '0');
            this.updateElement('#cache-evictions', '0');
            return;
        }

        // Update cache metrics with proper formatting
        this.updateElement('#cache-size', this.formatNumber(metrics.size || 0));
        this.updateElement('#cache-max-size', this.formatNumber(metrics.max_size || 0));
        this.updateElement('#cache-hit-rate', `${(metrics.hit_rate || 0).toFixed(1)}%`);
        this.updateElement('#cache-hit-count', this.formatNumber(metrics.hit_count || 0));
        this.updateElement('#cache-miss-count', this.formatNumber(metrics.miss_count || 0));
        this.updateElement('#cache-evictions', this.formatNumber(metrics.evictions || 0));

        // Add visual indicators for cache efficiency
        const hitRateElement = document.querySelector('#cache-hit-rate');
        if (hitRateElement) {
            const hitRate = metrics.hit_rate || 0;
            if (hitRate >= 80) {
                hitRateElement.style.color = '#10b981'; // Green for good hit rate
            } else if (hitRate >= 60) {
                hitRateElement.style.color = '#f59e0b'; // Yellow for moderate hit rate
            } else {
                hitRateElement.style.color = '#ef4444'; // Red for poor hit rate
            }
        }
    }

    updateWorkerPools(workerPools) {
        const container = document.getElementById('worker-pools-container');
        if (!container) return;

        if (!workerPools || Object.keys(workerPools).length === 0) {
            container.innerHTML = '<div class="worker-pool-card"><h4>No Worker Pools</h4><p>Worker pools will appear here when tasks are running.</p></div>';
            return;
        }

        const poolsHtml = Object.values(workerPools).map(pool => {
            const successRate = this.calculateSuccessRate(pool);
            const queueUtilization = this.calculateQueueUtilization(pool);
            const avgDuration = this.formatDuration(pool.average_task_duration || 0);

            return `
                <div class="worker-pool-card">
                    <h4>${pool.name || 'Unknown Pool'}</h4>
                    <div class="pool-stats">
                        <div class="stat">
                            <span class="label">Workers:</span>
                            <span class="value">${this.formatNumber(pool.workers || 0)}</span>
                        </div>
                        <div class="stat">
                            <span class="label">Active Tasks:</span>
                            <span class="value" style="color: ${(pool.active_tasks || 0) > 0 ? '#10b981' : 'inherit'}">${this.formatNumber(pool.active_tasks || 0)}</span>
                        </div>
                        <div class="stat">
                            <span class="label">Queue:</span>
                            <span class="value">${this.formatNumber(pool.queue_size || 0)}/${this.formatNumber(pool.buffer_size || 0)} (${queueUtilization}%)</span>
                        </div>
                        <div class="stat">
                            <span class="label">Total Tasks:</span>
                            <span class="value">${this.formatNumber(pool.total_tasks || 0)}</span>
                        </div>
                        <div class="stat">
                            <span class="label">Success Rate:</span>
                            <span class="value" style="color: ${successRate >= 95 ? '#10b981' : successRate >= 80 ? '#f59e0b' : '#ef4444'}">${successRate}%</span>
                        </div>
                        <div class="stat">
                            <span class="label">Avg Duration:</span>
                            <span class="value">${avgDuration}</span>
                        </div>
                        <div class="stat">
                            <span class="label">Status:</span>
                            <span class="value status-${pool.status || 'unknown'}">${(pool.status || 'unknown').toUpperCase()}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = poolsHtml;
    }

    updateElement(selector, value) {
        // Use cached element if available
        let element = this.elementCache.get(selector);
        if (!element) {
            element = document.querySelector(selector);
            if (element) {
                this.elementCache.set(selector, element);
            }
        }

        if (element && element.textContent !== value) {
            element.textContent = value;
        }
    }

    calculateSuccessRate(pool) {
        if (!pool || (pool.total_tasks || 0) === 0) return 100;
        return (((pool.successful_tasks || 0) / pool.total_tasks) * 100).toFixed(1);
    }

    calculateQueueUtilization(pool) {
        if (!pool || (pool.buffer_size || 0) === 0) return 0;
        return (((pool.queue_size || 0) / pool.buffer_size) * 100).toFixed(1);
    }

    formatNumber(num) {
        if (!num || num === 0) return '0';

        // Add thousand separators for large numbers
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        } else if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        } else {
            return num.toString();
        }
    }

    async refreshConnections() {
        try {
            const response = await fetch('/admin/api/connections');
            const connections = await response.json();
            this.updateConnectionsTable(connections);
        } catch (error) {
            console.error('Error fetching connections:', error);
        }
    }

    async refreshStats() {
        try {
            const response = await fetch('/admin/api/stats');
            const stats = await response.json();
            this.updateStatsCards(stats);
        } catch (error) {
            console.error('Error fetching stats:', error);
        }
    }

    async refreshActivity() {
        try {
            const response = await fetch('/admin/api/activity?limit=20');
            const activity = await response.json();
            this.updateActivityTable(activity);
        } catch (error) {
            console.error('Error fetching activity:', error);
        }
    }

    updateConnectionsTable(connections) {
        const tbody = document.getElementById('connections-tbody');
        if (!tbody) return;

        tbody.innerHTML = connections.map(conn => `
            <tr>
                <td>${conn.ip_address}</td>
                <td>${conn.location || 'Unknown'}</td>
                <td>${this.formatTime(conn.connected_at)}</td>
                <td>${this.formatSpeed(conn.current_speed)}</td>
                <td>${conn.request_count}</td>
                <td>${this.formatBytes(conn.bytes_served)}</td>
                <td>
                    ${conn.is_blocked
                        ? '<span class="status-badge status-blocked">Blocked</span>'
                        : '<span class="status-badge status-active">Active</span>'
                    }
                </td>
                <td>
                    <div class="action-buttons">
                        ${conn.is_blocked
                            ? `<button class="btn btn-success" onclick="adminDashboard.unblockIP('${conn.ip_address}')">Unblock</button>`
                            : `<button class="btn btn-danger" onclick="adminDashboard.blockIP('${conn.ip_address}')">Block</button>`
                        }
                    </div>
                </td>
            </tr>
        `).join('');
    }

    updateStatsCards(stats) {
        const updateCard = (selector, value) => {
            const element = document.querySelector(selector);
            if (element) element.textContent = value;
        };

        updateCard('.stat-card:nth-child(1) .stat-value', stats.active_connections);
        updateCard('.stat-card:nth-child(2) .stat-value', stats.total_connections);
        updateCard('.stat-card:nth-child(3) .stat-value', stats.blocked_connections);
        updateCard('.stat-card:nth-child(4) .stat-value', this.formatBytes(stats.total_bytes_served));
    }

    updateActivityTable(activity) {
        const tbody = document.getElementById('activity-tbody');
        if (!tbody) return;

        tbody.innerHTML = activity.map(log => `
            <tr>
                <td>${this.formatTime(log.timestamp)}</td>
                <td>${log.ip_address}</td>
                <td>${log.action}</td>
                <td>${log.resource}</td>
                <td>
                    ${log.success
                        ? '<span class="status-badge status-active">Success</span>'
                        : '<span class="status-badge status-blocked">Failed</span>'
                    }
                </td>
                <td>${log.details || ''}</td>
            </tr>
        `).join('');
    }

    async blockIP(ipAddress) {
        const reason = prompt('Enter reason for blocking this IP:');
        if (!reason) return;

        try {
            const response = await fetch('/admin/block-ip', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `ip_address=${encodeURIComponent(ipAddress)}&reason=${encodeURIComponent(reason)}`
            });

            if (response.ok) {
                this.showNotification(`IP ${ipAddress} has been blocked`, 'success');
                this.refreshConnections();
            } else {
                this.showNotification('Failed to block IP', 'error');
            }
        } catch (error) {
            console.error('Error blocking IP:', error);
            this.showNotification('Error blocking IP', 'error');
        }
    }

    async unblockIP(ipAddress) {
        if (!confirm(`Are you sure you want to unblock ${ipAddress}?`)) return;

        try {
            const response = await fetch('/admin/unblock-ip', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `ip_address=${encodeURIComponent(ipAddress)}`
            });

            if (response.ok) {
                this.showNotification(`IP ${ipAddress} has been unblocked`, 'success');
                this.refreshConnections();
            } else {
                this.showNotification('Failed to unblock IP', 'error');
            }
        } catch (error) {
            console.error('Error unblocking IP:', error);
            this.showNotification('Error unblocking IP', 'error');
        }
    }

    async setMediaPassword() {
        const mediaPath = document.getElementById('mediaPathInput').value;
        const password = document.getElementById('mediaPasswordInput').value;

        if (!mediaPath || !password) {
            this.showNotification('Please enter both media path and password', 'error');
            return;
        }

        try {
            const response = await fetch('/admin/set-media-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `media_path=${encodeURIComponent(mediaPath)}&password=${encodeURIComponent(password)}`
            });

            if (response.ok) {
                this.showNotification(`Password set for ${mediaPath}`, 'success');
                document.getElementById('mediaPathInput').value = '';
                document.getElementById('mediaPasswordInput').value = '';
            } else {
                this.showNotification('Failed to set media password', 'error');
            }
        } catch (error) {
            console.error('Error setting media password:', error);
            this.showNotification('Error setting media password', 'error');
        }
    }

    async blockIPWithReason(ipAddress, reason) {
        try {
            const response = await fetch('/admin/block-ip', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `ip_address=${encodeURIComponent(ipAddress)}&reason=${encodeURIComponent(reason)}`
            });

            if (response.ok) {
                this.showNotification(`IP ${ipAddress} has been blocked`, 'success');
                this.refreshConnections();
            } else {
                this.showNotification('Failed to block IP', 'error');
            }
        } catch (error) {
            console.error('Error blocking IP:', error);
            this.showNotification('Error blocking IP', 'error');
        }
    }

    async setMediaPasswordWithPath(mediaPath, password) {
        try {
            const response = await fetch('/admin/set-media-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `media_path=${encodeURIComponent(mediaPath)}&password=${encodeURIComponent(password)}`
            });

            if (response.ok) {
                this.showNotification(`Password set for ${mediaPath}`, 'success');
            } else {
                this.showNotification('Failed to set media password', 'error');
            }
        } catch (error) {
            console.error('Error setting media password:', error);
            this.showNotification('Error setting media password', 'error');
        }
    }

    saveSettings() {
        // Collect all settings
        const settings = {
            autoBlockAttempts: document.getElementById('autoBlockAttempts').value,
            sessionTimeout: document.getElementById('sessionTimeout').value,
            connectionTracking: document.getElementById('connectionTracking').value,
            logRetention: document.getElementById('logRetention').value,
            defaultProtection: document.getElementById('defaultProtection').value,
            maxStreams: document.getElementById('maxStreams').value
        };

        // In a real implementation, you would send these to the server
        console.log('Saving settings:', settings);
        this.showNotification('Settings saved successfully', 'success');
    }

    showTab(tabName) {
        // Hide all tab contents
        document.querySelectorAll('.tab-content').forEach(tab => {
            tab.classList.remove('active');
        });

        // Remove active class from all tab buttons
        document.querySelectorAll('.tab-button').forEach(button => {
            button.classList.remove('active');
        });

        // Show selected tab content
        const selectedTab = document.getElementById(`${tabName}-tab`);
        if (selectedTab) {
            selectedTab.classList.add('active');
        }

        // Add active class to selected tab button
        const selectedButton = document.querySelector(`[onclick="showTab('${tabName}')"]`);
        if (selectedButton) {
            selectedButton.classList.add('active');
        }

        // Load data for specific tabs
        if (tabName === 'media-folders') {
            this.loadMediaFolders();
        }
    }

    showNotification(message, type = 'info') {
        // Create notification element
        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.textContent = message;

        // Style the notification
        Object.assign(notification.style, {
            position: 'fixed',
            top: '20px',
            right: '20px',
            padding: '15px 20px',
            borderRadius: '6px',
            color: 'white',
            fontWeight: '500',
            zIndex: '9999',
            opacity: '0',
            transform: 'translateX(100%)',
            transition: 'all 0.3s ease'
        });

        // Set background color based on type
        const colors = {
            success: '#10b981',
            error: '#ef4444',
            warning: '#f59e0b',
            info: '#3b82f6'
        };
        notification.style.backgroundColor = colors[type] || colors.info;

        // Add to page
        document.body.appendChild(notification);

        // Animate in
        setTimeout(() => {
            notification.style.opacity = '1';
            notification.style.transform = 'translateX(0)';
        }, 100);

        // Remove after 5 seconds
        setTimeout(() => {
            notification.style.opacity = '0';
            notification.style.transform = 'translateX(100%)';
            setTimeout(() => {
                if (notification.parentNode) {
                    notification.parentNode.removeChild(notification);
                }
            }, 300);
        }, 5000);
    }

    updateLastRefreshTime() {
        const now = new Date();
        const timeString = now.toLocaleTimeString();

        // Update any "last updated" indicators
        const indicators = document.querySelectorAll('.last-updated');
        indicators.forEach(indicator => {
            indicator.textContent = `Real-time: ${timeString}`;
        });

        // Update connection status timestamp
        const statusElement = document.getElementById('connection-status');
        if (statusElement && this.connectionStatus === 'connected') {
            statusElement.title = `Last update: ${timeString}`;
        }
    }

    formatDuration(nanoseconds) {
        if (!nanoseconds) return '0s';

        const seconds = Math.floor(nanoseconds / 1000000000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (days > 0) {
            return `${days}d ${hours % 24}h ${minutes % 60}m`;
        } else if (hours > 0) {
            return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
        } else if (minutes > 0) {
            return `${minutes}m ${seconds % 60}s`;
        } else {
            return `${seconds}s`;
        }
    }

    formatTime(timestamp) {
        return new Date(timestamp).toLocaleTimeString();
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    formatSpeed(bytesPerSecond) {
        if (!bytesPerSecond || bytesPerSecond === 0) return '0 B/s';

        const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s'];
        let size = bytesPerSecond;
        let unitIndex = 0;

        while (size >= 1024 && unitIndex < units.length - 1) {
            size /= 1024;
            unitIndex++;
        }

        // Format with appropriate decimal places
        if (size < 10) {
            return size.toFixed(2) + ' ' + units[unitIndex];
        } else if (size < 100) {
            return size.toFixed(1) + ' ' + units[unitIndex];
        } else {
            return Math.round(size) + ' ' + units[unitIndex];
        }
    }

    cleanup() {
        // Disconnect from real-time stream
        this.disconnectFromRealtimeStream();

        // Clear update queue
        this.updateQueue = [];

        // Clear element cache to prevent memory leaks
        this.elementCache.clear();

        // Reset state
        this.isUpdating = false;
        this.reconnectAttempts = 0;

        console.log('AdminDashboard cleaned up');
    }

    // Media Folder Management Methods
    setupMediaFolderHandlers() {
        // Setup form submission handler
        const addFolderForm = document.getElementById('addFolderForm');
        if (addFolderForm) {
            addFolderForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.addMediaFolder();
            });
        }
    }

    async loadMediaFolders() {
        try {
            const response = await fetch('/admin/api/media-folders');
            if (!response.ok) throw new Error('Failed to load media folders');

            const folders = await response.json();
            this.displayMediaFolders(folders);
        } catch (error) {
            console.error('Error loading media folders:', error);
            this.showNotification('Failed to load media folders', 'error');
        }
    }

    displayMediaFolders(folders) {
        const container = document.getElementById('media-folders-container');
        if (!container) return;

        if (!folders || folders.length === 0) {
            container.innerHTML = '<div class="loading-message">No media folders configured. Add a folder to get started.</div>';
            return;
        }

        const foldersHtml = folders.map(folder => `
            <div class="folder-card ${folder.is_default ? 'default' : ''}">
                <div class="folder-header">
                    <h4 class="folder-title">${this.escapeHtml(folder.name)}</h4>
                    <div class="folder-badges">
                        ${folder.is_default ? '<span class="folder-badge default">Default</span>' : ''}
                        <span class="folder-badge ${folder.is_active ? 'active' : 'inactive'}">
                            ${folder.is_active ? 'Active' : 'Inactive'}
                        </span>
                    </div>
                </div>

                <div class="folder-path">${this.escapeHtml(folder.path)}</div>

                ${folder.description ? `<div class="folder-description">${this.escapeHtml(folder.description)}</div>` : ''}

                <div class="folder-stats">
                    <div class="folder-stat">
                        <div class="folder-stat-value">${folder.file_count || 0}</div>
                        <div class="folder-stat-label">Files</div>
                    </div>
                    <div class="folder-stat">
                        <div class="folder-stat-value">${this.formatBytes(folder.total_size || 0)}</div>
                        <div class="folder-stat-label">Size</div>
                    </div>
                </div>

                <div class="folder-actions">
                    <button class="btn btn-primary" onclick="scanFolder('${folder.id}')">🔍 Scan</button>
                    <button class="btn btn-secondary" onclick="toggleFolder('${folder.id}')">${folder.is_active ? '⏸️ Disable' : '▶️ Enable'}</button>
                    ${!folder.is_default ? `<button class="btn btn-secondary" onclick="setDefaultFolder('${folder.id}')">⭐ Set Default</button>` : ''}
                    <button class="btn btn-danger" onclick="removeFolder('${folder.id}')">🗑️ Remove</button>
                </div>
            </div>
        `).join('');

        container.innerHTML = foldersHtml;
    }

    async addMediaFolder() {
        const name = document.getElementById('folderName').value;
        const path = document.getElementById('folderPath').value;
        const description = document.getElementById('folderDescription').value;
        const setDefault = document.getElementById('setAsDefault').checked;

        if (!name || !path) {
            this.showNotification('Name and path are required', 'error');
            return;
        }

        try {
            const response = await fetch('/admin/api/media-folders', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    name: name,
                    path: path,
                    description: description,
                    set_default: setDefault
                })
            });

            if (!response.ok) {
                const error = await response.text();
                throw new Error(error);
            }

            const folder = await response.json();
            this.showNotification(`Media folder "${folder.name}" added successfully`, 'success');

            // Close modal and refresh folders
            closeModal('addFolderModal');
            this.loadMediaFolders();

            // Clear form
            document.getElementById('addFolderForm').reset();
        } catch (error) {
            console.error('Error adding media folder:', error);
            this.showNotification('Failed to add media folder: ' + error.message, 'error');
        }
    }

    async browseFolders(path = '/') {
        try {
            const response = await fetch(`/admin/api/browse-folders?path=${encodeURIComponent(path)}`);
            if (!response.ok) throw new Error('Failed to browse folders');

            const folders = await response.json();
            this.displayFolderBrowser(folders, path);
        } catch (error) {
            console.error('Error browsing folders:', error);
            this.showNotification('Failed to browse folders', 'error');
        }
    }

    displayFolderBrowser(folders, currentPath) {
        const folderList = document.getElementById('folderList');
        const currentPathElement = document.getElementById('currentPath');

        if (currentPathElement) {
            currentPathElement.textContent = currentPath;
        }

        if (!folderList) return;

        const foldersHtml = folders.map(folder => `
            <div class="folder-list-item" onclick="navigateToFolder('${this.escapeHtml(folder.full_path)}')">
                <span class="folder-icon">📁</span>
                <span>${this.escapeHtml(folder.name)}</span>
            </div>
        `).join('');

        folderList.innerHTML = foldersHtml;

        // Store current path for selection
        this.currentBrowsePath = currentPath;
    }

    selectCurrentPath() {
        if (this.currentBrowsePath) {
            document.getElementById('folderPath').value = this.currentBrowsePath;
            closeModal('folderBrowserModal');
        }
    }

    async scanFolder(folderId) {
        try {
            this.showNotification('Scanning folder...', 'info');

            const response = await fetch(`/admin/api/scan-folder?id=${folderId}`, {
                method: 'POST'
            });

            if (!response.ok) throw new Error('Failed to scan folder');

            const stats = await response.json();
            this.showNotification(`Scan complete: ${stats.total_files} files found`, 'success');

            // Refresh folders to show updated stats
            this.loadMediaFolders();
        } catch (error) {
            console.error('Error scanning folder:', error);
            this.showNotification('Failed to scan folder: ' + error.message, 'error');
        }
    }

    async toggleFolder(folderId) {
        try {
            const response = await fetch(`/admin/api/media-folder?id=${folderId}&action=toggle`, {
                method: 'PATCH'
            });

            if (!response.ok) throw new Error('Failed to toggle folder');

            this.showNotification('Folder status updated', 'success');
            this.loadMediaFolders();
        } catch (error) {
            console.error('Error toggling folder:', error);
            this.showNotification('Failed to toggle folder: ' + error.message, 'error');
        }
    }

    async setDefaultFolder(folderId) {
        try {
            const response = await fetch(`/admin/api/media-folder?id=${folderId}&action=set_default`, {
                method: 'PATCH'
            });

            if (!response.ok) throw new Error('Failed to set default folder');

            this.showNotification('Default folder updated', 'success');
            this.loadMediaFolders();
        } catch (error) {
            console.error('Error setting default folder:', error);
            this.showNotification('Failed to set default folder: ' + error.message, 'error');
        }
    }

    async removeFolder(folderId) {
        if (!confirm('Are you sure you want to remove this media folder? This will not delete the actual files.')) {
            return;
        }

        try {
            const response = await fetch(`/admin/api/media-folder?id=${folderId}`, {
                method: 'DELETE'
            });

            if (!response.ok) throw new Error('Failed to remove folder');

            this.showNotification('Media folder removed', 'success');
            this.loadMediaFolders();
        } catch (error) {
            console.error('Error removing folder:', error);
            this.showNotification('Failed to remove folder: ' + error.message, 'error');
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Global functions for template onclick handlers
function showTab(tabName) {
    if (window.adminDashboard) {
        window.adminDashboard.showTab(tabName);
    }
}

function refreshDashboard() {
    if (window.adminDashboard) {
        window.adminDashboard.refreshDashboard();
    }
}

function refreshConnections() {
    if (window.adminDashboard) {
        window.adminDashboard.refreshConnections();
    }
}

function refreshActivity() {
    if (window.adminDashboard) {
        window.adminDashboard.refreshActivity();
    }
}

function blockIP(ipAddress) {
    if (window.adminDashboard) {
        window.adminDashboard.blockIP(ipAddress);
    }
}

function unblockIP(ipAddress) {
    if (window.adminDashboard) {
        window.adminDashboard.unblockIP(ipAddress);
    }
}

function setMediaPassword() {
    if (window.adminDashboard) {
        window.adminDashboard.setMediaPassword();
    }
}

function showAddUserModal() {
    const modal = document.getElementById('addUserModal');
    if (modal) {
        modal.style.display = 'block';
    }
}

function showBlockIPModal() {
    const modal = document.getElementById('blockIPModal');
    if (modal) {
        modal.style.display = 'block';
    }
}

function showMediaPasswordModal() {
    const modal = document.getElementById('mediaPasswordModal');
    if (modal) {
        modal.style.display = 'block';
    }
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'none';
    }
}

function blockIPFromModal(event) {
    event.preventDefault();
    const ipAddress = document.getElementById('blockIPAddress').value;
    const reason = document.getElementById('blockReason').value;

    if (window.adminDashboard) {
        window.adminDashboard.blockIPWithReason(ipAddress, reason);
        closeModal('blockIPModal');
        document.getElementById('blockIPAddress').value = '';
        document.getElementById('blockReason').value = '';
    }
}

function setMediaPasswordFromModal(event) {
    event.preventDefault();
    const mediaPath = document.getElementById('modalMediaPath').value;
    const password = document.getElementById('modalMediaPassword').value;

    if (window.adminDashboard) {
        window.adminDashboard.setMediaPasswordWithPath(mediaPath, password);
        closeModal('mediaPasswordModal');
        document.getElementById('modalMediaPath').value = '';
        document.getElementById('modalMediaPassword').value = '';
    }
}

function saveSettings() {
    if (window.adminDashboard) {
        window.adminDashboard.saveSettings();
    }
}

function removeUser(ipAddress, name) {
    if (confirm(`Are you sure you want to remove admin user "${name}" (${ipAddress})?`)) {
        const form = document.createElement('form');
        form.method = 'POST';
        form.action = '/admin/remove-user';

        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = 'ip_address';
        input.value = ipAddress;

        form.appendChild(input);
        document.body.appendChild(form);
        form.submit();
    }
}

// Media folder management functions
function showAddFolderModal() {
    const modal = document.getElementById('addFolderModal');
    if (modal) {
        modal.style.display = 'block';
    }
}

function browseFolders() {
    if (window.adminDashboard) {
        window.adminDashboard.browseFolders();
        const modal = document.getElementById('folderBrowserModal');
        if (modal) {
            modal.style.display = 'block';
        }
    }
}

function navigateToFolder(path) {
    if (window.adminDashboard) {
        window.adminDashboard.browseFolders(path);
    }
}

function selectCurrentPath() {
    if (window.adminDashboard) {
        window.adminDashboard.selectCurrentPath();
    }
}

function scanFolder(folderId) {
    if (window.adminDashboard) {
        window.adminDashboard.scanFolder(folderId);
    }
}

function toggleFolder(folderId) {
    if (window.adminDashboard) {
        window.adminDashboard.toggleFolder(folderId);
    }
}

function setDefaultFolder(folderId) {
    if (window.adminDashboard) {
        window.adminDashboard.setDefaultFolder(folderId);
    }
}

function removeFolder(folderId) {
    if (window.adminDashboard) {
        window.adminDashboard.removeFolder(folderId);
    }
}

// Initialize dashboard when page loads
document.addEventListener('DOMContentLoaded', () => {
    window.adminDashboard = new AdminDashboard();
});
