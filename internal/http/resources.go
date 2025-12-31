package http

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type ResourcesSnapshot struct {
	HostIP    string         `json:"hostIp"`
	UpdatedAt int64          `json:"updatedAt"`
	CPU       CPUStats       `json:"cpu"`
	Memory    MemoryStats    `json:"memory"`
	Disks     []DiskStats    `json:"disks"`
	GPUs      []GPUStats     `json:"gpus,omitempty"`
	Processes int            `json:"processes"`
	History   []HistoryPoint `json:"history,omitempty"`
	Errors    SnapshotError  `json:"errors"`
}

type SnapshotError struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disks  string `json:"disks"`
	GPUs   string `json:"gpus"`
	HostIP string `json:"hostIp"`
}

type CPUStats struct {
	Percent             float64  `json:"percent"`
	Model               string   `json:"model"`
	PhysicalCores       int      `json:"physicalCores"`
	LogicalCores        int      `json:"logicalCores"`
	CurrentMHz          float64  `json:"currentMHz"`
	MaxMHz              float64  `json:"maxMHz"`
	CurrentPercentOfMax float64  `json:"currentPercentOfMax"`
	TemperatureC        *float64 `json:"temperatureC,omitempty"`
	PerformanceCores    int      `json:"performanceCores"`
	EfficiencyCores     int      `json:"efficiencyCores"`
	PerformanceThreads  int      `json:"performanceThreads"`
	EfficiencyThreads   int      `json:"efficiencyThreads"`
}

type MemoryStats struct {
	TotalBytes      uint64             `json:"totalBytes"`
	UsedBytes       uint64             `json:"usedBytes"`
	UsedPercent     float64            `json:"usedPercent"`
	SwapTotalBytes  uint64             `json:"swapTotalBytes"`
	SwapUsedBytes   uint64             `json:"swapUsedBytes"`
	SwapUsedPercent float64            `json:"swapUsedPercent"`
	Modules         []MemoryModuleInfo `json:"modules,omitempty"`
}

type MemoryModuleInfo struct {
	Label     string `json:"label"`
	Vendor    string `json:"vendor"`
	SizeBytes uint64 `json:"sizeBytes"`
}

type DiskStats struct {
	Mountpoint  string  `json:"mountpoint"`
	Device      string  `json:"device"`
	Filesystem  string  `json:"filesystem"`
	DriveType   string  `json:"driveType"`
	Model       string  `json:"model"`
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

type GPUStats struct {
	Index              int      `json:"index"`
	Name               string   `json:"name"`
	Vendor             string   `json:"vendor"`
	Driver             string   `json:"driver"`
	UtilizationPercent *float64 `json:"utilizationPercent,omitempty"`
	MemoryTotalBytes   *uint64  `json:"memoryTotalBytes,omitempty"`
	MemoryUsedBytes    *uint64  `json:"memoryUsedBytes,omitempty"`
	TemperatureC       *float64 `json:"temperatureC,omitempty"`
}

type HistoryPoint struct {
	Time  int64              `json:"time"`
	CPU   float64            `json:"cpu"`
	Mem   float64            `json:"mem"`
	Disks map[string]float64 `json:"disks,omitempty"`
}

type diskMeta struct {
	DriveType         string
	StorageController string
	Model             string
}

type ResourceMonitor struct {
	mu       sync.RWMutex
	snapshot ResourcesSnapshot

	// CPU percent is derived from deltas between successive samples.
	prevTotal   float64
	prevIdle    float64
	havePrevCPU bool

	memoryModules       []MemoryModuleInfo
	memoryModulesLoaded bool

	diskMeta          map[string]diskMeta
	diskMetaUpdatedAt time.Time

	gpuMeta          []GPUStats
	gpuMetaUpdatedAt time.Time

	hostIP          string
	hostIPUpdatedAt time.Time
	hostIPErr       error

	cpuStatic          CPUStats
	cpuStaticUpdatedAt time.Time
	cpuStaticErr       error

	cpuDynamic          CPUStats
	cpuDynamicUpdatedAt time.Time
	cpuDynamicErr       error

	disksCache     []DiskStats
	disksUpdatedAt time.Time
	disksErr       error

	gpusCache     []GPUStats
	gpusUpdatedAt time.Time
	gpusErr       error

	history []HistoryPoint
}

func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		snapshot: ResourcesSnapshot{
			CPU:    CPUStats{Percent: 0},
			Memory: MemoryStats{},
			Disks:  nil,
			Errors: SnapshotError{},
		},
	}
}

