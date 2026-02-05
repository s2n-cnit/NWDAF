# NWDAF Web Dashboard - Quick Start Guide

## 🚀 Quick Start

### 1. Build (if needed)

```bash
cd /home/paolob/Nextcloud/Coding/GolandProjects/NWDAF
go build -o cmd/nwdaf/build/nwdaf ./cmd/nwdaf/main.go
```

### 2. Start NWDAF

```bash
./cmd/nwdaf/build/nwdaf
```

### 3. Open Dashboard

Open your browser to: **http://localhost:8080**

That's it! 🎉

## 📋 What You'll See

1. **Settings Panel** at the top
   - Adjust polling interval with slider (2-60 seconds)
   - Toggle auto-refresh on/off

2. **Status Bar** showing connection state

3. **Filter Boxes** to search for specific metrics

4. **Two Charts Side-by-Side**
   - Left: Raw metrics from collectors
   - Right: Forecasted metrics from analytics

5. **Data Tables** showing latest metric values

6. **Overview Statistics** at the bottom

## ⚙️ Configuration

### Change Server Port

Edit `config/config.yaml`:

```yaml
server:
  bindIP: "0.0.0.0"
  port: 8080  # Change this
```

### Adjust Polling Interval

Use the slider in the dashboard UI (no restart needed)

## 🔧 Troubleshooting

### "No data available" message

**Cause**: Collectors not running or no metrics being generated

**Solution**:
1. Start data collectors: `./cmd/dcollector/build/dcollector`
2. Ensure HPE_amf_collector or other collectors are active
3. Check Redis is running and accessible

### "Connection error" in status bar

**Cause**: API endpoints not accessible

**Solution**:
1. Verify Data Archiver is running (port 8081)
2. Verify Analytics Engine is running (port 8084)
3. Check logs for errors

### Charts not updating

**Cause**: Auto-refresh might be disabled

**Solution**:
- Check the auto-refresh checkbox in settings is ✅ checked
- Or click the pause/play icon in the navbar

## 📱 Mobile Access

Access from mobile device on same network:

```
http://<server-ip>:8080
```

Example: `http://192.168.1.100:8080`

## 🎯 Key Features

- **Polling**: 2-60 seconds (use slider)
- **Filter**: Type metric name in search boxes
- **Toggle**: Pause/resume with checkbox or icon
- **Clear**: Use "Clear Data" in settings menu
- **Responsive**: Works on all devices

## 📚 More Information

- Full documentation: `web/README.md`
- Implementation details: `web/IMPLEMENTATION_SUMMARY.md`
- API documentation: Main project README

## 💡 Tips

1. **Start with 5s polling** (default) - good balance
2. **Use filters** to focus on specific metrics
3. **Clear data** if charts get too crowded
4. **Mobile**: Works great on tablets/phones
5. **Multiple tabs**: Open multiple browser tabs for different views

Enjoy monitoring your NWDAF! 🎉📊
