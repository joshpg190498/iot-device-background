package system

// Métricas para la estrategia de agregación consciente de la salud del hardware.
// Solo librería estándar: leen directamente de /proc y /sys (Linux).
//
//   - cpu_freq : frecuencia REAL vs máxima de la CPU. Si cae por debajo del máximo
//                mientras el dispositivo entrena, hay throttling (térmico o por bajo
//                voltaje). En reposo es normal que baje (ahorro de energía).
//                En Raspberry Pi se lee del FIRMWARE (vcgencmd): la interfaz cpufreq
//                de Linux (scaling_cur_freq) muestra la frecuencia SOLICITADA y no
//                refleja el throttling del firmware (medido: sysfs 1800 MHz mientras
//                el clock real bajaba a 600–1600 MHz a 84–86 °C). En otros SBC se usa
//                cpufreq como respaldo. Se reporta la fuente usada.
//   - pressure : PSI (Pressure Stall Information) del kernel: porcentaje del tiempo
//                en que las tareas esperan CPU o memoria. Mide CONTENCIÓN, que es lo
//                que degrada el entrenamiento; el %CPU no la distingue porque el
//                propio entrenamiento satura la CPU. Requiere el parámetro de arranque
//                psi=1 en kernels con CONFIG_PSI_DEFAULT_DISABLED=y.

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Rutas configurables para poder probar con archivos de ejemplo.
var (
	cpufreqGlob  = "/sys/devices/system/cpu/cpufreq/policy*"
	pressureDir  = "/proc/pressure"
	vcgencmdName = "vcgencmd" // herramienta de firmware de Raspberry Pi
)

func getCPUFrequency() (map[string]interface{}, error) {
	maxMHz, sysCurMHz, err := readSysfsFreq()
	if err != nil {
		return nil, err
	}

	curMHz, source := sysCurMHz, "sysfs"
	out := map[string]interface{}{}
	if hz, ok := firmwareClockHz(); ok {
		curMHz, source = hz/1e6, "firmware"
	}
	if flags, ok := firmwareThrottled(); ok {
		// Bits (doc. Raspberry Pi): 0 bajo voltaje, 1 frecuencia recortada,
		// 2 throttling activo, 3 límite térmico soft; 16–19 = "ocurrió desde el arranque".
		out["throttledFlags"] = flags
		// "Ahora": recorte (1), throttling (2) o límite soft (3). Medido en NODE001: con
		// solo el bit 3 activo (0xe0008) la frecuencia real ya bajaba a ~1.53 GHz.
		out["throttledNow"] = flags&0xE != 0
	}

	out["curFreqMHz"] = round2(curMHz)
	out["maxFreqMHz"] = round2(maxMHz)
	out["freqRatio"] = round4(math.Min(1.0, curMHz/maxMHz)) // 1.0 = frecuencia máxima
	out["freqSource"] = source
	return out, nil
}

// readSysfsFreq: frecuencia máxima (hardware) y actual SOLICITADA según cpufreq, en MHz.
func readSysfsFreq() (maxMHz, curMHz float64, err error) {
	policies, err := filepath.Glob(cpufreqGlob)
	if err != nil || len(policies) == 0 {
		return 0, 0, fmt.Errorf("cpufreq no disponible en %s", cpufreqGlob)
	}
	var sumCur, sumMax float64
	var n int
	for _, p := range policies {
		cur, errCur := readIntFile(filepath.Join(p, "scaling_cur_freq"))
		max, errMax := readIntFile(filepath.Join(p, "cpuinfo_max_freq"))
		if errCur != nil || errMax != nil || max <= 0 {
			continue
		}
		sumCur += float64(cur)
		sumMax += float64(max)
		n++
	}
	if n == 0 {
		return 0, 0, fmt.Errorf("no se pudo leer la frecuencia de ninguna política cpufreq")
	}
	return sumMax / float64(n) / 1000.0, sumCur / float64(n) / 1000.0, nil // kHz -> MHz
}

// runVcgencmd ejecuta vcgencmd con timeout corto; ok=false si no existe o falla.
func runVcgencmd(args ...string) (string, bool) {
	path, err := exec.LookPath(vcgencmdName)
	if err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// firmwareClockHz: "frequency(48)=1531353472" -> 1531353472
func firmwareClockHz() (float64, bool) {
	out, ok := runVcgencmd("measure_clock", "arm")
	if !ok {
		return 0, false
	}
	_, v, found := strings.Cut(out, "=")
	if !found {
		return 0, false
	}
	hz, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || hz <= 0 {
		return 0, false
	}
	return hz, true
}

// firmwareThrottled: "throttled=0xe0008" -> 0xe0008
func firmwareThrottled() (int64, bool) {
	out, ok := runVcgencmd("get_throttled")
	if !ok {
		return 0, false
	}
	_, v, found := strings.Cut(out, "=")
	if !found {
		return 0, false
	}
	flags, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(v), "0x"), 16, 64)
	if err != nil {
		return 0, false
	}
	return flags, true
}

func getPressure() (map[string]interface{}, error) {
	cpu, err := readPSIFile(filepath.Join(pressureDir, "cpu"))
	if err != nil {
		return nil, fmt.Errorf("PSI no disponible (¿falta psi=1 en el arranque?): %w", err)
	}
	mem, err := readPSIFile(filepath.Join(pressureDir, "memory"))
	if err != nil {
		return nil, fmt.Errorf("PSI de memoria no disponible: %w", err)
	}

	// Los 'total' son contadores acumulados en microsegundos: permiten calcular la
	// presión EXACTA dentro de la ventana de entrenamiento (Δtotal / Δt), sin
	// depender del promedio móvil de 10 s.
	return map[string]interface{}{
		"cpuSomeAvg10": cpu["some"].avg10,
		"cpuSomeAvg60": cpu["some"].avg60,
		"cpuSomeTotal": cpu["some"].total,
		"memSomeAvg10": mem["some"].avg10,
		"memFullAvg10": mem["full"].avg10,
		"memSomeTotal": mem["some"].total,
		"memFullTotal": mem["full"].total,
	}, nil
}

type psiLine struct {
	avg10, avg60, avg300 float64
	total                uint64
}

// readPSIFile lee un archivo /proc/pressure/* con el formato:
//
//	some avg10=0.00 avg60=0.14 avg300=0.25 total=1472730
//	full avg10=0.00 avg60=0.00 avg300=0.00 total=0
func readPSIFile(path string) (map[string]psiLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parsePSI(bufio.NewScanner(f))
}

func parsePSI(sc *bufio.Scanner) (map[string]psiLine, error) {
	out := map[string]psiLine{}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		var l psiLine
		for _, kv := range fields[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			switch k {
			case "avg10":
				l.avg10, _ = strconv.ParseFloat(v, 64)
			case "avg60":
				l.avg60, _ = strconv.ParseFloat(v, 64)
			case "avg300":
				l.avg300, _ = strconv.ParseFloat(v, 64)
			case "total":
				l.total, _ = strconv.ParseUint(v, 10, 64)
			}
		}
		out[fields[0]] = l
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if _, ok := out["some"]; !ok {
		return nil, fmt.Errorf("formato PSI inesperado (sin línea 'some')")
	}
	return out, nil
}

func readIntFile(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }
func round4(x float64) float64 { return math.Round(x*10000) / 10000 }
