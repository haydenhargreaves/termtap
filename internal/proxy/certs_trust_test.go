package proxy

import (
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestIsTrustedBySystem_TrustedViaCertEnv(t *testing.T) {
	ca := newTestCA(t)

	rootFile := filepath.Join(t.TempDir(), "roots.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw})
	if err := os.WriteFile(rootFile, pemBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(root cert) error = %v", err)
	}

	t.Setenv("SSL_CERT_FILE", rootFile)
	t.Setenv("SSL_CERT_DIR", t.TempDir())

	trusted, err := ca.IsTrustedBySystem()
	if err != nil {
		t.Fatalf("IsTrustedBySystem() error = %v", err)
	}
	if !trusted {
		t.Fatal("trusted = false, want true when CA is in configured cert file")
	}
}
