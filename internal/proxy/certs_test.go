package proxy

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeCertHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "host and port", in: "example.com:443", want: "example.com"},
		{name: "plain host", in: "example.com", want: "example.com"},
		{name: "whitespace trims", in: "  example.com:8443 ", want: "example.com"},
		{name: "empty", in: "   ", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeCertHost(tt.in); got != tt.want {
				t.Fatalf("normalizeCertHost(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRandSerialNumber(t *testing.T) {
	t.Parallel()

	serial, err := randSerialNumber()
	if err != nil {
		t.Fatalf("randSerialNumber() error = %v", err)
	}
	if serial == nil {
		t.Fatal("serial is nil")
	}
	if serial.Sign() < 0 {
		t.Fatalf("serial must be non-negative, got %v", serial)
	}

	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	if serial.Cmp(limit) >= 0 {
		t.Fatalf("serial must be < 2^128, got %v", serial)
	}
}

func TestWriteFileAtomically(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "cert.pem")

	if err := writeFileAtomically(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("first writeFileAtomically() error = %v", err)
	}
	if err := writeFileAtomically(path, []byte("second"), 0o600); err != nil {
		t.Fatalf("second writeFileAtomically() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(data), "second"; got != want {
		t.Fatalf("file contents = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file permissions = %#o, want %#o", got, 0o600)
	}
}

func TestCertificateAuthority_Basics(t *testing.T) {
	t.Parallel()

	var nilCA *CertificateAuthority
	if got := nilCA.CertPath(); got != "" {
		t.Fatalf("nil CertPath() = %q, want empty", got)
	}
	if got := nilCA.WasCreated(); got {
		t.Fatalf("nil WasCreated() = %v, want false", got)
	}

	ca := newTestCA(t)
	ca.certPath = "/tmp/test-ca.pem"
	ca.wasCreated = true
	if got, want := ca.CertPath(), "/tmp/test-ca.pem"; got != want {
		t.Fatalf("CertPath() = %q, want %q", got, want)
	}
	if !ca.WasCreated() {
		t.Fatal("WasCreated() = false, want true")
	}
}

func TestCertificateForHost(t *testing.T) {
	t.Parallel()

	ca := newTestCA(t)

	t.Run("empty host returns error", func(t *testing.T) {
		t.Parallel()
		cert, err := ca.CertificateForHost("   ")
		if err == nil {
			t.Fatal("CertificateForHost() error = nil, want non-nil")
		}
		if cert != nil {
			t.Fatalf("cert = %#v, want nil", cert)
		}
	})

	t.Run("cache hit returns same pointer", func(t *testing.T) {
		t.Parallel()

		c1, err := ca.CertificateForHost("example.com:443")
		if err != nil {
			t.Fatalf("first CertificateForHost() error = %v", err)
		}
		c2, err := ca.CertificateForHost("example.com")
		if err != nil {
			t.Fatalf("second CertificateForHost() error = %v", err)
		}

		if c1 != c2 {
			t.Fatal("expected same certificate pointer from cache")
		}
	})

	t.Run("ip and dns SAN selection", func(t *testing.T) {
		t.Parallel()

		ipCert, err := ca.CertificateForHost("127.0.0.1:443")
		if err != nil {
			t.Fatalf("ip CertificateForHost() error = %v", err)
		}
		if ipCert.Leaf == nil {
			t.Fatal("ip cert leaf is nil")
		}
		if len(ipCert.Leaf.IPAddresses) == 0 {
			t.Fatal("ip cert should contain IP SAN")
		}
		if len(ipCert.Leaf.DNSNames) != 0 {
			t.Fatalf("ip cert DNSNames = %v, want empty", ipCert.Leaf.DNSNames)
		}

		dnsCert, err := ca.CertificateForHost("service.local")
		if err != nil {
			t.Fatalf("dns CertificateForHost() error = %v", err)
		}
		if dnsCert.Leaf == nil {
			t.Fatal("dns cert leaf is nil")
		}
		if len(dnsCert.Leaf.DNSNames) == 0 {
			t.Fatal("dns cert should contain DNS SAN")
		}
	})

	t.Run("evicts oldest entry over maxLeafCerts", func(t *testing.T) {
		t.Parallel()

		ca2 := newTestCA(t)
		for i := 0; i < maxLeafCerts+1; i++ {
			host := filepath.Base(filepath.Join("h", big.NewInt(int64(i)).String()+".example"))
			if _, err := ca2.CertificateForHost(host); err != nil {
				t.Fatalf("CertificateForHost(%q) error = %v", host, err)
			}
		}

		if len(ca2.leafOrder) != maxLeafCerts {
			t.Fatalf("leafOrder len = %d, want %d", len(ca2.leafOrder), maxLeafCerts)
		}
		if _, ok := ca2.leafCert["0.example"]; ok {
			t.Fatal("expected oldest cert to be evicted")
		}
	})
}

func TestIsTrustedBySystem(t *testing.T) {
	t.Parallel()

	var nilCA *CertificateAuthority
	_, err := nilCA.IsTrustedBySystem()
	if err == nil {
		t.Fatal("nil IsTrustedBySystem() error = nil, want non-nil")
	}

	ca := &CertificateAuthority{}
	_, err = ca.IsTrustedBySystem()
	if err == nil {
		t.Fatal("missing-cert IsTrustedBySystem() error = nil, want non-nil")
	}

	t.Run("untrusted generated CA returns false without error", func(t *testing.T) {
		t.Parallel()
		ca := newTestCA(t)

		trusted, err := ca.IsTrustedBySystem()
		if err != nil {
			t.Fatalf("IsTrustedBySystem() error = %v, want nil for unknown authority", err)
		}
		if trusted {
			t.Fatal("trusted = true, want false for generated test CA")
		}
	})
}

func TestEnsureCertificateAuthority(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	ca, err := EnsureCertificateAuthority()
	if err != nil {
		t.Fatalf("EnsureCertificateAuthority() error = %v", err)
	}
	if ca == nil {
		t.Fatal("EnsureCertificateAuthority() returned nil CA")
	}
	if ca.CertPath() == "" {
		t.Fatal("EnsureCertificateAuthority() returned empty cert path")
	}
	if _, statErr := os.Stat(ca.CertPath()); statErr != nil {
		t.Fatalf("expected cert on disk, stat error = %v", statErr)
	}
}

func TestWriteFileAtomically_ErrorPath(t *testing.T) {
	t.Parallel()

	err := writeFileAtomically(filepath.Join("/nope", "bad", "path.pem"), []byte("x"), 0o600)
	if err == nil {
		t.Fatal("writeFileAtomically() error = nil, want non-nil")
	}
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	// Accept platform-dependent fs errors as long as function fails.
}

func TestWriteFileAtomically_RenameErrorWhenTargetIsDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target-as-dir")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(targetDir) error = %v", err)
	}

	err := writeFileAtomically(targetDir, []byte("x"), 0o600)
	if err == nil {
		t.Fatal("writeFileAtomically() error = nil, want non-nil")
	}
}

// TODO: Add deterministic tests for loadOrCreateCertificateAuthority trust-store interactions.
