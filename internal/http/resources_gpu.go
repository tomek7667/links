package http

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jaypipes/ghw"
)

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
	path, err := findNvidiaSMI()
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

func findNvidiaSMI() (string, error) {
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		return p, nil
	}

	if runtime.GOOS == "windows" {
		candidates := []string{
			os.ExpandEnv(`%ProgramFiles%\NVIDIA Corporation\NVSMI\nvidia-smi.exe`),
			os.ExpandEnv(`%ProgramFiles(x86)%\NVIDIA Corporation\NVSMI\nvidia-smi.exe`),
		}
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("nvidia-smi not found")
}