func (m *ResourceMonitor) Start(stop <-chan struct{}) {
	m.update()
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.update()
			}
		}
	}()
}

func (m *ResourceMonitor) Snapshot(includeHistory bool) ResourcesSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := m.snapshot
	snap.Disks = append([]DiskStats(nil), m.snapshot.Disks...)
	snap.GPUs = append([]GPUStats(nil), m.snapshot.GPUs...)
	if includeHistory {
		snap.History = cloneHistory(m.history)
	}
	return snap
}

func (m *ResourceMonitor) update() {
	now := time.Now()
	var errs SnapshotError

	if m.hostIP == "" || now.Sub(m.hostIPUpdatedAt) >= hostIPTTL {
		m.hostIP, m.hostIPErr = preferredHostIP()
		m.hostIPUpdatedAt = now
	}
	if m.hostIPErr != nil {
		errs.HostIP = m.hostIPErr.Error()
	}

	cpuPercent, cpuPercentErr := m.sampleCPUPercent()

	if m.cpuStaticUpdatedAt.IsZero() || now.Sub(m.cpuStaticUpdatedAt) >= cpuStaticTTL {
		m.cpuStatic, m.cpuStaticErr = sampleCPUStaticInfo()
		m.cpuStaticUpdatedAt = now
	}

	cpuDynTTL := cpuDynamicTTLOther
	if runtime.GOOS == "linux" {
		cpuDynTTL = cpuDynamicTTLLinux
	}
	if m.cpuDynamicUpdatedAt.IsZero() || now.Sub(m.cpuDynamicUpdatedAt) >= cpuDynTTL {
		m.cpuDynamic, m.cpuDynamicErr = sampleCPUDynamicInfo()
		m.cpuDynamicUpdatedAt = now
	}

	cpuStats := CPUStats{
		Percent:             cpuPercent,
		Model:               m.cpuStatic.Model,
		PhysicalCores:       m.cpuStatic.PhysicalCores,
		LogicalCores:        m.cpuStatic.LogicalCores,
		CurrentMHz:          m.cpuDynamic.CurrentMHz,
		MaxMHz:              m.cpuDynamic.MaxMHz,
		CurrentPercentOfMax: m.cpuDynamic.CurrentPercentOfMax,
		TemperatureC:        m.cpuDynamic.TemperatureC,
		PerformanceCores:    m.cpuDynamic.PerformanceCores,
		EfficiencyCores:     m.cpuDynamic.EfficiencyCores,
		PerformanceThreads:  m.cpuDynamic.PerformanceThreads,
		EfficiencyThreads:   m.cpuDynamic.EfficiencyThreads,
	}

	var cpuErrs []string
	if cpuPercentErr != nil {
		cpuErrs = append(cpuErrs, cpuPercentErr.Error())
	}
	if m.cpuStaticErr != nil {
		cpuErrs = append(cpuErrs, m.cpuStaticErr.Error())
	}
	if m.cpuDynamicErr != nil {
		cpuErrs = append(cpuErrs, m.cpuDynamicErr.Error())
	}
	if len(cpuErrs) > 0 {
		errs.CPU = strings.Join(cpuErrs, "; ")
	}

	memStats, err := m.sampleMemory()
	if err != nil {
		errs.Memory = err.Error()
	}

	if m.disksUpdatedAt.IsZero() || now.Sub(m.disksUpdatedAt) >= disksSampleTTL {
		disks, err := m.sampleDisks()
		if disks != nil || err == nil {
			m.disksCache = disks
		}
		m.disksErr = err
		m.disksUpdatedAt = now
	}
	if m.disksErr != nil {
		errs.Disks = m.disksErr.Error()
	}

	if m.gpusUpdatedAt.IsZero() || now.Sub(m.gpusUpdatedAt) >= gpusSampleTTL {
		gpus, err := m.sampleGPUs()
		if gpus != nil || err == nil {
			m.gpusCache = gpus
		}
		m.gpusErr = err
		m.gpusUpdatedAt = now
	}
	if m.gpusErr != nil {
		errs.GPUs = m.gpusErr.Error()
	}

	procCount, procErr := sampleProcessCount()
	if procErr != nil {
		errs.CPU = strings.TrimSpace(strings.Join([]string{errs.CPU, fmt.Sprintf("processes: %v", procErr)}, "; "))
	}

	snap := ResourcesSnapshot{
		HostIP:    m.hostIP,
		UpdatedAt: now.UnixMilli(),
		CPU:       cpuStats,
		Memory:    memStats,
		Disks:     m.disksCache,
		GPUs:      m.gpusCache,
		Processes: procCount,
		Errors:    errs,
	}

	m.mu.Lock()
	m.snapshot = snap
	m.appendHistoryLocked(snap)
	m.mu.Unlock()
}

