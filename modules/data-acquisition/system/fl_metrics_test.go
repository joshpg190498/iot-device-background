package system

import (
	"os"
	"path/filepath"
	"testing"
)

// Monta un /sys y /proc falsos con los valores reales vistos en la Pi.
func fakeFS(t *testing.T, cur, max string, cpuPSI, memPSI string) {
	t.Helper()
	root := t.TempDir()
	pol := filepath.Join(root, "sys", "policy0")
	os.MkdirAll(pol, 0o755)
	os.WriteFile(filepath.Join(pol, "scaling_cur_freq"), []byte(cur+"\n"), 0o644)
	os.WriteFile(filepath.Join(pol, "cpuinfo_max_freq"), []byte(max+"\n"), 0o644)
	pr := filepath.Join(root, "pressure")
	os.MkdirAll(pr, 0o755)
	if cpuPSI != "" {
		os.WriteFile(filepath.Join(pr, "cpu"), []byte(cpuPSI), 0o644)
	}
	if memPSI != "" {
		os.WriteFile(filepath.Join(pr, "memory"), []byte(memPSI), 0o644)
	}
	oldGlob, oldDir, oldVc := cpufreqGlob, pressureDir, vcgencmdName
	cpufreqGlob, pressureDir = filepath.Join(root, "sys", "policy*"), pr
	vcgencmdName = "vcgencmd-inexistente" // por defecto: sin firmware (otro SBC)
	t.Cleanup(func() { cpufreqGlob, pressureDir, vcgencmdName = oldGlob, oldDir, oldVc })
}

// fakeVcgencmd instala un vcgencmd falso que responde como la Pi real.
func fakeVcgencmd(t *testing.T, clock, throttled string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n measure_clock) echo \"" + clock + "\" ;;\n get_throttled) echo \"" + throttled + "\" ;;\nesac\n"
	path := filepath.Join(dir, "vcgencmd-fake")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	vcgencmdName = path
}

const piCPU = "some avg10=0.00 avg60=0.14 avg300=0.25 total=1472730\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n"
const piMem = "some avg10=1.50 avg60=0.00 avg300=0.00 total=724\nfull avg10=0.25 avg60=0.00 avg300=0.00 total=676\n"

func TestCPUFrequency_SysfsFallback(t *testing.T) {
	fakeFS(t, "600000", "1800000", piCPU, piMem)
	d, err := CallFunctionByName("cpu_freq")
	if err != nil {
		t.Fatal(err)
	}
	if d["curFreqMHz"] != 600.0 || d["maxFreqMHz"] != 1800.0 || d["freqRatio"] != 0.3333 || d["freqSource"] != "sysfs" {
		t.Fatalf("valores inesperados: %v", d)
	}
	if _, ok := d["throttledFlags"]; ok {
		t.Fatal("sin firmware no debía reportar flags")
	}
}

// Caso medido en NODE001 (13/10, 84.7 °C): sysfs decía 1800 MHz, el firmware 600 MHz.
func TestCPUFrequency_FirmwareSeesThrottlingSysfsMisses(t *testing.T) {
	fakeFS(t, "1800000", "1800000", piCPU, piMem)
	fakeVcgencmd(t, "frequency(48)=600169920", "throttled=0xe0006")
	d, err := CallFunctionByName("cpu_freq")
	if err != nil {
		t.Fatal(err)
	}
	if d["freqSource"] != "firmware" || d["curFreqMHz"] != 600.17 || d["freqRatio"] != 0.3334 {
		t.Fatalf("debía usar la frecuencia real del firmware: %v", d)
	}
	if d["throttledFlags"] != int64(0xe0006) || d["throttledNow"] != true {
		t.Fatalf("flags inesperados: %v", d)
	}
}

// 0xe0008: solo límite térmico soft (bit 3), frecuencia real 1.53 GHz.
func TestCPUFrequency_SoftLimitCountsAsThrottling(t *testing.T) {
	fakeFS(t, "1800000", "1800000", piCPU, piMem)
	fakeVcgencmd(t, "frequency(48)=1531353472", "throttled=0xe0008")
	d, _ := CallFunctionByName("cpu_freq")
	if d["freqRatio"] != 0.8508 || d["throttledNow"] != true {
		t.Fatalf("valores inesperados: %v", d)
	}
}

// Solo bits históricos (16-19): ocurrió antes, no ahora.
func TestCPUFrequency_HistoricalFlagsOnly(t *testing.T) {
	fakeFS(t, "1800000", "1800000", piCPU, piMem)
	fakeVcgencmd(t, "frequency(48)=1800457088", "throttled=0xe0000")
	d, _ := CallFunctionByName("cpu_freq")
	if d["freqRatio"] != 1.0 || d["throttledNow"] != false {
		t.Fatalf("valores inesperados: %v", d)
	}
}

func TestCPUFrequency_Full(t *testing.T) {
	fakeFS(t, "1800000", "1800000", piCPU, piMem)
	d, _ := CallFunctionByName("cpu_freq")
	if d["freqRatio"] != 1.0 {
		t.Fatalf("esperaba ratio 1.0, got %v", d)
	}
}

func TestPressure_ParsesPiOutput(t *testing.T) {
	fakeFS(t, "1800000", "1800000", piCPU, piMem)
	d, err := CallFunctionByName("pressure")
	if err != nil {
		t.Fatal(err)
	}
	if d["cpuSomeAvg60"] != 0.14 || d["cpuSomeTotal"] != uint64(1472730) ||
		d["memSomeAvg10"] != 1.5 || d["memFullAvg10"] != 0.25 || d["memFullTotal"] != uint64(676) {
		t.Fatalf("valores inesperados: %v", d)
	}
}

func TestPressure_MissingPSI_ClearError(t *testing.T) {
	fakeFS(t, "1800000", "1800000", "", "")
	if _, err := CallFunctionByName("pressure"); err == nil {
		t.Fatal("sin /proc/pressure debía devolver error")
	}
}
