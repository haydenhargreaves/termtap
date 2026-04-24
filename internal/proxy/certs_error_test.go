package proxy

import (
	"crypto/tls"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateCertificateAuthority_RecreatesWhenKeyMissing(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	baseDir := filepath.Join(configRoot, caDirName)
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	certPath := filepath.Join(baseDir, caCertName)
	if err := os.WriteFile(certPath, []byte("stale-cert"), 0o600); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}

	ca, err := loadOrCreateCertificateAuthority()
	if err != nil {
		t.Fatalf("loadOrCreateCertificateAuthority() error = %v", err)
	}
	if !ca.WasCreated() {
		t.Fatal("WasCreated = false, want true when key is missing")
	}
	if _, err := os.Stat(filepath.Join(baseDir, caKeyName)); err != nil {
		t.Fatalf("expected key file to be created, stat error = %v", err)
	}
}

func TestLoadOrCreateCertificateAuthority_LoadErrorOnCorruptFiles(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	baseDir := filepath.Join(configRoot, caDirName)
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	certPath := filepath.Join(baseDir, caCertName)
	keyPath := filepath.Join(baseDir, caKeyName)
	if err := os.WriteFile(certPath, []byte("not-a-pem"), 0o600); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("not-a-pem"), 0o600); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}

	_, err := loadOrCreateCertificateAuthority()
	if err == nil {
		t.Fatal("loadOrCreateCertificateAuthority() error = nil, want non-nil")
	}
}

func TestCertificateAuthorityLoad_ErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		certBytes []byte
		keyBytes  []byte
		wantPart  string
	}{
		{
			name:      "invalid cert pem",
			certBytes: []byte("bad-cert"),
			keyBytes:  []byte("bad-key"),
			wantPart:  "decode ca cert pem",
		},
		{
			name:      "parse cert fails",
			certBytes: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("bogus")}),
			keyBytes:  []byte("bad-key"),
			wantPart:  "parse ca cert",
		},
		{
			name:      "invalid key pem",
			certBytes: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: newTestCA(t).cert.Raw}),
			keyBytes:  []byte("bad-key"),
			wantPart:  "decode ca key pem",
		},
		{
			name:      "parse key fails",
			certBytes: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: newTestCA(t).cert.Raw}),
			keyBytes:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("bogus")}),
			wantPart:  "parse ca key",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			ca := &CertificateAuthority{
				certPath: filepath.Join(dir, caCertName),
				keyPath:  filepath.Join(dir, caKeyName),
				leafCert: make(map[string]*tls.Certificate),
			}

			if err := os.WriteFile(ca.certPath, tt.certBytes, 0o600); err != nil {
				t.Fatalf("write cert file error = %v", err)
			}
			if err := os.WriteFile(ca.keyPath, tt.keyBytes, 0o600); err != nil {
				t.Fatalf("write key file error = %v", err)
			}

			err := ca.load()
			if err == nil {
				t.Fatal("load() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantPart) {
				t.Fatalf("load() error = %q, want contains %q", err.Error(), tt.wantPart)
			}
		})
	}
}

func TestCertificateAuthorityCreate_ErrorWhenWritePathInvalid(t *testing.T) {
	t.Parallel()

	ca := &CertificateAuthority{
		certPath: filepath.Join("/nope", "missing", "ca-cert.pem"),
		keyPath:  filepath.Join("/nope", "missing", "ca-key.pem"),
		leafCert: make(map[string]*tls.Certificate),
	}

	err := ca.create()
	if err == nil {
		t.Fatal("create() error = nil, want non-nil")
	}
}

func TestCertificateAuthorityCreate_WriteErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("write ca cert wraps error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		badCertPath := filepath.Join(dir, "cert-as-dir")
		if err := os.MkdirAll(badCertPath, 0o700); err != nil {
			t.Fatalf("MkdirAll(cert dir) error = %v", err)
		}

		ca := &CertificateAuthority{
			certPath: badCertPath,
			keyPath:  filepath.Join(dir, "ca-key.pem"),
			leafCert: make(map[string]*tls.Certificate),
		}

		err := ca.create()
		if err == nil {
			t.Fatal("create() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "write ca cert") {
			t.Fatalf("create() error = %q, want contains %q", err.Error(), "write ca cert")
		}
	})

	t.Run("write ca key wraps error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		badKeyPath := filepath.Join(dir, "key-as-dir")
		if err := os.MkdirAll(badKeyPath, 0o700); err != nil {
			t.Fatalf("MkdirAll(key dir) error = %v", err)
		}

		ca := &CertificateAuthority{
			certPath: filepath.Join(dir, "ca-cert.pem"),
			keyPath:  badKeyPath,
			leafCert: make(map[string]*tls.Certificate),
		}

		err := ca.create()
		if err == nil {
			t.Fatal("create() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "write ca key") {
			t.Fatalf("create() error = %q, want contains %q", err.Error(), "write ca key")
		}
	})
}

func TestCertificateAuthorityLoad_ReadErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("read cert failure", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		ca := &CertificateAuthority{
			certPath: filepath.Join(dir, "missing-cert.pem"),
			keyPath:  filepath.Join(dir, "missing-key.pem"),
			leafCert: make(map[string]*tls.Certificate),
		}

		err := ca.load()
		if err == nil {
			t.Fatal("load() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "read ca cert") {
			t.Fatalf("load() error = %q, want contains %q", err.Error(), "read ca cert")
		}
	})

	t.Run("read key failure", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		goodCA := newTestCA(t)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: goodCA.cert.Raw})

		certPath := filepath.Join(dir, "cert.pem")
		if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
			t.Fatalf("WriteFile(cert) error = %v", err)
		}

		ca := &CertificateAuthority{
			certPath: certPath,
			keyPath:  filepath.Join(dir, "missing-key.pem"),
			leafCert: make(map[string]*tls.Certificate),
		}

		err := ca.load()
		if err == nil {
			t.Fatal("load() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "read ca key") {
			t.Fatalf("load() error = %q, want contains %q", err.Error(), "read ca key")
		}
	})
}