func (m *ResourceMonitor) appendHistoryLocked(snap ResourcesSnapshot) {
	hp := HistoryPoint{
		Time: snap.UpdatedAt,
		CPU:  snap.CPU.Percent,
		Mem:  snap.Memory.UsedPercent,
	}

	for _, d := range snap.Disks {
		if d.Mountpoint == "" {
			continue
		}
		if hp.Disks == nil {
			hp.Disks = make(map[string]float64)
		}
		hp.Disks[d.Mountpoint] = d.UsedPercent
	}

	m.history = append(m.history, hp)

	cutoff := snap.UpdatedAt - int64(historyMaxAge/time.Millisecond)
	trim := 0
	for trim < len(m.history) && m.history[trim].Time < cutoff {
		trim++
	}
	if trim > 0 {
		m.history = append([]HistoryPoint(nil), m.history[trim:]...)
	}
	if len(m.history) > historyMaxPoints {
		m.history = append([]HistoryPoint(nil), m.history[len(m.history)-historyMaxPoints:]...)
	}
}

func (m *ResourceMonitor) sampleCPUPercent() (float64, error) {
	times, err := cpu.Times(false)
	if err != nil {
		return 0, err
	}
	if len(times) == 0 {
		return 0, nil
	}

	t := times[0]
	total := t.Total()
	idle := t.Idle + t.Iowait

	if !m.havePrevCPU {
		m.prevTotal = total
		m.prevIdle = idle
		m.havePrevCPU = true
		return 0, nil
	}

	totalDelta := total - m.prevTotal
	idleDelta := idle - m.prevIdle

	m.prevTotal = total
	m.prevIdle = idle

	if totalDelta <= 0 {
		return 0, nil
	}

	busyDelta := totalDelta - idleDelta
	if busyDelta < 0 {
		busyDelta = 0
	}

	percent := busyDelta / totalDelta * 100
	if percent < 0 {
		return 0, nil
	}
	if percent > 100 {
		return 100, nil
	}
	return percent, nil
}

type cpuFreqSummary struct {
	CurrentMHz         float64
	MaxMHz             float64
	PerformanceCores   int
	EfficiencyCores    int
	PerformanceThreads int
	EfficiencyThreads  int
}

