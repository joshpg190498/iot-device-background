package system

import (
	"fmt"
	"log"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"

	models "ceiot-tf-sbc/modules/data-acquisition/models"
)

func GetDeviceInfo(deviceID string) ([]models.Device, error) {
	var devices []models.Device

	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	cpuInfo, _ := cpu.Info()

	devices = append(devices, models.Device{
		IDDevice: deviceID,
		Field:    "hostname",
		Value:    hostInfo.Hostname,
	})
	devices = append(devices, models.Device{
		IDDevice: deviceID,
		Field:    "processor",
		Value:    fmt.Sprintf("%s %s @ %.2f GHz", cpuInfo[0].ModelName, cpuInfo[0].VendorID, cpuInfo[0].Mhz/1000.0),
	})

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	devices = append(devices, models.Device{
		IDDevice: deviceID,
		Field:    "ram",
		Value:    fmt.Sprintf("%.2f GB", float64(memInfo.Total)/1024/1024/1024),
	})

	devices = append(devices, models.Device{
		IDDevice: deviceID,
		Field:    "hostID",
		Value:    hostInfo.HostID,
	})
	devices = append(devices, models.Device{
		IDDevice: deviceID,
		Field:    "os",
		Value:    fmt.Sprintf("%s, %s", hostInfo.OS, hostInfo.PlatformFamily),
	})
	devices = append(devices, models.Device{
		IDDevice: deviceID,
		Field:    "kernel",
		Value:    hostInfo.KernelVersion,
	})

	return devices, nil
}

func GetCpuInfo() {
	cpuUsage, _ := cpu.Percent(0, false)
	log.Printf("   Uso de CPU: %.2f%%\n", cpuUsage[0])
	memInfo, _ := mem.VirtualMemory()
	fmt.Printf("   Total: %v, Libre: %v, Usado: %v\n", memInfo.Total, memInfo.Free, memInfo.Used)
	diskInfo, _ := disk.Partitions(false)
	log.Println(diskInfo)
	netInfo, _ := net.IOCounters(false)
	for _, net := range netInfo {
		fmt.Printf("   Nombre: %v, Bytes recibidos: %v, Bytes enviados: %v\n", net.Name, net.BytesRecv, net.BytesSent)
	}
	avg, err := load.Avg()
	if err != nil {
		fmt.Printf("Error obteniendo la carga promedio: %v", err)
		return
	}

	log.Printf("Load Average - 1 min: %.2f, 5 min: %.2f, 15 min: %.2f\n", avg.Load1, avg.Load5, avg.Load15)
}
