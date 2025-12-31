package http

import (
	"context"
	"math"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

func sampleProcessCount() (int, error) {
	pids, err := process.Pids()
	if err != nil {
		return 0, err
	}
	return len(pids), nil
}

func (m *ResourceMonitor) sampleTopProcesses(now time.Time, logicalCores int, memTotal uint64) (*ProcessSample, *ProcessSample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	elapsedSec := now.Sub(m.lastProcessSample).Seconds()
	newPrev := make(map[int32]float64, len(procs))

	cores := logicalCores
	if cores <= 0 {
		cores = runtime.NumCPU()
	}
	if cores <= 0 {
		cores = 1
	}

	var topCPU *ProcessSample
	var topMem *ProcessSample

	for _, p := range procs {
		if p == nil {
			continue
		}
		pid := p.Pid
		name, _ := p.NameWithContext(ctx)

		if times, err := p.TimesWithContext(ctx); err == nil {
			totalCPU := cpuTimesTotalPtr(times)
			newPrev[pid] = totalCPU
			if elapsedSec > 0 {
				if prev, ok := m.prevProcessTimes[pid]; ok {
					delta := totalCPU - prev
					if delta < 0 {
						delta = 0
					}
					cpuPct := (delta / elapsedSec) * 100 / float64(cores)
					if cpuPct < 0 {
						cpuPct = 0
					}
					if cpuPct > 100 {
						cpuPct = math.Min(cpuPct, 100)
					}
					if topCPU == nil || cpuPct > topCPU.CPUPercent {
						topCPU = &ProcessSample{
							PID:        int(pid),
							Name:       name,
							CPUPercent: cpuPct,
						}
					}
				}
			}
		}

		if memInfo, err := p.MemoryInfoWithContext(ctx); err == nil && memInfo != nil {
			memBytes := memInfo.RSS
			memPct := 0.0
			if memTotal > 0 {
				memPct = float64(memBytes) / float64(memTotal) * 100
			}
			if topMem == nil || memBytes > topMem.MemoryBytes {
				topMem = &ProcessSample{
					PID:           int(pid),
					Name:          name,
					MemoryBytes:   memBytes,
					MemoryPercent: memPct,
				}
			}
			if topCPU != nil && topCPU.PID == int(pid) && topCPU.Name == name {
				topCPU.MemoryBytes = memBytes
				topCPU.MemoryPercent = memPct
			}
		}
	}

	m.prevProcessTimes = newPrev
	m.lastProcessSample = now

	return topCPU, topMem, nil
}
