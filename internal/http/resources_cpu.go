package http

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
)

func (m *ResourceMonitor) sampleCPUPercent() (float64, error) {
	times, err := cpu.Times(false)
	if err != nil {
		return 0, err
	}
	if len(times) == 0 {
		return 0, nil
	}

	t := times[0]
	total := cpuTimesTotal(t)
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

	usage := (totalDelta - idleDelta) / totalDelta * 100
	if usage < 0 {
		return 0, nil
	}
	if usage > 100 {
		return 100, nil
	}
	return usage, nil
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

func cpuTimesTotal(t cpu.TimesStat) float64 {
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Guest + t.GuestNice
}

func cpuTimesTotalPtr(t *cpu.TimesStat) float64 {
	if t == nil {
		return 0
	}
	return cpuTimesTotal(*t)
}

func isTemperatureUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not implemented") || strings.Contains(msg, "not supported")
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
