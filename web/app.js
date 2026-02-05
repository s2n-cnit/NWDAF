// NWDAF Dashboard Application
class NWDAFDashboard {
    constructor() {
        this.pollingInterval = 5000; // 5 seconds default
        this.isPolling = true;
        this.metricsData = [];
        this.computedData = [];
        this.metricsChart = null;
        this.computedChart = null;
        this.pollTimer = null;
        this.metricsFilter = '';
        this.computedFilter = '';
        this.stats = {
            totalMetrics: 0,
            totalComputed: 0,
            uniqueMetrics: new Set()
        };
    }

    async init() {
        console.log('Initializing NWDAF Dashboard...');

        // Initialize Material components
        this.initMaterialize();

        // Setup event listeners
        this.setupEventListeners();

        // Initialize charts
        this.initCharts();

        // Start polling
        this.startPolling();
    }

    initMaterialize() {
        // Initialize dropdowns
        const dropdowns = document.querySelectorAll('.dropdown-trigger');
        M.Dropdown.init(dropdowns, {
            coverTrigger: false,
            constrainWidth: false
        });

        // Initialize tabs
        const tabs = document.querySelectorAll('.tabs');
        M.Tabs.init(tabs);

        // Initialize tooltips
        const tooltips = document.querySelectorAll('.tooltipped');
        M.Tooltip.init(tooltips);
    }

    setupEventListeners() {
        // Polling interval slider
        const slider = document.getElementById('pollingSlider');
        slider.addEventListener('input', (e) => {
            const value = parseInt(e.target.value);
            document.getElementById('pollingValue').textContent = value;
            this.setPollingInterval(value * 1000);
        });

        // Auto-refresh checkbox
        const checkbox = document.getElementById('autoRefreshCheckbox');
        checkbox.addEventListener('change', (e) => {
            this.isPolling = e.target.checked;
            if (this.isPolling) {
                this.startPolling();
                M.toast({html: 'Auto-refresh enabled', classes: 'green'});
            } else {
                this.stopPolling();
                M.toast({html: 'Auto-refresh paused', classes: 'orange'});
            }
        });

        // Refresh toggle (for mobile/icon)
        const refreshToggle = document.getElementById('refreshToggle');
        if (refreshToggle) {
            refreshToggle.addEventListener('click', (e) => {
                e.preventDefault();
                checkbox.checked = !checkbox.checked;
                checkbox.dispatchEvent(new Event('change'));
            });
        }

        // Metrics filter
        document.getElementById('metricsFilter').addEventListener('input', (e) => {
            this.metricsFilter = e.target.value.toLowerCase();
            this.applyFilters();
        });

        // Computed filter
        document.getElementById('computedFilter').addEventListener('input', (e) => {
            this.computedFilter = e.target.value.toLowerCase();
            this.applyFilters();
        });

        // Clear data
        document.getElementById('clear-data').addEventListener('click', () => this.clearData());
    }