func sampleCPUStaticInfo() (CPUStats, error) {
	stats := CPUStats{}
	var warnings []string

	info, err := cpu.Info()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("cpu info: %v", err))
	} else if len(info) > 0 {
		stats.Model = strings.TrimSpace(info[0].ModelName)
	}

	physical, err := cpu.Counts(false)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("cpu physical cores: %v", err))
	} else {
		stats.PhysicalCores = physical
	}

	logical, err := cpu.Counts(true)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("cpu logical cores: %v", err))
	} else {
		stats.LogicalCores = logical
	}

	if len(warnings) > 0 {
		return stats, fmt.Errorf("%s", strings.Join(warnings, "; "))
	}
	return stats, nil
}

func sampleCPUDynamicInfo() (CPUStats, error) {
	stats := CPUStats{}
	var warnings []string

	switch runtime.GOOS {
	case "linux":
		freq, err := linuxCPUFreqSummary()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("cpu freq: %v", err))
		} else {
			stats.CurrentMHz = freq.CurrentMHz
			stats.MaxMHz = freq.MaxMHz
			if stats.MaxMHz > 0 && stats.CurrentMHz > 0 {
				stats.CurrentPercentOfMax = stats.CurrentMHz / stats.MaxMHz * 100
			}
			stats.PerformanceCores = freq.PerformanceCores
			stats.EfficiencyCores = freq.EfficiencyCores
			stats.PerformanceThreads = freq.PerformanceThreads
			stats.EfficiencyThreads = freq.EfficiencyThreads
		}
	default:
		info, err := cpu.Info()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("cpu info: %v", err))
		} else {
			var sumMHz float64
			var n int
			for _, i := range info {
				if i.Mhz <= 0 {
					continue
				}
				sumMHz += i.Mhz
				n++
			}
			if n > 0 {
				stats.CurrentMHz = sumMHz / float64(n)
			}
		}
	}

	tempC, err := sampleCPUTemperatureC()
	if err != nil {
		if !isTemperatureUnavailable(err) {
			warnings = append(warnings, fmt.Sprintf("cpu temp: %v", err))
		}
	} else if tempC != nil {
		stats.TemperatureC = tempC
	}

	if len(warnings) > 0 {
		return stats, fmt.Errorf("%s", strings.Join(warnings, "; "))
	}
	return stats, nil
}

func sampleCPUTemperatureC() (*float64, error) {
	temps, err := host.SensorsTemperatures()
	if err != nil {
		return nil, err
	}

	var best *float64
	bestScore := -1
	bestTemp := -1.0

	for _, t := range temps {
		temp := t.Temperature
		if temp <= 0 || !isFiniteFloat(temp) {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(t.SensorKey))
		score := 0
		switch {
		case strings.Contains(key, "package"):
			score += 50
		case strings.Contains(key, "tctl") || strings.Contains(key, "tdie"):
			score += 40
		}
		if strings.Contains(key, "coretemp") || strings.Contains(key, "k10temp") {
			score += 20
		}
		if strings.Contains(key, "cpu") {
			score += 10
		}
		if strings.Contains(key, "core") {
			score += 5
		}

		if score > bestScore || (score == bestScore && temp > bestTemp) {
			v := temp
			best = &v
			bestScore = score
			bestTemp = temp
		}
	}

	return best, nil
}

func isFiniteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func isTemperatureUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not implemented") || strings.Contains(msg, "not supported")
}

func cloneHistory(src []HistoryPoint) []HistoryPoint {
	if len(src) == 0 {
		return nil
	}
	out := make([]HistoryPoint, len(src))
	for i, h := range src {
		out[i] = h
		if h.Disks != nil {
			dm := make(map[string]float64, len(h.Disks))
			for k, v := range h.Disks {
				dm[k] = v
			}
			out[i].Disks = dm
		}
	}
	return out
}

func sampleProcessCount() (int, error) {
	pids, err := process.Pids()
	if err != nil {
		return 0, err
	}
	return len(pids), nil
}

