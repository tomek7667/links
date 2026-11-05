package http

import (
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	psnet "github.com/shirou/gopsutil/v3/net"
)

const hostInfoTTL = 1 * time.Minute

func (m *ResourceMonitor) sampleHostInfo(now time.Time) (HostInfo, error) {
	if m.hostInfoUpdatedAt.IsZero() || now.Sub(m.hostInfoUpdatedAt) >= hostInfoTTL {
		info, err := host.Info()
		m.hostInfoUpdatedAt = now
		m.hostInfoErr = err
		if err == nil && info != nil {
			platform := strings.TrimSpace(strings.Join([]string{info.Platform, info.PlatformVersion}, " "))
			if platform == "" {
				platform = info.OS
			}
			m.hostInfo = HostInfo{
				Hostname: info.Hostname,
				OS:       info.OS,
				Platform: platform,
				Kernel:   info.KernelVersion,
				Arch:     runtime.GOARCH,
				BootTime: int64(info.BootTime),
			}
		}
	}

	hi := m.hostInfo
	if hi.BootTime > 0 {
		hi.UptimeSec = max(0, now.Unix()-hi.BootTime)
	}
	return hi, m.hostInfoErr
}

func sampleLoad() *LoadStats {
	// gopsutil emulates load average on Windows with a background collector;
	// it is not a meaningful number there, so skip it.
	if runtime.GOOS == "windows" {
		return nil
	}
	avg, err := load.Avg()
	if err != nil || avg == nil {
		return nil
	}
	return &LoadStats{Load1: avg.Load1, Load5: avg.Load5, Load15: avg.Load15}
}

// skipNetInterface filters loopback and virtual interfaces so container and
// bridge traffic is not counted twice on top of the physical NIC.
func skipNetInterface(name string) bool {
	n := strings.ToLower(name)
	for _, prefix := range []string{"lo", "veth", "docker", "br-", "virbr", "cni", "flannel", "cali", "vxlan", "kube", "loopback"} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

func (m *ResourceMonitor) sampleNetwork(now time.Time) (NetworkStats, error) {
	counters, err := psnet.IOCounters(true)
	if err != nil {
		return NetworkStats{}, err
	}

	var rx, tx uint64
	for _, c := range counters {
		if skipNetInterface(c.Name) {
			continue
		}
		rx += c.BytesRecv
		tx += c.BytesSent
	}

	stats := NetworkStats{RxTotalBytes: rx, TxTotalBytes: tx}
	if m.haveNetPrev {
		if dt := now.Sub(m.netPrevAt).Seconds(); dt > 0 {
			// Counters can reset (interface restart); report 0 instead of a huge spike.
			if rx >= m.netPrevRx {
				stats.RxBytesPerSec = float64(rx-m.netPrevRx) / dt
			}
			if tx >= m.netPrevTx {
				stats.TxBytesPerSec = float64(tx-m.netPrevTx) / dt
			}
		}
	}
	m.netPrevRx, m.netPrevTx, m.netPrevAt, m.haveNetPrev = rx, tx, now, true
	return stats, nil
}
