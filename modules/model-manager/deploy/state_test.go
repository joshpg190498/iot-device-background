package deploy

import (
	"testing"
	"time"
)

func TestValidateCommand(t *testing.T) {
	good := sha256Hex([]byte("x"))
	cases := []struct {
		name    string
		idModel string
		version int
		url     string
		sha     string
		wantErr bool
	}{
		{"ok", "fashion-cnn", 1, "http://srv/m.tflite", good, false},
		{"path traversal", "../etc", 1, "http://srv/m", good, true},
		{"slash en id", "a/b", 1, "http://srv/m", good, true},
		{"version cero", "m", 0, "http://srv/m", good, true},
		{"url vacía", "m", 1, "", good, true},
		{"sha corto", "m", 1, "http://srv/m", "abc", true},
	}
	for _, c := range cases {
		err := ValidateCommand(c.idModel, c.version, c.url, c.sha)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: wantErr=%v got %v", c.name, c.wantErr, err)
		}
	}
}

func TestCurrentVersionAndReactivate(t *testing.T) {
	dir := t.TempDir()

	// sin despliegues: versión 0, sin error
	if v, err := CurrentVersion(dir, "m"); err != nil || v != 0 {
		t.Fatalf("esperaba (0,nil), got (%d,%v)", v, err)
	}

	c1, c2 := []byte("uno"), []byte("dos")
	for i, c := range [][]byte{c1, c2} {
		srv := serve(c)
		if _, err := DownloadVerifyActivate(dir, "m", i+1, srv.URL, sha256Hex(c), true, 5*time.Second); err != nil {
			t.Fatalf("v%d: %v", i+1, err)
		}
		srv.Close()
	}
	if v, _ := CurrentVersion(dir, "m"); v != 2 {
		t.Fatalf("current debía ser 2, es %d", v)
	}

	// rollback a v1 sin red: ya está en disco y verificada
	if !HasVerifiedVersion(dir, "m", 1, sha256Hex(c1)) {
		t.Fatal("v1 debía estar verificada en disco")
	}
	if err := ActivateExisting(dir, "m", 1); err != nil {
		t.Fatalf("reactivando v1: %v", err)
	}
	if v, _ := CurrentVersion(dir, "m"); v != 1 {
		t.Fatalf("current debía volver a 1, es %d", v)
	}
}
