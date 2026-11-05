package http

type ResourcesSnapshot struct {
	// HostIP is the IP the UI should show and substitute for {ip}: the settings
	// override when set, otherwise DetectedHostIP.
	HostIP         string            `json:"hostIp"`
	DetectedHostIP string            `json:"detectedHostIp"`
	HostIPOverride bool              `json:"hostIpOverride"`
	HostIPs        []HostIPCandidate `json:"hostIps,omitempty"`
	Host           HostInfo          `json:"host"`
	UpdatedAt      int64             `json:"updatedAt"`
	CPU            CPUStats          `json:"cpu"`
	Load           *LoadStats        `json:"load,omitempty"`
	Memory         MemoryStats       `json:"memory"`
	Network        NetworkStats      `json:"network"`
	Disks          []DiskStats       `json:"disks"`
	GPUs           []GPUStats        `json:"gpus,omitempty"`
	Processes      int               `json:"processes"`
	TopCPU         *ProcessSample    `json:"topCpu,omitempty"`
	TopMemory      *ProcessSample    `json:"topMemory,omitempty"`
	History        []HistoryPoint    `json:"history,omitempty"`
	Errors         SnapshotError     `json:"errors"`
}

type HostInfo struct {
	Hostname  string `json:"hostname"`
	OS        string `json:"os"`
	Platform  string `json:"platform"`
	Kernel    string `json:"kernel"`
	Arch      string `json:"arch"`
	BootTime  int64  `json:"bootTime"`
	UptimeSec int64  `json:"uptimeSec"`
}

type LoadStats struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

type NetworkStats struct {
	RxBytesPerSec float64 `json:"rxBytesPerSec"`
	TxBytesPerSec float64 `json:"txBytesPerSec"`
	RxTotalBytes  uint64  `json:"rxTotalBytes"`
	TxTotalBytes  uint64  `json:"txTotalBytes"`
}

type SnapshotError struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Disks   string `json:"disks"`
	GPUs    string `json:"gpus"`
	HostIP  string `json:"hostIp"`
	Host    string `json:"host"`
	Network string `json:"network"`
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
	Rx    float64            `json:"rx"`
	Tx    float64            `json:"tx"`
	Disks map[string]float64 `json:"disks,omitempty"`
}

type diskMeta struct {
	DriveType         string
	StorageController string
	Model             string
}
