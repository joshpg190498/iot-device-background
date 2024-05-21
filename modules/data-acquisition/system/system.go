package system

import (
	"fmt"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
)

type SystemInfo struct {
	Hostname string
	Platform string
	CPU      string
	RAM      string
	Disk     string
}

func GetSystemInfo() {
	info, _ := host.Info()
	fmt.Println("info:", info.HostID, info.Hostname, info.OS, info.Platform, info.PlatformFamily, info.PlatformVersion, info.KernelArch, info.KernelVersion, info.Procs)

	cpuInfo, _ := cpu.Info()
	for _, cpucpuInfoEle := range cpuInfo {
		fmt.Println("info", cpucpuInfoEle.CPU, cpucpuInfoEle.VendorID, cpucpuInfoEle.Family, cpucpuInfoEle.Model, cpucpuInfoEle.Mhz, cpucpuInfoEle.Cores)
	}

}