    initCharts() {
        const chartConfig = {
            type: 'line',
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            unit: 'minute',
                            displayFormats: {
                                minute: 'HH:mm'
                            }
                        },
                        title: {
                            display: true,
                            text: 'Time'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'Value'
                        },
                        beginAtZero: true
                    }
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top'
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false
                    }
                },
                interaction: {
                    mode: 'nearest',
                    axis: 'x',
                    intersect: false
                }
            }
        };

        // Initialize metrics chart
        const metricsCtx = document.getElementById('metricsChart').getContext('2d');
        this.metricsChart = new Chart(metricsCtx, {
            ...chartConfig,
            data: { datasets: [] }
        });

        // Initialize computed metrics chart
        const computedCtx = document.getElementById('computedChart').getContext('2d');
        this.computedChart = new Chart(computedCtx, {
            ...chartConfig,
            data: { datasets: [] }
        });
    }

    setPollingInterval(interval) {
        this.pollingInterval = interval;

        if (this.isPolling) {
            this.stopPolling();
            this.startPolling();
        }
    }

    startPolling() {
        if (this.pollTimer) {
            clearInterval(this.pollTimer);
        }

        // Initial fetch
        this.fetchData();

        // Set up interval
        this.pollTimer = setInterval(() => {
            if (this.isPolling) {
                this.fetchData();
            }
        }, this.pollingInterval);
    }

    stopPolling() {
        if (this.pollTimer) {
            clearInterval(this.pollTimer);
            this.pollTimer = null;
        }
    }

    async fetchData() {
        this.updateStatus('loading', 'Fetching data...');

        try {
            // Fetch both endpoints
            const [metricsResponse, computedResponse] = await Promise.all([
                fetch('/api/metrics').catch(err => ({ ok: false, error: err })),
                fetch('/api/computed-metrics').catch(err => ({ ok: false, error: err }))
            ]);

            // Process metrics
            if (metricsResponse.ok) {
                const metricsText = await metricsResponse.text();
                if (metricsText && metricsText.trim()) {
                    try {
                        const metricsJson = JSON.parse(metricsText);
                        this.processMetrics(metricsJson);
                    } catch (parseErr) {
                        console.warn('Failed to parse metrics response:', parseErr);
                        this.handleEmptyResponse('metrics');
                    }
                } else {
                    this.handleEmptyResponse('metrics');
                }
            } else {
                this.handleEmptyResponse('metrics');
            }

            // Process computed metrics
            if (computedResponse.ok) {
                const computedText = await computedResponse.text();
                if (computedText && computedText.trim()) {
                    try {
                        const computedJson = JSON.parse(computedText);
                        this.processComputedMetrics(computedJson);
                    } catch (parseErr) {
                        console.warn('Failed to parse computed metrics response:', parseErr);
                        this.handleEmptyResponse('computed');
                    }
                } else {
                    this.handleEmptyResponse('computed');
                }
            } else {
                this.handleEmptyResponse('computed');
            }

            this.updateStatus('connected', 'Connected');
            this.updateLastUpdate();
        } catch (error) {
            console.error('Error fetching data:', error);
            this.updateStatus('disconnected', 'Connection error');
        }
    }

    handleEmptyResponse(type) {
        if (type === 'metrics') {
            document.getElementById('metricsEmpty').style.display = 'block';
            document.getElementById('metricsChartContainer').style.display = 'none';
        } else if (type === 'computed') {
            document.getElementById('computedEmpty').style.display = 'block';
            document.getElementById('computedChartContainer').style.display = 'none';
        }
    }

    processMetrics(data) {
        console.log('Raw metrics data received:', data);

        if (!data || (Array.isArray(data) && data.length === 0) || Object.keys(data).length === 0) {
            this.handleEmptyResponse('metrics');
            return;
        }

        // Show chart, hide empty message
        document.getElementById('metricsEmpty').style.display = 'none';
        document.getElementById('metricsChartContainer').style.display = 'block';

        const metrics = Array.isArray(data) ? data : Object.values(data).flat();

        if (metrics.length > 0) {
            console.log('Processed metrics array length:', metrics.length);
            console.log('Sample metric:', metrics[0]);
        }

        if (metrics.length === 0) {
            this.handleEmptyResponse('metrics');
            return;
        }

        // Update stats
        this.stats.totalMetrics += metrics.length;
        metrics.forEach(m => this.stats.uniqueMetrics.add(m.Name || m.name));

        // Group by metric name
        const grouped = this.groupByMetricName(metrics);

        // Update chart
        this.updateChart(this.metricsChart, grouped);

        // Update table
        this.updateMetricsTable(metrics);

        // Update overview
        this.updateOverview();
    }

    processComputedMetrics(data) {
        console.log('Computed metrics data received:', data);

        if (!data || (Array.isArray(data) && data.length === 0) || Object.keys(data).length === 0) {
            this.handleEmptyResponse('computed');
            return;
        }

        // Show chart, hide empty message
        document.getElementById('computedEmpty').style.display = 'none';
        document.getElementById('computedChartContainer').style.display = 'block';

        // API returns array of ComputedMetricResponse objects:
        // [
        //   {
        //     "pluginName": "SarimaNuePlugin",
        //     "metricsByName": {
        //       "NWDAF_CONN_DEV_forecasted_value": [ {...}, {...}, ... ]
        //     },
        //     "lastUpdateTime": "2026-02-05T10:00:00Z"
        //   }
        // ]

        let allComputedMetrics = [];
        let metricsByNameCombined = {};

        // Check if data is array of ComputedMetricResponse objects
        if (Array.isArray(data)) {
            data.forEach(response => {
                console.log('Processing plugin response:', response.pluginName);

                if (response.metricsByName && typeof response.metricsByName === 'object') {
                    // Merge metricsByName from all plugins
                    Object.keys(response.metricsByName).forEach(metricName => {
                        const metricList = response.metricsByName[metricName];

                        if (Array.isArray(metricList)) {
                            console.log(`Metric "${metricName}" has ${metricList.length} forecast points`);

                            // Add to combined metrics
                            if (!metricsByNameCombined[metricName]) {
                                metricsByNameCombined[metricName] = [];
                            }
                            metricsByNameCombined[metricName] = metricsByNameCombined[metricName].concat(metricList);

                            // Also flatten for table display
                            allComputedMetrics = allComputedMetrics.concat(metricList);
                        }
                    });
                }
            });
        }

        if (allComputedMetrics.length === 0) {
            console.log('No computed metrics found in response');
            this.handleEmptyResponse('computed');
            return;
        }

        console.log('Total computed metrics:', allComputedMetrics.length);
        console.log('Sample computed metric:', allComputedMetrics[0]);

        // Update stats
        this.stats.totalComputed += allComputedMetrics.length;

        // Group by metric name - pass the combined metricsByName object
        const grouped = this.groupComputedMetricsByName(metricsByNameCombined);

        // Update chart
        this.updateChart(this.computedChart, grouped);

        // Update table
        this.updateComputedTable(allComputedMetrics);

        // Update overview
        this.updateOverview();
    }

    groupByMetricName(metrics) {
        const grouped = {};

        metrics.forEach(metric => {
            // Try all possible field name variations
            const name = metric.Name || metric.name || metric.metric_name || metric.metricName ||
                        (metric.description && metric.description.includes('metric') ? metric.description : null) ||
                        'Unknown';

            // Log the metric structure for debugging if name is Unknown
            if (name === 'Unknown') {
                console.warn('Metric with unknown name:', metric);
            }

            if (!grouped[name]) {
                grouped[name] = [];
            }

            // Try all possible timestamp fields
            const timestamp = metric.ReceivedAt || metric.received_at || metric.timestamp ||
                            metric.Timestamp || metric.time || new Date().toISOString();

            // Try all possible value fields
            const value = parseFloat(
                metric.Value !== undefined ? metric.Value :
                metric.value !== undefined ? metric.value :
                metric.val !== undefined ? metric.val : 0
            );

            grouped[name].push({
                x: new Date(timestamp),
                y: isNaN(value) ? 0 : value
            });
        });

        // Sort by timestamp and keep last 50 points per metric
        Object.keys(grouped).forEach(key => {
            grouped[key].sort((a, b) => a.x - b.x);
            if (grouped[key].length > 50) {
                grouped[key] = grouped[key].slice(-50);
            }
        });

        return grouped;
    }

    groupComputedMetricsByName(data) {
        const grouped = {};

        // Computed metrics come as an object where:
        // - Keys are metric names (e.g., "NWDAF_CONN_DEV_forecasted_value")
        // - Values are arrays of forecast points for different future times

        if (!Array.isArray(data) && typeof data === 'object') {
            // Process each metric name
            Object.keys(data).forEach(metricName => {
                const forecastPoints = data[metricName];

                if (!Array.isArray(forecastPoints)) {
                    return;
                }

                // Use the metric name as the key (already in the object structure)
                // or extract from the first metric if needed
                const name = metricName ||
                           (forecastPoints.length > 0 ?
                            (forecastPoints[0].Name || forecastPoints[0].name || 'Unknown') :
                            'Unknown');

                if (!grouped[name]) {
                    grouped[name] = [];
                }

                // Add all forecast points for this metric
                forecastPoints.forEach(point => {
                    const timestamp = point.ReceivedAt || point.receivedAt || point.timestamp ||
                                    point.Timestamp || point.time || new Date().toISOString();

                    const value = parseFloat(
                        point.Value !== undefined ? point.Value :
                        point.value !== undefined ? point.value :
                        point.val !== undefined ? point.val : 0
                    );

                    grouped[name].push({
                        x: new Date(timestamp),
                        y: isNaN(value) ? 0 : value
                    });
                });
            });
        } else {
            // Fallback to standard grouping if it's an array
            return this.groupByMetricName(Array.isArray(data) ? data : []);
        }

        // Sort by timestamp
        Object.keys(grouped).forEach(key => {
            grouped[key].sort((a, b) => a.x - b.x);
            // For forecasts, we might want to keep more points (they're future predictions)
            if (grouped[key].length > 100) {
                grouped[key] = grouped[key].slice(-100);
            }
        });

        return grouped;
    }

    applyFilters() {
        // Filter metrics chart
        if (this.metricsChart && this.metricsChart.data.datasets) {
            this.metricsChart.data.datasets.forEach(dataset => {
                if (this.metricsFilter) {
                    dataset.hidden = !dataset.label.toLowerCase().includes(this.metricsFilter);
                } else {
                    dataset.hidden = false;
                }
            });
            this.metricsChart.update('none');
        }

        // Filter computed chart
        if (this.computedChart && this.computedChart.data.datasets) {
            this.computedChart.data.datasets.forEach(dataset => {
                if (this.computedFilter) {
                    dataset.hidden = !dataset.label.toLowerCase().includes(this.computedFilter);
                } else {
                    dataset.hidden = false;
                }
            });
            this.computedChart.update('none');
        }
    }

    updateChart(chart, groupedData) {
        const colors = [
            '#1976d2', '#388e3c', '#d32f2f', '#f57c00', '#7b1fa2',
            '#0097a7', '#c2185b', '#5d4037', '#455a64', '#e64a19'
        ];

        const datasets = Object.keys(groupedData).map((name, index) => ({
            label: name,
            data: groupedData[name],
            borderColor: colors[index % colors.length],
            backgroundColor: colors[index % colors.length] + '20',
            borderWidth: 2,
            fill: true,
            tension: 0.4,
            pointRadius: 3,
            pointHoverRadius: 5
        }));

        chart.data.datasets = datasets;
        chart.update('none'); // Update without animation for smoother updates
    }

    updateMetricsTable(metrics) {
        const tbody = document.getElementById('metricsTableBody');

        // Get last 10 metrics
        const latest = metrics.slice(-10).reverse();

        if (latest.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="center-align grey-text">No data</td></tr>';
            return;
        }

        tbody.innerHTML = latest.map(metric => {
            const name = metric.Name || metric.name || metric.metric_name || metric.metricName || 'N/A';
            const value = metric.Value !== undefined ? metric.Value :
                         metric.value !== undefined ? metric.value : 'N/A';
            const nfType = metric.nfType || 'N/A';
            const timestamp = metric.receivedAt;

            return `
                <tr>
                    <td><strong>${this.escapeHtml(name)}</strong></td>
                    <td>${this.formatValue(value)}</td>
                    <td>${this.escapeHtml(nfType)}</td>
                    <td>${this.formatTimestamp(timestamp)}</td>
                </tr>
            `;
        }).join('');
    }

    updateComputedTable(computed) {
        const tbody = document.getElementById('computedTableBody');

        // Get last 10 computed metrics
        const latest = computed.slice(-10).reverse();

        if (latest.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="center-align grey-text">No data</td></tr>';
            return;
        }

        tbody.innerHTML = latest.map(metric => {
            const name = metric.Name || metric.name || metric.metric_name || metric.metricName || 'N/A';
            const value = metric.Value !== undefined ? metric.Value :
                         metric.value !== undefined ? metric.value : 'N/A';
            const nfType = metric.nfType || 'N/A';
            const timestamp = metric.receivedAt || 'N/A';

            return `
                <tr>
                    <td><strong>${this.escapeHtml(name)}</strong></td>
                    <td>${this.formatValue(value)}</td>
                    <td>${this.escapeHtml(nfType)}</td>
                    <td>${this.formatTimestamp(timestamp)}</td>
                </tr>
            `;
        }).join('');
    }

    updateOverview() {
        document.getElementById('totalMetrics').textContent = this.stats.totalMetrics;
        document.getElementById('totalComputed').textContent = this.stats.totalComputed;
        document.getElementById('uniqueMetrics').textContent = this.stats.uniqueMetrics.size;

        // Update active metrics list
        const activeList = document.getElementById('activeMetricsList');
        if (this.stats.uniqueMetrics.size > 0) {
            activeList.innerHTML = Array.from(this.stats.uniqueMetrics)
                .sort()
                .map(name => `
                    <div class="collection-item">
                        <i class="material-icons left">show_chart</i>
                        ${this.escapeHtml(name)}
                    </div>
                `).join('');
        } else {
            activeList.innerHTML = '<div class="collection-item center-align grey-text">No active metrics</div>';
        }
    }

    updateStatus(status, text) {
        const statusText = document.getElementById('statusText');
        const icon = statusText.querySelector('i');

        icon.className = `material-icons tiny status-${status}`;
        statusText.childNodes[1].textContent = text;
    }

    updateLastUpdate() {
        const now = new Date();
        document.getElementById('lastUpdate').textContent =
            `Last update: ${now.toLocaleTimeString()}`;
    }

    clearData() {
        this.stats.totalMetrics = 0;
        this.stats.totalComputed = 0;
        this.stats.uniqueMetrics.clear();

        // Clear charts
        this.metricsChart.data.datasets = [];
        this.metricsChart.update();

        this.computedChart.data.datasets = [];
        this.computedChart.update();

        // Clear tables
        document.getElementById('metricsTableBody').innerHTML =
            '<tr><td colspan="4" class="center-align grey-text">No data</td></tr>';
        document.getElementById('computedTableBody').innerHTML =
            '<tr><td colspan="4" class="center-align grey-text">No data</td></tr>';

        this.updateOverview();

        M.toast({html: 'Data cleared', classes: 'blue'});
    }

    formatValue(value) {
        if (value === undefined || value === null) return 'N/A';
        const num = parseFloat(value);
        if (isNaN(num)) return String(value);
        return num.toFixed(2);
    }

    formatTimestamp(timestamp) {
        if (!timestamp) return 'N/A';
        try {
            const date = new Date(timestamp);
            return date.toLocaleString();
        } catch (e) {
            return String(timestamp);
        }
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = String(text);
        return div.innerHTML;
    }
}

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    const dashboard = new NWDAFDashboard();
    dashboard.init();
});
