package http

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/disk"
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
	parts, err := disk.Partitions(true)
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
		"overlay":    {},
		"squashfs":   {},
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
			mp := strings.TrimSpace(p.Mountpoint)
			if mp == "" {
				continue
			}
			if mp == "/mnt" || strings.HasPrefix(mp, "/mnt/") || strings.HasPrefix(mp, "/media/") || strings.HasPrefix(mp, "/run/media/") {
				selected[mp] = struct{}{}
				continue
			}
			dev := strings.TrimSpace(p.Device)
			if strings.HasPrefix(dev, "/dev/") && !strings.Contains(dev, "loop") {
				selected[mp] = struct{}{}
				continue
			}
			if strings.HasPrefix(mp, "/") && !strings.HasPrefix(mp, "/sys/") && !strings.HasPrefix(mp, "/proc/") && !strings.HasPrefix(mp, "/dev/") {
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
		if controller != "" && !strings.EqualFold(controller, "unknown") {
			return strings.ToUpper(controller)
		}
		return ""
	}
	return strings.ToUpper(driveType)
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
