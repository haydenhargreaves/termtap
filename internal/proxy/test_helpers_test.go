package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

func drainEvents(t *testing.T, ch <-chan model.Event, n int, timeout time.Duration) []model.Event {
	t.Helper()

	events := make([]model.Event, 0, n)
	deadline := time.After(timeout)
	for len(events) < n {
		select {
		case ev := <-ch:
			events = append(events, ev)
		case <-deadline:
			t.Fatalf("timeout waiting for %d events, got %d", n, len(events))
		}
	}

	return events
}

func hasEventType(events []model.Event, typ model.EventType) bool {
	for _, ev := range events {
		if ev.Type == typ {
			return true
		}
	}
	return false
}

func newTestCA(t *testing.T) *CertificateAuthority {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	return &CertificateAuthority{
		cert:     cert,
		key:      key,
		leafCert: make(map[string]*tls.Certificate),
	}
}
