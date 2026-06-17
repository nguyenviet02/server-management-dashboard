package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Collector gathers system and container metrics.
type Collector struct {
	logger *slog.Logger
}

// NewCollector creates a new Collector.
func NewCollector(logger *slog.Logger) *Collector {
	return &Collector{logger: logger}
}

// CollectSystem gathers system-level metrics using gopsutil.
func (c *Collector) CollectSystem() (*MetricSnapshot, error) {
	snap := &MetricSnapshot{}

	// CPU
	cpuPcts, err := cpu.Percent(0, false)
	if err == nil && len(cpuPcts) > 0 {
		snap.CPUPercent = cpuPcts[0]
	}

	// Load average
	loadAvg, err := load.Avg()
	if err == nil && loadAvg != nil {
		snap.LoadAvg1 = loadAvg.Load1
		snap.LoadAvg5 = loadAvg.Load5
		snap.LoadAvg15 = loadAvg.Load15
	}

	// Memory
	vmem, err := mem.VirtualMemory()
	if err == nil && vmem != nil {
		snap.MemTotal = vmem.Total
		snap.MemUsed = vmem.Used
		snap.MemPercent = vmem.UsedPercent
	}

	// Swap
	swap, err := mem.SwapMemory()
	if err == nil && swap != nil {
		snap.SwapTotal = swap.Total
		snap.SwapUsed = swap.Used
	}

	// Disk usage (root partition)
	diskUsage, err := disk.Usage("/")
	if err == nil && diskUsage != nil {
		snap.DiskTotal = diskUsage.Total
		snap.DiskUsed = diskUsage.Used
		snap.DiskPercent = diskUsage.UsedPercent
	}

	// Disk I/O (aggregate all disks)
	diskIO, err := disk.IOCounters()
	if err == nil {
		for _, d := range diskIO {
			snap.DiskReadBytes += d.ReadBytes
			snap.DiskWriteBytes += d.WriteBytes
		}
	}

	// Network I/O (aggregate all interfaces)
	netIO, err := net.IOCounters(false)
	if err == nil && len(netIO) > 0 {
		snap.NetRecvBytes = netIO[0].BytesRecv
		snap.NetSentBytes = netIO[0].BytesSent
	}

	// CPU Temperature via sensors
	temps, terr := sensors.SensorsTemperatures()
	if terr == nil {
		var maxTemp float64
		for _, t := range temps {
			s := strings.ToLower(t.SensorKey)
			if strings.Contains(s, "cpu") || strings.Contains(s, "core") || strings.Contains(s, "package") || strings.Contains(s, "k10temp") || strings.Contains(s, "coretemp") {
				if t.Temperature > maxTemp {
					maxTemp = t.Temperature
				}
			}
		}
		// Fallback: use the highest sensor reading if no CPU-specific sensors found
		if maxTemp == 0 && len(temps) > 0 {
			for _, t := range temps {
				if t.Temperature > maxTemp && t.Temperature < 120 {
					maxTemp = t.Temperature
				}
			}
		}
		snap.CpuTemp = maxTemp
	}

	// CPU Frequency percentage (current / max)
	cpuFreqs, ferr := cpu.Info()
	if ferr == nil && len(cpuFreqs) > 0 {
		var totalCurrent, totalMax float64
		for _, info := range cpuFreqs {
			totalCurrent += info.Mhz
			if info.Mhz > totalMax {
				totalMax = info.Mhz
			}
		}
		avgMhz := totalCurrent / float64(len(cpuFreqs))
		if totalMax > 0 {
			snap.CpuFreqPercent = (avgMhz / totalMax) * 100
		}
	}

	// Power / charging status — read from /sys/class/power_supply (Linux)
	snap.PowerPlugged = readPowerPlugged()

	return snap, nil
}

// readPowerPlugged checks /sys/class/power_supply for AC adapter or battery status.
// Returns true if plugged in (or on a system without battery like a server).
func readPowerPlugged() bool {
	dir := "/sys/class/power_supply"
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Not Linux or no power supply info → assume plugged in (server)
		return true
	}

	hasBattery := false
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		// Look for AC adapter entries first
		if strings.HasPrefix(name, "ac") || strings.Contains(name, "ac_adapter") || strings.Contains(name, "mains") {
			onlinePath := dir + "/" + e.Name() + "/online"
			data, rerr := os.ReadFile(onlinePath)
			if rerr == nil && strings.TrimSpace(string(data)) == "1" {
				return true
			}
		}
		if strings.HasPrefix(name, "bat") {
			hasBattery = true
			statusPath := dir + "/" + e.Name() + "/status"
			data, rerr := os.ReadFile(statusPath)
			if rerr == nil {
				status := strings.TrimSpace(strings.ToLower(string(data)))
				if status == "charging" || status == "full" {
					return true
				}
			}
		}
	}

	// If there's a battery but it's discharging, return false
	if hasBattery {
		return false
	}
	// No battery found → server/desktop, assume plugged in
	return true
}

// dockerStatsEntry represents one entry from `docker stats --no-stream --format json`.
type dockerStatsEntry struct {
	ID       string `json:"ID"`
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
}

// CollectContainers gathers per-container metrics via the Docker CLI.
func (c *Collector) CollectContainers() ([]ContainerMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker stats: %w", err)
	}

	var metrics []ContainerMetric
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		var entry dockerStatsEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			c.logger.Warn("parse docker stats line", "err", err, "line", line)
			continue
		}

		cm := ContainerMetric{
			ID:     entry.ID,
			Name:   strings.TrimPrefix(entry.Name, "/"),
			Status: "running",
		}

		// Parse CPU percentage (e.g. "0.50%")
		cm.CPUPercent = parsePercent(entry.CPUPerc)

		// Parse memory percentage (e.g. "1.23%")
		cm.MemPercent = parsePercent(entry.MemPerc)

		// Parse memory usage (e.g. "50MiB / 512MiB")
		parts := strings.Split(entry.MemUsage, "/")
		if len(parts) == 2 {
			cm.MemUsage = parseBytes(strings.TrimSpace(parts[0]))
			cm.MemLimit = parseBytes(strings.TrimSpace(parts[1]))
		}

		metrics = append(metrics, cm)
	}

	return metrics, nil
}

// parsePercent parses a percentage string like "1.23%" to a float64.
func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

// parseBytes parses a human-readable byte string like "50.5MiB" to uint64 bytes.
func parseBytes(s string) uint64 {
	s = strings.TrimSpace(s)
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	unit = strings.ToLower(unit)
	switch {
	case strings.HasPrefix(unit, "kib"), strings.HasPrefix(unit, "kb"):
		return uint64(val * 1024)
	case strings.HasPrefix(unit, "mib"), strings.HasPrefix(unit, "mb"):
		return uint64(val * 1024 * 1024)
	case strings.HasPrefix(unit, "gib"), strings.HasPrefix(unit, "gb"):
		return uint64(val * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "tib"), strings.HasPrefix(unit, "tb"):
		return uint64(val * 1024 * 1024 * 1024 * 1024)
	default:
		return uint64(val)
	}
}
