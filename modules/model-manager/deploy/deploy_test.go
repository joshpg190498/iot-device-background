package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper: servidor HTTP en memoria que sirve `content` (offline, sin red real).
func serve(content []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestDownloadVerifyActivate_OK(t *testing.T) {
	content := []byte("modelo tflite de prueba")
	srv := serve(content)
	defer srv.Close()

	dir := t.TempDir()
	res, err := DownloadVerifyActivate(dir, "fashion-cnn", 5, srv.URL, sha256Hex(content), true, 5*time.Second)
	if err != nil {
		t.Fatalf("esperaba éxito, error: %v", err)
	}
	if !res.SHA256OK || !res.Activated {
		t.Fatalf("esperaba SHA256OK y Activated true, got %+v", res)
	}

	// el artefacto quedó en versions/5/model.tflite
	if _, err := os.Stat(res.ModelPath); err != nil {
		t.Fatalf("no se encontró el artefacto: %v", err)
	}

	// 'current' apunta a versions/5 y se resuelve al modelo correcto
	current := filepath.Join(dir, "fashion-cnn", "current")
	target, err := os.Readlink(current)
	if err != nil {
		t.Fatalf("no se pudo leer symlink current: %v", err)
	}
	if filepath.Base(target) != "5" {
		t.Fatalf("current debe apuntar a versions/5, apunta a %s", target)
	}
	got, err := os.ReadFile(filepath.Join(current, "model.tflite"))
	if err != nil || string(got) != string(content) {
		t.Fatalf("el modelo activo no coincide: %v", err)
	}
}

func TestDownloadVerifyActivate_SHAMismatch(t *testing.T) {
	content := []byte("contenido bueno")
	srv := serve(content)
	defer srv.Close()

	dir := t.TempDir()
	badHash := sha256Hex([]byte("otra cosa"))
	res, err := DownloadVerifyActivate(dir, "fashion-cnn", 7, srv.URL, badHash, true, 5*time.Second)

	if err == nil {
		t.Fatal("esperaba error por sha256 no coincidente")
	}
	if res.SHA256OK || res.Activated {
		t.Fatalf("no debía marcar OK ni activar, got %+v", res)
	}
	// el artefacto corrupto NO debe conservarse
	modelPath := filepath.Join(dir, "fashion-cnn", "versions", "7", "model.tflite")
	if _, err := os.Stat(modelPath); !os.IsNotExist(err) {
		t.Fatalf("el artefacto corrupto no debía conservarse")
	}
	// no debe existir symlink 'current'
	if _, err := os.Lstat(filepath.Join(dir, "fashion-cnn", "current")); !os.IsNotExist(err) {
		t.Fatalf("no debía activarse ningún modelo")
	}
}

func TestActivateSecondVersionSwitchesSymlink(t *testing.T) {
	c1 := []byte("version uno")
	c2 := []byte("version dos")
	dir := t.TempDir()

	srv1 := serve(c1)
	_, err := DownloadVerifyActivate(dir, "m", 1, srv1.URL, sha256Hex(c1), true, 5*time.Second)
	srv1.Close()
	if err != nil {
		t.Fatalf("v1: %v", err)
	}

	srv2 := serve(c2)
	_, err = DownloadVerifyActivate(dir, "m", 2, srv2.URL, sha256Hex(c2), true, 5*time.Second)
	srv2.Close()
	if err != nil {
		t.Fatalf("v2: %v", err)
	}

	// current debe apuntar ahora a la versión 2
	current := filepath.Join(dir, "m", "current")
	got, err := os.ReadFile(filepath.Join(current, "model.tflite"))
	if err != nil || string(got) != string(c2) {
		t.Fatalf("current debía servir la versión 2, got %q err %v", string(got), err)
	}
}
