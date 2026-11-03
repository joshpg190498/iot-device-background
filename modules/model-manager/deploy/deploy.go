// Package deploy implementa la lógica de despliegue de un modelo en el
// dispositivo: descarga del artefacto (.tflite) por HTTP, verificación de
// integridad (sha256) y activación por symlink atómico. No depende de MQTT ni
// de otros paquetes del proyecto, para poder probarse de forma aislada.
//
// Layout de artefactos en el dispositivo:
//
//	$MODELS_DIR/<id_model>/
//	  versions/<version>/model.tflite
//	  current -> versions/<version>     (symlink al modelo activo)
package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Result describe el resultado de una activación de modelo.
type Result struct {
	VersionDir string // ruta a versions/<version>
	ModelPath  string // ruta al .tflite de esta versión
	SHA256OK   bool   // true si el hash coincidió
	Activated  bool   // true si se actualizó el symlink 'current'
}

// Download descarga url a destPath. Escribe primero a un archivo temporal y
// renombra al final, para no dejar un artefacto parcial si la descarga se corta.
func Download(url, destPath string, timeout time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creando directorio destino: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("descargando %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga %s: status HTTP %d", url, resp.StatusCode)
	}

	tmp := destPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creando archivo temporal: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("escribiendo artefacto: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, destPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renombrando artefacto: %w", err)
	}
	return nil
}

// FileSHA256 calcula el sha256 (hex) del contenido de un archivo.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// activateSymlink actualiza el symlink 'current' de forma atómica: crea un
// symlink temporal y lo renombra sobre el existente (rename es atómico).
func activateSymlink(currentLink, target string) error {
	tmp := currentLink + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, currentLink); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// DownloadVerifyActivate ejecuta el flujo completo de despliegue.
//
// Si el sha256 NO coincide: borra el artefacto corrupto, NO activa y devuelve
// error con SHA256OK=false (nunca se despliega un modelo corrupto).
func DownloadVerifyActivate(
	modelsDir, idModel string,
	version int,
	url, expectedSHA256 string,
	activate bool,
	timeout time.Duration,
) (Result, error) {
	base := filepath.Join(modelsDir, idModel)
	versionDir := filepath.Join(base, "versions", strconv.Itoa(version))
	modelPath := filepath.Join(versionDir, "model.tflite")

	if err := Download(url, modelPath, timeout); err != nil {
		return Result{}, err
	}

	sum, err := FileSHA256(modelPath)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(sum, expectedSHA256) {
		os.Remove(modelPath) // artefacto corrupto: no se conserva
		return Result{SHA256OK: false}, fmt.Errorf(
			"sha256 no coincide: esperado=%s obtenido=%s", expectedSHA256, sum,
		)
	}

	res := Result{VersionDir: versionDir, ModelPath: modelPath, SHA256OK: true}
	if activate {
		if err := activateSymlink(filepath.Join(base, "current"), versionDir); err != nil {
			return res, fmt.Errorf("activando symlink 'current': %w", err)
		}
		res.Activated = true
	}
	return res, nil
}