func linuxCPUFreqSummary() (cpuFreqSummary, error) {
	const cpuRoot = "/sys/devices/system/cpu"

	entries, err := os.ReadDir(cpuRoot)
	if err != nil {
		return cpuFreqSummary{}, err
	}

	type coreAgg struct {
		maxKHz  int64
		threads int
	}

	cores := make(map[string]*coreAgg)

	var curSumKHz int64
	var curCount int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "cpu") {
			continue
		}
		_, err := strconv.Atoi(strings.TrimPrefix(name, "cpu"))
		if err != nil {
			continue
		}

		base := cpuRoot + "/" + name
		maxKHz, err := readIntFromFile(base + "/cpufreq/cpuinfo_max_freq")
		if err != nil || maxKHz <= 0 {
			maxKHz, err = readIntFromFile(base + "/cpufreq/scaling_max_freq")
		}
		if err != nil || maxKHz <= 0 {
			continue
		}

		curKHz, err := readIntFromFile(base + "/cpufreq/scaling_cur_freq")
		if err != nil || curKHz <= 0 {
			curKHz, err = readIntFromFile(base + "/cpufreq/cpuinfo_cur_freq")
		}
		if err == nil && curKHz > 0 {
			curSumKHz += curKHz
			curCount++
		}

		pkg, errPkg := readIntFromFile(base + "/topology/physical_package_id")
		core, errCore := readIntFromFile(base + "/topology/core_id")
		key := name
		if errPkg == nil && errCore == nil {
			key = fmt.Sprintf("%d:%d", pkg, core)
		}

		agg := cores[key]
		if agg == nil {
			agg = &coreAgg{}
			cores[key] = agg
		}
		if maxKHz > agg.maxKHz {
			agg.maxKHz = maxKHz
		}
		agg.threads++
	}

	if len(cores) == 0 {
		return cpuFreqSummary{}, fmt.Errorf("no cpufreq data found")
	}

	uniqueMax := make([]int64, 0, len(cores))
	seen := make(map[int64]struct{})
	for _, c := range cores {
		if c.maxKHz <= 0 {
			continue
		}
		if _, ok := seen[c.maxKHz]; ok {
			continue
		}
		seen[c.maxKHz] = struct{}{}
		uniqueMax = append(uniqueMax, c.maxKHz)
	}
	sort.Slice(uniqueMax, func(i, j int) bool { return uniqueMax[i] < uniqueMax[j] })

	perfKHz := uniqueMax[len(uniqueMax)-1]
	effKHz := uniqueMax[0]

	const tolKHz = 50_000 // 50 MHz tolerance
	var perfCores, effCores, perfThreads, effThreads int

	if len(uniqueMax) >= 2 && absInt64(perfKHz-effKHz) >= 100_000 {
		for _, c := range cores {
			if absInt64(c.maxKHz-perfKHz) <= tolKHz {
				perfCores++
				perfThreads += c.threads
			} else if absInt64(c.maxKHz-effKHz) <= tolKHz {
				effCores++
				effThreads += c.threads
			}
		}
	}

	curMHz := 0.0
	if curCount > 0 {
		curMHz = float64(curSumKHz) / float64(curCount) / 1000
	}

	return cpuFreqSummary{
		CurrentMHz:         curMHz,
		MaxMHz:             float64(perfKHz) / 1000,
		PerformanceCores:   perfCores,
		EfficiencyCores:    effCores,
		PerformanceThreads: perfThreads,
		EfficiencyThreads:  effThreads,
	}, nil
}

