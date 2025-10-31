package monitor

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemStats contains all collected system metrics
type SystemStats struct {
	BootTime   BootTime   `json:"boot_time"`
	CPUInfo    CPUInfo    `json:"cpu_info"`
	MemoryInfo MemoryInfo `json:"memory_info"`
	DiskInfo   DiskInfo   `json:"disk_info"`
}

// BootTime contains boot time information
type BootTime struct {
	BootTime string `json:"boot_time"`
}

// CPUInfo contains CPU metrics
type CPUInfo struct {
	PhysicalCores   int32              `json:"physical_cores"`
	TotalCores      int32              `json:"total_cores"`
	MaxFrequency    string             `json:"max_frequency"`
	MinFrequency    string             `json:"min_frequency"`
	CurrentFrequency string             `json:"current_frequency"`
	CPUUsagePerCore map[string]string  `json:"cpu_usage_per_core"`
	TotalCPUUsage   string             `json:"total_cpu_usage"`
}

// MemoryInfo contains memory metrics
type MemoryInfo struct {
	VirtualMemory VirtualMemory `json:"virtual_memory"`
	SwapMemory    SwapMemory    `json:"swap_memory"`
}

// VirtualMemory contains virtual memory metrics
type VirtualMemory struct {
	Total      string `json:"total"`
	Available  string `json:"available"`
	Percentage string `json:"percentage"`
}

// SwapMemory contains swap memory metrics
type SwapMemory struct {
	Total      string `json:"total"`
	Free       string `json:"free"`
	Used       string `json:"used"`
	Percentage string `json:"percentage"`
}

// DiskInfo contains disk metrics
type DiskInfo struct {
	Partitions []PartitionInfo `json:"partitions"`
	IOStats    IOStats         `json:"io_stats"`
}

// PartitionInfo contains information about a disk partition
type PartitionInfo struct {
	Device     string `json:"device"`
	Mountpoint string `json:"mountpoint"`
	FSType     string `json:"file_system_type"`
	TotalSize  string `json:"total_size"`
	Used       string `json:"used"`
	Free       string `json:"free"`
	Percentage string `json:"percentage"`
}

// IOStats contains disk IO statistics
type IOStats struct {
	TotalRead  string `json:"total_read"`
	TotalWrite string `json:"total_write"`
}

// GetSystemStats collects all system statistics
func GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// Get boot time
	if bootInfo, err := getBootTime(); err == nil {
		stats.BootTime = bootInfo
	} else {
		log.Printf("Error getting boot time: %v", err)
	}

	// Get CPU info
	if cpuInfo, err := getCPUInfo(); err == nil {
		stats.CPUInfo = cpuInfo
	} else {
		return nil, fmt.Errorf("error getting CPU info: %w", err)
	}

	// Get memory info
	if memInfo, err := getMemoryInfo(); err == nil {
		stats.MemoryInfo = memInfo
	} else {
		return nil, fmt.Errorf("error getting memory info: %w", err)
	}

	// Get disk info
	if diskInfo, err := getDiskInfo(); err == nil {
		stats.DiskInfo = diskInfo
	} else {
		return nil, fmt.Errorf("error getting disk info: %w", err)
	}

	return stats, nil
}

// getBootTime retrieves the system boot time
func getBootTime() (BootTime, error) {
	bootTimestamp, err := host.BootTime()
	if err != nil {
		return BootTime{}, err
	}

	// Convert Unix timestamp to time.Time
	bootTime := time.Unix(int64(bootTimestamp), 0)

	return BootTime{
		BootTime: fmt.Sprintf("%d/%d/%d %d:%d:%d",
			bootTime.Year(), int(bootTime.Month()), bootTime.Day(),
			bootTime.Hour(), bootTime.Minute(), bootTime.Second()),
	}, nil
}

// getCPUInfo retrieves CPU information
func getCPUInfo() (CPUInfo, error) {
	cpuInfo := CPUInfo{}

	// Get core counts
	physicalCores, err := cpu.Counts(false)
	if err != nil {
		return cpuInfo, err
	}
	cpuInfo.PhysicalCores = int32(physicalCores)

	totalCores, err := cpu.Counts(true)
	if err != nil {
		return cpuInfo, err
	}
	cpuInfo.TotalCores = int32(totalCores)

	// Get CPU frequency (v4 API - may not be available on all platforms)
	// This field is optional and may not be available
	cpuInfo.MaxFrequency = "N/A"
	cpuInfo.MinFrequency = "N/A"
	cpuInfo.CurrentFrequency = "N/A"

	// Get per-core CPU usage
	cpuUsages, err := cpu.Percent(0, true)
	if err != nil {
		return cpuInfo, err
	}

	cpuInfo.CPUUsagePerCore = make(map[string]string)
	for i, usage := range cpuUsages {
		cpuInfo.CPUUsagePerCore[fmt.Sprintf("core_%d", i)] = fmt.Sprintf("%.2f", usage)
	}

	// Get total CPU usage
	totalUsage, err := cpu.Percent(0, false)
	if err != nil {
		return cpuInfo, err
	}
	if len(totalUsage) > 0 {
		cpuInfo.TotalCPUUsage = fmt.Sprintf("%.2f", totalUsage[0])
	}

	return cpuInfo, nil
}

