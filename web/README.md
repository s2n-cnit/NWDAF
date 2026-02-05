# NWDAF Web Dashboard

A Material Design web interface for monitoring and visualizing NWDAF metrics in real-time.

## Features

### ✨ Key Capabilities

- **Real-time Monitoring**: Live data streaming from NWDAF APIs
- **Side-by-Side Visualization**: Raw metrics and computed/forecasted metrics displayed simultaneously
- **Interactive Line Charts**: Time-series visualization using Chart.js
- **Configurable Polling**: Adjustable refresh interval from 2 to 60 seconds using a slider
- **Metric Filtering**: Search and filter metrics by name in real-time
- **Toggleable Auto-refresh**: Pause and resume data updates
- **Empty State Handling**: Graceful display when no data is available
- **Responsive Design**: Works on desktop, tablet, and mobile devices
- **Material Design**: Clean, modern UI following Material Design principles

### 📊 Display Sections

1. **Settings Panel**
   - Polling interval slider (2-60 seconds, default: 5 seconds)
   - Auto-refresh toggle checkbox

2. **Status Bar**
   - Connection status (Connected/Loading/Disconnected)
   - Last update timestamp

3. **Metric Filters**
   - Search/filter raw metrics by name
   - Search/filter computed metrics by name

4. **Side-by-Side Charts**
   - **Left**: Raw metrics from collectors (HPE AMF, Free5GC, etc.)
   - **Right**: Computed/forecasted metrics from analytics plugins

5. **Data Tables**
   - Latest 10 raw metrics with details
   - Latest 10 computed metrics with details

6. **Overview Statistics**
   - Total metrics received
   - Total forecasts generated
   - Unique metric types
   - Active metrics list

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Web Browser                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              NWDAF Web Dashboard (SPA)                │  │
│  │  - Chart.js for visualizations                        │  │
│  │  - Materialize CSS for Material Design               │  │
│  │  - Polling every 2-60 seconds (configurable)         │  │
│  └────────────────┬─────────────────┬───────────────────┘  │
└───────────────────┼─────────────────┼──────────────────────┘
                    │                 │
                    │ HTTP GET        │ HTTP GET
                    │                 │
        ┌───────────▼─────┐   ┌───────▼────────────┐
        │  /api/metrics   │   │ /api/computed-     │
        │                 │   │      metrics       │
        │ (Data Archiver) │   │ (Analytics Engine) │
        └─────────────────┘   └────────────────────┘
```

## Files

- **`index.html`**: Main HTML structure
- **`styles.css`**: Material Design styles and responsive layout
- **`app.js`**: JavaScript application logic
- **`libs/`**: Local copies of all dependencies (no CDN required)
  - `css/` - Materialize CSS, Material Icons, Roboto font
  - `js/` - Chart.js, date adapter, Materialize JS
  - `fonts/` - Material Icons and Roboto font files

## Configuration

### Polling Interval

Default: **5 seconds**  
Range: **2-60 seconds**  
Adjustable via slider in the Settings panel

### Auto-refresh

Default: **Enabled**  
Toggle via checkbox in Settings panel or nav icon

### API Endpoints

The dashboard polls these endpoints:

- **`/api/metrics`**: Raw metrics from data collectors
- **`/api/computed-metrics`**: Forecasted metrics from analytics engine

## Usage

### Development

The web UI is embedded in the main NWDAF executable using Go's `embed` package.

### Building

```bash
cd /path/to/NWDAF
go build -o cmd/nwdaf/build/nwdaf ./cmd/nwdaf/main.go
```

The web files are automatically embedded during build.

### Running

```bash
./cmd/nwdaf/build/nwdaf
```

Then open your browser to: `http://localhost:8080` (or configured port)

### Configuration

The web dashboard is served on the same port as the NWDAF HTTP API, configured in `config.yaml`:

```yaml
server:
  bindIP: "0.0.0.0"
  port: 8080
```

## API Response Handling

The dashboard gracefully handles:

- **Empty responses**: Shows "No data available" message
- **Invalid JSON**: Logs warning and displays empty state
- **Connection errors**: Updates status indicator
- **No data**: Displays appropriate empty states

### Expected API Response Format

```json
{
  "metric_name": [
    {
      "Name": "NWDAF_CONN_DEV",
      "Value": 42.5,
      "NFType": "AMF",
      "NFid": "hpe-amf-001",
      "ReceivedAt": "2026-02-05T10:30:00Z"
    }
  ]
}
```

Or as a flat array:

```json
[
  {
    "Name": "NWDAF_cpu_usage_percent",
    "Value": 45.2,
    "NFType": "AMF",
    "ReceivedAt": "2026-02-05T10:30:00Z"
  }
]
```

## Features in Detail

### Chart Display

- **Max 50 points** per metric (sliding window)
- **10 color palette** for multiple metrics
- **Time-based X-axis** with automatic formatting
- **Tooltip** on hover showing exact values
- **Legend** for metric identification
- **Smooth animations** with bezier curves

### Filtering

Type in the filter boxes to:
- Hide/show specific metrics on charts
- Filter happens client-side (no API calls)
- Case-insensitive substring matching

### Empty State

When no data is available:
- Charts hidden, friendly message displayed
- Icon and explanatory text shown
- Polling continues in background

### Responsive Design

- **Desktop**: Full side-by-side layout
- **Tablet**: Stacked charts with optimized spacing
- **Mobile**: Single column, optimized for small screens

## Browser Compatibility

- Chrome/Edge (recommended)
- Firefox
- Safari
- Any modern browser with ES6+ support

## Dependencies

### Local Libraries (Included)

All dependencies are included locally in the `libs/` directory:

- **Chart.js 4.4.1**: Time-series charting - [MIT License](libs/LICENSES/Chart.js-LICENSE.txt)
- **Materialize CSS 1.0.0**: Material Design framework - [MIT License](libs/LICENSES/Materialize-LICENSE.txt)
- **Material Icons**: Google's icon font - [Apache 2.0](libs/LICENSES/Material-Icons-LICENSE.txt)
- **Roboto Font**: Material Design typography - [Apache 2.0](libs/LICENSES/Roboto-Font-LICENSE.txt)

**No CDN or external dependencies required** - works completely offline!

See [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt) for detailed attribution and [LICENSE_COMPLIANCE.md](LICENSE_COMPLIANCE.md) for licensing information.

## Troubleshooting

### Dashboard shows "Connection error"

- Check that NWDAF main service is running
- Verify API endpoints are accessible
- Check browser console for errors

### No data displayed

- Verify collectors are running and publishing metrics
- Check Redis connectivity
- Ensure Data Archiver is operational
- Look at network tab in browser dev tools

### Charts not updating

- Check auto-refresh is enabled (checkbox)
- Verify polling interval is reasonable (not too high)
- Check browser console for JavaScript errors

### Slow performance

- Increase polling interval (reduce frequency)
- Clear accumulated data using "Clear Data" button
- Check if too many metrics are being collected

## Future Enhancements

Possible improvements:
- [ ] Export data as CSV/JSON
- [ ] Customizable time ranges
- [ ] Metric comparison views
- [ ] Alert thresholds and notifications
- [ ] Dark mode
- [ ] Historical data playback
- [ ] Metric aggregation options
- [ ] Custom dashboard layouts