func readIntFromFile(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseInt(s, 10, 64)
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (m *ResourceMonitor) sampleMemory() (MemoryStats, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return MemoryStats{}, err
	}
	if vm == nil {
		return MemoryStats{}, nil
	}
	sm, err := mem.SwapMemory()
	if err != nil {
		return MemoryStats{}, err
	}

	stats := MemoryStats{
		TotalBytes:      vm.Total,
		UsedBytes:       vm.Used,
		UsedPercent:     vm.UsedPercent,
		SwapTotalBytes:  sm.Total,
		SwapUsedBytes:   sm.Used,
		SwapUsedPercent: sm.UsedPercent,
	}

	if modules, err := m.getMemoryModules(); err == nil && len(modules) > 0 {
		stats.Modules = modules
	}

	return stats, nil
}

func (m *ResourceMonitor) getMemoryModules() ([]MemoryModuleInfo, error) {
	if m.memoryModulesLoaded {
		return m.memoryModules, nil
	}

	info, err := ghw.Memory()
	if err != nil {
		return nil, err
	}

	modules := make([]MemoryModuleInfo, 0, len(info.Modules))
	for _, mod := range info.Modules {
		if mod == nil {
			continue
		}
		sizeBytes := uint64(0)
		if mod.SizeBytes > 0 {
			sizeBytes = uint64(mod.SizeBytes)
		}
		modules = append(modules, MemoryModuleInfo{
			Label:     strings.TrimSpace(mod.Label),
			Vendor:    strings.TrimSpace(mod.Vendor),
			SizeBytes: sizeBytes,
		})
	}

	m.memoryModules = modules
	m.memoryModulesLoaded = true
	return modules, nil
}

const (
	hardwareMetaTTL    = 30 * time.Second
	hostIPTTL          = 30 * time.Second
	cpuStaticTTL       = 1 * time.Minute
	cpuDynamicTTLLinux = 2 * time.Second
	cpuDynamicTTLOther = 5 * time.Second
	disksSampleTTL     = 5 * time.Second
	gpusSampleTTL      = 5 * time.Second
	historyMaxAge      = 30 * time.Minute
	historyMaxPoints   = 2000
)

func (m *ResourceMonitor) getDiskMeta() (map[string]diskMeta, error) {
	if m.diskMeta != nil && time.Since(m.diskMetaUpdatedAt) < hardwareMetaTTL {
		return m.diskMeta, nil
	}

	info, err := ghw.Block()
	if err != nil {
		return m.diskMeta, err
	}

	meta := make(map[string]diskMeta)
	for _, d := range info.Disks {
		model := normalizeSpaces(strings.TrimSpace(d.Vendor + " " + d.Model))
		driveType := diskTypeLabel(d.DriveType.String(), d.StorageController.String())
		for _, p := range d.Partitions {
			if p == nil || p.MountPoint == "" {
				continue
			}
			meta[p.MountPoint] = diskMeta{
				DriveType:         driveType,
				StorageController: strings.TrimSpace(d.StorageController.String()),
				Model:             model,
			}
		}
	}

	m.diskMeta = meta
	m.diskMetaUpdatedAt = time.Now()
	return meta, nil
}

