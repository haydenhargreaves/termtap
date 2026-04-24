package proxy

import (
	"os"
	"testing"
)

func TestLoadOrCreateCertificateAuthority_CreateThenLoad(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	ca1, err := loadOrCreateCertificateAuthority()
	if err != nil {
		t.Fatalf("first loadOrCreateCertificateAuthority() error = %v", err)
	}
	if ca1 == nil {
		t.Fatal("first CA is nil")
	}
	if !ca1.WasCreated() {
		t.Fatal("first CA should report WasCreated=true")
	}
	if ca1.CertPath() == "" {
		t.Fatal("first CA CertPath is empty")
	}
	if _, err := os.Stat(ca1.CertPath()); err != nil {
		t.Fatalf("first CA cert file missing: %v", err)
	}

	ca2, err := loadOrCreateCertificateAuthority()
	if err != nil {
		t.Fatalf("second loadOrCreateCertificateAuthority() error = %v", err)
	}
	if ca2 == nil {
		t.Fatal("second CA is nil")
	}
	if ca2.WasCreated() {
		t.Fatal("second CA should report WasCreated=false (loaded existing)")
	}
	if ca2.CertPath() != ca1.CertPath() {
		t.Fatalf("cert path mismatch: first=%q second=%q", ca1.CertPath(), ca2.CertPath())
	}
	if ca2.cert == nil || ca2.key == nil {
		t.Fatal("loaded CA should include cert and key")
	}
}
