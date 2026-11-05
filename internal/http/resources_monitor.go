package http

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

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
	hostIPs         []HostIPCandidate
	hostIPUpdatedAt time.Time
	hostIPErr       error

	hostInfo          HostInfo
	hostInfoUpdatedAt time.Time
	hostInfoErr       error

	netPrevRx   uint64
	netPrevTx   uint64
	netPrevAt   time.Time
	haveNetPrev bool

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

	prevProcessTimes   map[int32]float64
	lastProcessSample  time.Time
	boardModel         string
	boardModelResolved bool

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
	snap.HostIPs = append([]HostIPCandidate(nil), m.snapshot.HostIPs...)
	if includeHistory {
		snap.History = cloneHistory(m.history)
	}
	return snap
}

// DetectedHostIP returns the auto-detected LAN IP from the latest sample.
func (m *ResourceMonitor) DetectedHostIP() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot.DetectedHostIP
}

func (m *ResourceMonitor) Hostname() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot.Host.Hostname
}

func (m *ResourceMonitor) update() {
	now := time.Now()
	var errs SnapshotError

	if m.hostIP == "" || now.Sub(m.hostIPUpdatedAt) >= hostIPTTL {
		m.hostIP, m.hostIPs, m.hostIPErr = detectHostIPs()
		m.hostIPUpdatedAt = now
	}
	if m.hostIPErr != nil {
		errs.HostIP = m.hostIPErr.Error()
	}

	// CPU % and network rates are deltas, so the very first sample has no
	// baseline and reads as 0. Keep it out of the history graph.
	baselineReady := m.havePrevCPU
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

	topCPU, topMem, topProcErr := m.sampleTopProcesses(now, cpuStats.LogicalCores, memStats.TotalBytes)
	if topProcErr != nil {
		errs.CPU = strings.TrimSpace(strings.Join([]string{errs.CPU, fmt.Sprintf("top processes: %v", topProcErr)}, "; "))
	}

	hostInfo, hostErr := m.sampleHostInfo(now)
	if hostErr != nil {
		errs.Host = hostErr.Error()
	}

	netStats, netErr := m.sampleNetwork(now)
	if netErr != nil {
		errs.Network = netErr.Error()
	}

	snap := ResourcesSnapshot{
		HostIP:         m.hostIP,
		DetectedHostIP: m.hostIP,
		HostIPs:        m.hostIPs,
		Host:           hostInfo,
		UpdatedAt:      now.UnixMilli(),
		CPU:            cpuStats,
		Load:           sampleLoad(),
		Memory:         memStats,
		Network:        netStats,
		Disks:          m.disksCache,
		GPUs:           m.gpusCache,
		Processes:      procCount,
		TopCPU:         topCPU,
		TopMemory:      topMem,
		Errors:         errs,
	}

	m.mu.Lock()
	m.snapshot = snap
	if baselineReady {
		m.appendHistoryLocked(snap)
	}
	m.mu.Unlock()
}

func (m *ResourceMonitor) appendHistoryLocked(snap ResourcesSnapshot) {
	hp := HistoryPoint{
		Time: snap.UpdatedAt,
		CPU:  snap.CPU.Percent,
		Mem:  snap.Memory.UsedPercent,
		Rx:   snap.Network.RxBytesPerSec,
		Tx:   snap.Network.TxBytesPerSec,
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
