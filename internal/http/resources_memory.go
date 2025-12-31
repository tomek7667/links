package http

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/mem"
)

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
	} else if len(stats.Modules) == 0 {
		model := m.boardModelName()
		if strings.Contains(strings.ToLower(model), "raspberry pi") {
			stats.Modules = []MemoryModuleInfo{
				{
					Label:     "SoC",
					Vendor:    model,
					SizeBytes: vm.Total,
				},
			}
		}
	}

	if swapDevices, err := getSwapDevices(); err == nil && len(swapDevices) > 0 {
		stats.SwapDevices = swapDevices
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

func (m *ResourceMonitor) boardModelName() string {
	if m.boardModelResolved {
		return m.boardModel
	}
	m.boardModelResolved = true

	if runtime.GOOS == "linux" {
		paths := []string{
			"/proc/device-tree/model",
			"/sys/firmware/devicetree/base/model",
		}
		for _, p := range paths {
			if b, err := os.ReadFile(p); err == nil {
				model := strings.TrimRight(strings.TrimSpace(string(b)), "\x00")
				if model != "" {
					m.boardModel = model
					break
				}
			}
		}
	}

	return m.boardModel
}

func getSwapDevices() ([]SwapDeviceStats, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}

	f, err := os.Open("/proc/swaps")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var swaps []SwapDeviceStats
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		sizeKB, _ := strconv.ParseUint(fields[2], 10, 64)
		usedKB, _ := strconv.ParseUint(fields[3], 10, 64)
		swaps = append(swaps, SwapDeviceStats{
			Name:      fields[0],
			Type:      fields[1],
			SizeBytes: sizeKB * 1024,
			UsedBytes: usedKB * 1024,
		})
	}
	if err := scanner.Err(); err != nil {
		return swaps, err
	}
	return swaps, nil
}