func (m *ResourceMonitor) sampleDisks() ([]DiskStats, error) {
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	meta, metaErr := m.getDiskMeta()

	partsByMount := make(map[string]disk.PartitionStat, len(parts))
	for _, p := range parts {
		if p.Mountpoint == "" {
			continue
		}
		if _, ok := partsByMount[p.Mountpoint]; !ok {
			partsByMount[p.Mountpoint] = p
		}
	}

	ignoredFSTypes := map[string]struct{}{
		"autofs":     {},
		"cgroup":     {},
		"cgroup2":    {},
		"configfs":   {},
		"debugfs":    {},
		"devfs":      {},
		"devpts":     {},
		"devtmpfs":   {},
		"fusectl":    {},
		"hugetlbfs":  {},
		"mqueue":     {},
		"proc":       {},
		"pstore":     {},
		"securityfs": {},
		"sysfs":      {},
		"tmpfs":      {},
		"tracefs":    {},
	}

	selected := make(map[string]struct{})

	switch runtime.GOOS {
	case "linux":
		selected["/"] = struct{}{}
		for _, p := range parts {
			if p.Mountpoint == "" || p.Mountpoint == "/" {
				continue
			}
			if _, ignore := ignoredFSTypes[p.Fstype]; ignore {
				continue
			}
			if p.Mountpoint == "/mnt" || strings.HasPrefix(p.Mountpoint, "/mnt/") {
				selected[p.Mountpoint] = struct{}{}
			}
		}
	case "windows":
		for _, p := range parts {
			mp := p.Mountpoint
			if mp == "" {
				continue
			}
			if len(mp) >= 2 && mp[1] == ':' {
				selected[mp] = struct{}{}
			}
		}
	default:
		selected["/"] = struct{}{}
	}

	if len(selected) == 0 {
		for _, p := range parts {
			if p.Mountpoint != "" {
				selected[p.Mountpoint] = struct{}{}
				break
			}
		}
	}

	mountpoints := make([]string, 0, len(selected))
	for mp := range selected {
		mountpoints = append(mountpoints, mp)
	}
	sort.Strings(mountpoints)

	out := make([]DiskStats, 0, len(mountpoints))
	for _, mp := range mountpoints {
		usage, err := disk.Usage(mp)
		if err != nil || usage == nil {
			continue
		}

		var device, fstype string
		if p, ok := partsByMount[mp]; ok {
			device = strings.TrimSpace(p.Device)
			fstype = strings.TrimSpace(p.Fstype)
		}
		if fstype == "" {
			fstype = strings.TrimSpace(usage.Fstype)
		}

		ds := DiskStats{
			Mountpoint:  mp,
			Device:      device,
			Filesystem:  fstype,
			TotalBytes:  usage.Total,
			UsedBytes:   usage.Used,
			UsedPercent: usage.UsedPercent,
		}
		if meta != nil {
			if m, ok := meta[mp]; ok {
				ds.DriveType = m.DriveType
				ds.Model = m.Model
			}
		}
		out = append(out, ds)
	}

	if metaErr != nil {
		return out, fmt.Errorf("disk metadata: %w", metaErr)
	}
	return out, nil
}

func diskTypeLabel(driveType, controller string) string {
	controller = strings.TrimSpace(controller)
	if strings.EqualFold(controller, "nvme") {
		return "NVMe"
	}
	driveType = strings.TrimSpace(driveType)
	if driveType == "" || strings.EqualFold(driveType, "unknown") {
		return ""
	}
	return strings.ToUpper(driveType)
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

type nvidiaSMIGPU struct {
	Name          string
	UtilPercent   float64
	MemUsedBytes  uint64
	MemTotalBytes uint64
	TempC         float64
}

func (m *ResourceMonitor) getGPUMeta() ([]GPUStats, error) {
	if m.gpuMeta != nil && time.Since(m.gpuMetaUpdatedAt) < hardwareMetaTTL {
		return m.gpuMeta, nil
	}

	info, err := ghw.GPU()
	if err != nil {
		return m.gpuMeta, err
	}

	gpus := make([]GPUStats, 0, len(info.GraphicsCards))
	for _, card := range info.GraphicsCards {
		gs := GPUStats{Index: card.Index}
		if card.DeviceInfo != nil {
			gs.Driver = strings.TrimSpace(card.DeviceInfo.Driver)
			if card.DeviceInfo.Vendor != nil {
				gs.Vendor = strings.TrimSpace(card.DeviceInfo.Vendor.Name)
			}
			if card.DeviceInfo.Product != nil {
				gs.Name = strings.TrimSpace(card.DeviceInfo.Product.Name)
			}
		}
		if gs.Name == "" && gs.Vendor != "" {
			gs.Name = gs.Vendor
		}
		gpus = append(gpus, gs)
	}

	sort.Slice(gpus, func(i, j int) bool { return gpus[i].Index < gpus[j].Index })

	m.gpuMeta = gpus
	m.gpuMetaUpdatedAt = time.Now()
	return gpus, nil
}

func (m *ResourceMonitor) sampleGPUs() ([]GPUStats, error) {
	base, ghwErr := m.getGPUMeta()
	gpus := append([]GPUStats(nil), base...)

	metrics, smiErr := nvidiaSMIMetrics()
	if len(metrics) > 0 {
		gpus = mergeNvidiaSMIMetrics(gpus, metrics)
	}

	if len(gpus) == 0 {
		if ghwErr != nil && smiErr != nil {
			return nil, fmt.Errorf("gpu: ghw=%v; nvidia-smi=%v", ghwErr, smiErr)
		}
		return nil, nil
	}
	return gpus, nil
}

func nvidiaSMIMetrics() ([]nvidiaSMIGPU, error) {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path,
		"--query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}

	lines := strings.Split(text, "\n")
	metrics := make([]nvidiaSMIGPU, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		util, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		memUsedMiB, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		memTotalMiB, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		temp, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)

		metrics = append(metrics, nvidiaSMIGPU{
			Name:          name,
			UtilPercent:   util,
			MemUsedBytes:  uint64(memUsedMiB * 1024 * 1024),
			MemTotalBytes: uint64(memTotalMiB * 1024 * 1024),
			TempC:         temp,
		})
	}
	return metrics, nil
}

