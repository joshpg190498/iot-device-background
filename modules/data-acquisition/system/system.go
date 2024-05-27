package system

import (
	"fmt"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"

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
