package system

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"

	models "ceiot-tf-sbc/modules/data-acquisition/models"
)

type FuncType func() (map[string]interface{}, error)

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

func getRAMUsage() (map[string]interface{}, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	ramUsage := map[string]interface{}{
		"totalRAM":       v.Total / 1024 / 1024, // MB
		"freeRAM":        v.Free / 1024 / 1024,  // MB
		"usedRAM":        v.Used / 1024 / 1024,  // MB
		"usedPercentRAM": math.Round(v.UsedPercent*100) / 100,
	}

	return ramUsage, nil

}

func getDiskUsage() (map[string]interface{}, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	diskUsages := make(map[string]interface{})
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			return nil, err
		}
		diskUsages[partition.Device] = map[string]interface{}{
			"totalDisk":       usage.Total / 1024 / 1024, // MB
			"freeDisk":        usage.Free / 1024 / 1024,  // MB
			"usedDisk":        usage.Used / 1024 / 1024,  // MB
			"UsedPercentDisk": math.Round(usage.UsedPercent*100) / 100,
		}
	}

	return diskUsages, nil
}

func getNetworkStats() (map[string]interface{}, error) {
	n, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	networkStats := make(map[string]interface{})
	for _, io := range n {
		data := map[string]interface{}{
			"bytesSent":   io.BytesSent,
			"bytesRecv":   io.BytesRecv,
			"packetsSent": io.PacketsSent,
			"packetsRecv": io.PacketsRecv,
			"errout":      io.Errout,
			"errin":       io.Errin,
			"dropin":      io.Dropin,
			"dropout":     io.Dropout,
		}
		networkStats[io.Name] = data
	}

	return networkStats, nil
}

func getNetworkInfo() (map[string]interface{}, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	networkInfo := make(map[string]interface{})
	for _, iface := range interfaces {
		data := map[string]interface{}{
			"mtu":          iface.MTU,
			"hardwareAddr": iface.HardwareAddr,
			"flags":        iface.Flags,
			"addrs":        iface.Addrs,
		}
		networkInfo[iface.Name] = data
	}

	return networkInfo, nil
}

func getCPUTemperature() (map[string]interface{}, error) {
	sensors, err := host.SensorsTemperatures()
	if err != nil {
		return nil, err
	}

	cpuTemps := make(map[string]interface{})
	for _, sensor := range sensors {
		if sensor.SensorKey == "coretemp" || sensor.SensorKey == "k10temp" || sensor.SensorKey == "k8temp" {
			cpuTemps[sensor.SensorKey] = sensor.Temperature
		}
	}

	return cpuTemps, nil
}

func getUptime() (map[string]interface{}, error) {
	uptime, err := host.Uptime()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"uptime": int(uptime / 60), // en minutos
	}, nil
}

func getLastReboot() (map[string]interface{}, error) {
	bootTime, err := host.BootTime()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"lastReboot": time.Unix(int64(bootTime), 0).UTC().Format(time.RFC3339), // utc
	}, nil
}

func getCPUUsage() (map[string]interface{}, error) {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"cpuUsage": math.Round(percentages[0]*100) / 100,
	}, nil
}

func getLoadAverage() (map[string]interface{}, error) {
	avg, err := load.Avg()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"loadAverage1m":  avg.Load1,
		"loadAverage5m":  avg.Load5,
		"loadAverage15m": avg.Load15,
	}, nil
}

func getSystemHealthFunctions() map[string]FuncType {
	return map[string]FuncType{
		"ram":         getRAMUsage,
		"disk":        getDiskUsage,
		"net_stats":   getNetworkStats,
		"net_info":    getNetworkInfo,
		"cpu_temp":    getCPUTemperature,
		"uptime":      getUptime,
		"last_reboot": getLastReboot,
		"cpu_usage":   getCPUUsage,
		"load_avg":    getLoadAverage,
	}
}

func CallFunctionByName(name string) (map[string]interface{}, error) {
	functions := getSystemHealthFunctions()
	if function, exists := functions[name]; exists {
		return function()
	}
	return nil, errors.New("function not found")
}
