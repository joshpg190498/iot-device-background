package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	idModelRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	sha256Re  = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)
)

// ValidateCommand valida los campos que terminan formando rutas en disco o que
// se usan para verificar integridad. Evita path traversal (id_model = "../..")
// y comandos malformados antes de tocar el filesystem.
func ValidateCommand(idModel string, version int, url, sha string) error {
	if !idModelRe.MatchString(idModel) || strings.Contains(idModel, "..") {
		return fmt.Errorf("id_model inválido: %q", idModel)
	}
	if version <= 0 {
		return fmt.Errorf("version inválida: %d", version)
	}
	if url == "" {
		return fmt.Errorf("url vacía")
	}
	if !sha256Re.MatchString(sha) {
		return fmt.Errorf("sha256 inválido (se esperan 64 caracteres hex)")
	}
	return nil
}

// CurrentVersion devuelve la versión a la que apunta el symlink 'current' de
// un modelo. Devuelve (0, nil) si el modelo aún no tiene versión activa.
func CurrentVersion(modelsDir, idModel string) (int, error) {
	target, err := os.Readlink(filepath.Join(modelsDir, idModel, "current"))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(filepath.Base(target))
	if err != nil {
		return 0, fmt.Errorf("symlink 'current' apunta a un destino inesperado: %s", target)
	}
	return v, nil
}

// HasVerifiedVersion indica si versions/<version>/model.tflite ya existe en
// disco y su sha256 coincide con el esperado (permite reactivar sin descargar).
func HasVerifiedVersion(modelsDir, idModel string, version int, expectedSHA256 string) bool {
	p := filepath.Join(modelsDir, idModel, "versions", strconv.Itoa(version), "model.tflite")
	sum, err := FileSHA256(p)
	return err == nil && strings.EqualFold(sum, expectedSHA256)
}

// ActivateExisting activa una versión ya presente en disco (sin descargar).
func ActivateExisting(modelsDir, idModel string, version int) error {
	base := filepath.Join(modelsDir, idModel)
	return activateSymlink(filepath.Join(base, "current"),
		filepath.Join(base, "versions", strconv.Itoa(version)))
}