// getMemoryInfo retrieves memory information
func getMemoryInfo() (MemoryInfo, error) {
	memInfo := MemoryInfo{}

	// Get virtual memory
	vMemory, err := mem.VirtualMemory()
	if err != nil {
		return memInfo, err
	}

	totalFreeMemory := vMemory.Available
	// Try to read from /proc/meminfo if available (more accurate on Linux)
	if freeMem := readMemFreeFromProc(); freeMem > 0 {
		totalFreeMemory = uint64(freeMem)
	}

	percentage := 100 - ((float64(totalFreeMemory) / float64(vMemory.Total)) * 100)

	memInfo.VirtualMemory = VirtualMemory{
		Total:      formatBytes(vMemory.Total),
		Available:  formatBytes(totalFreeMemory),
		Percentage: fmt.Sprintf("%.2f", percentage),
	}

	// Get swap memory
	swapMem, err := mem.SwapMemory()
	if err != nil {
		return memInfo, err
	}

	memInfo.SwapMemory = SwapMemory{
		Total:      formatBytes(swapMem.Total),
		Free:       formatBytes(swapMem.Free),
		Used:       formatBytes(swapMem.Used),
		Percentage: fmt.Sprintf("%.2f", swapMem.UsedPercent),
	}

	return memInfo, nil
}

// getDiskInfo retrieves disk information
func getDiskInfo() (DiskInfo, error) {
	diskInfo := DiskInfo{
		Partitions: []PartitionInfo{},
	}

	// Get disk partitions
	partitions, err := disk.Partitions(false)
	if err != nil {
		return diskInfo, err
	}

	for _, partition := range partitions {
		// Skip loop devices
		if strings.HasPrefix(partition.Device, "/dev/loop") {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			log.Printf("Error getting disk usage for %s: %v", partition.Mountpoint, err)
			continue
		}

		diskInfo.Partitions = append(diskInfo.Partitions, PartitionInfo{
			Device:     partition.Device,
			Mountpoint: partition.Mountpoint,
			FSType:     partition.Fstype,
			TotalSize:  formatBytes(usage.Total),
			Used:       formatBytes(usage.Used),
			Free:       formatBytes(usage.Free),
			Percentage: fmt.Sprintf("%.2f", usage.UsedPercent),
		})
	}

	// Get disk IO stats
	ioCounters, err := disk.IOCounters()
	if err == nil && len(ioCounters) > 0 {
		totalRead := uint64(0)
		totalWrite := uint64(0)
		for _, counter := range ioCounters {
			totalRead += counter.ReadBytes
			totalWrite += counter.WriteBytes
		}
		diskInfo.IOStats = IOStats{
			TotalRead:  formatBytes(totalRead),
			TotalWrite: formatBytes(totalWrite),
		}
	}

	return diskInfo, nil
}

// readMemFreeFromProc reads MemFree from /proc/meminfo
func readMemFreeFromProc() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemFree:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if memFree, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					return memFree * 1024 // Convert KB to bytes
				}
			}
		}
	}
	return 0
}

// formatBytes formats bytes to human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.2fB", float64(bytes))
	}

	div := unit
	expStr := "KMGTPE"
	for i := 0; i < len(expStr); i++ {
		if bytes < uint64(div)*unit {
			return fmt.Sprintf("%.2f%cB", float64(bytes)/float64(div), expStr[i])
		}
		div *= unit
	}

	return fmt.Sprintf("%.2fPB", float64(bytes)/float64(div))
}

// GetBootTimeAsFloat returns boot time as float for use in monitoring
func (s *SystemStats) GetBootTimeAsFloat() (float64, error) {
	hostStat, err := host.BootTime()
	if err != nil {
		return 0, err
	}
	return float64(hostStat), nil
}

// GetTotalCPUUsageAsFloat returns total CPU usage as float
func (s *SystemStats) GetTotalCPUUsageAsFloat() (float64, error) {
	totalUsage, err := cpu.Percent(0, false)
	if err != nil {
		return 0, err
	}
	if len(totalUsage) > 0 {
		return totalUsage[0], nil
	}
	return 0, fmt.Errorf("unable to get CPU usage")
}

// GetMemoryFreePercentage returns free memory as percentage
func (s *SystemStats) GetMemoryFreePercentage() (float64, error) {
	vMemory, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}

	totalFreeMemory := vMemory.Available
	if freeMem := readMemFreeFromProc(); freeMem > 0 {
		totalFreeMemory = uint64(freeMem)
	}

	freePercent := (float64(totalFreeMemory) / float64(vMemory.Total)) * 100
	return freePercent, nil
}

// GetDiskPartitions returns disk partitions for threshold checking
func (s *SystemStats) GetDiskPartitions() []PartitionInfo {
	return s.DiskInfo.Partitions
}