func mergeNvidiaSMIMetrics(gpus []GPUStats, metrics []nvidiaSMIGPU) []GPUStats {
	nvidiaIdx := make([]int, 0, len(gpus))
	for i := range gpus {
		if strings.Contains(strings.ToLower(gpus[i].Vendor), "nvidia") {
			nvidiaIdx = append(nvidiaIdx, i)
		}
	}

	if len(nvidiaIdx) == 0 {
		for i, m := range metrics {
			util := m.UtilPercent
			temp := m.TempC
			memUsed := m.MemUsedBytes
			memTotal := m.MemTotalBytes
			gpus = append(gpus, GPUStats{
				Index:              i,
				Name:               m.Name,
				Vendor:             "NVIDIA",
				UtilizationPercent: &util,
				MemoryUsedBytes:    &memUsed,
				MemoryTotalBytes:   &memTotal,
				TemperatureC:       &temp,
			})
		}
		return gpus
	}

	for i, m := range metrics {
		if i >= len(nvidiaIdx) {
			break
		}
		pos := nvidiaIdx[i]

		util := m.UtilPercent
		temp := m.TempC
		memUsed := m.MemUsedBytes
		memTotal := m.MemTotalBytes

		if strings.TrimSpace(gpus[pos].Name) == "" {
			gpus[pos].Name = m.Name
		}
		gpus[pos].UtilizationPercent = &util
		gpus[pos].TemperatureC = &temp
		gpus[pos].MemoryUsedBytes = &memUsed
		gpus[pos].MemoryTotalBytes = &memTotal
	}

	return gpus
}

func preferredHostIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var candidates []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch a := addr.(type) {
			case *net.IPNet:
				ip = a.IP
			case *net.IPAddr:
				ip = a.IP
			default:
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			candidates = append(candidates, ip)
		}
	}

	for _, ip := range candidates {
		if ip[0] == 192 && ip[1] == 168 && ip[2] == 1 {
			return ip.String(), nil
		}
	}
	for _, ip := range candidates {
		if ip[0] == 192 && ip[1] == 168 {
			return ip.String(), nil
		}
	}
	for _, ip := range candidates {
		if isPrivateIPv4(ip) {
			return ip.String(), nil
		}
	}
	if len(candidates) > 0 {
		return candidates[0].String(), nil
	}
	return "", nil
}

func isPrivateIPv4(ip net.IP) bool {
	if len(ip) != 4 {
		ip = ip.To4()
	}
	if len(ip) != 4 {
		return false
	}

	if ip[0] == 10 {
		return true
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}
