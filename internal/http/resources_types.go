package http

type ResourcesSnapshot struct {
	HostIP    string         `json:"hostIp"`
	UpdatedAt int64          `json:"updatedAt"`
	CPU       CPUStats       `json:"cpu"`
	Memory    MemoryStats    `json:"memory"`
	Disks     []DiskStats    `json:"disks"`
	GPUs      []GPUStats     `json:"gpus,omitempty"`
	Processes int            `json:"processes"`
	TopCPU    *ProcessSample `json:"topCpu,omitempty"`
	TopMemory *ProcessSample `json:"topMemory,omitempty"`
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
	SwapDevices     []SwapDeviceStats  `json:"swapDevices,omitempty"`
}

type MemoryModuleInfo struct {
	Label     string `json:"label"`
	Vendor    string `json:"vendor"`
	SizeBytes uint64 `json:"sizeBytes"`
}

type SwapDeviceStats struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	SizeBytes uint64 `json:"sizeBytes"`
	UsedBytes uint64 `json:"usedBytes"`
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

type ProcessSample struct {
	PID           int     `json:"pid"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpuPercent,omitempty"`
	MemoryBytes   uint64  `json:"memoryBytes,omitempty"`
	MemoryPercent float64 `json:"memoryPercent,omitempty"`
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
