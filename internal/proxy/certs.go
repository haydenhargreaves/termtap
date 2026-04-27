package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	caDirName    = "termtap"
	caCertName   = "mitm-ca-cert.pem"
	caKeyName    = "mitm-ca-key.pem"
	caValidFor   = 10 * 365 * 24 * time.Hour
	leafValidFor = 7 * 24 * time.Hour
	maxLeafCerts = 256
)

type CertificateAuthority struct {
	cert       *x509.Certificate
	key        *ecdsa.PrivateKey
	certPath   string
	keyPath    string
	wasCreated bool

	mu        sync.Mutex
	leafCert  map[string]*tls.Certificate
	leafOrder []string
}

func loadOrCreateCertificateAuthority() (*CertificateAuthority, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}

	baseDir := filepath.Join(configDir, caDirName)
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	ca := &CertificateAuthority{
		certPath: filepath.Join(baseDir, caCertName),
		keyPath:  filepath.Join(baseDir, caKeyName),
		leafCert: make(map[string]*tls.Certificate),
	}

	if _, err := os.Stat(ca.certPath); err == nil {
		if _, err := os.Stat(ca.keyPath); err == nil {
			if err := ca.load(); err != nil {
				return nil, err
			}
			return ca, nil
		}
	}

	if err := ca.create(); err != nil {
		return nil, err
	}

	ca.wasCreated = true
	return ca, nil
}

func (ca *CertificateAuthority) load() error {
	certPEM, err := os.ReadFile(ca.certPath)
	if err != nil {
		return fmt.Errorf("read ca cert: %w", err)
	}

	keyPEM, err := os.ReadFile(ca.keyPath)
	if err != nil {
		return fmt.Errorf("read ca key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("decode ca cert pem")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse ca cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("decode ca key pem")
	}

	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse ca key: %w", err)
	}

	ca.cert = cert
	ca.key = key
	return nil
}

func (ca *CertificateAuthority) create() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ca key: %w", err)
	}

	serial, err := randSerialNumber()
	if err != nil {
		return err
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "termtap Local MITM CA",
			Organization: []string{"termtap"},
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(caValidFor),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create ca cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal ca key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := writeFileAtomically(ca.certPath, certPEM, 0o600); err != nil {
		return fmt.Errorf("write ca cert: %w", err)
	}
	if err := writeFileAtomically(ca.keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write ca key: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("parse created ca cert: %w", err)
	}

	ca.cert = cert
	ca.key = key
	return nil
}

func (ca *CertificateAuthority) CertificateForHost(host string) (*tls.Certificate, error) {
	host = normalizeCertHost(host)
	if host == "" {
		return nil, fmt.Errorf("empty host for certificate")
	}

	ca.mu.Lock()
	defer ca.mu.Unlock()

	if cert, ok := ca.leafCert[host]; ok {
		return cert, nil
	}

	serial, err := randSerialNumber()
	if err != nil {
		return nil, err
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(leafValidFor),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &leafKey.PublicKey, ca.key)
	if err != nil {
		return nil, fmt.Errorf("create leaf cert: %w", err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{der, ca.cert.Raw},
		PrivateKey:  leafKey,
	}
	leafParsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse leaf cert: %w", err)
	}
	tlsCert.Leaf = leafParsed

	ca.leafCert[host] = tlsCert
	ca.leafOrder = append(ca.leafOrder, host)
	if len(ca.leafOrder) > maxLeafCerts {
		evicted := ca.leafOrder[0]
		ca.leafOrder = ca.leafOrder[1:]
		delete(ca.leafCert, evicted)
	}

	return tlsCert, nil
}

func (ca *CertificateAuthority) CertPath() string {
	if ca == nil {
		return ""
	}
	return ca.certPath
}

func (ca *CertificateAuthority) WasCreated() bool {
	if ca == nil {
		return false
	}
	return ca.wasCreated
}

func (ca *CertificateAuthority) IsTrustedBySystem() (bool, error) {
	if ca == nil || ca.cert == nil {
		return false, fmt.Errorf("certificate authority is unavailable")
	}

	roots, err := x509.SystemCertPool()
	if err != nil {
		return false, fmt.Errorf("load system cert pool: %w", err)
	}
	if roots == nil {
		return false, nil
	}

	_, err = ca.cert.Verify(x509.VerifyOptions{Roots: roots})
	if err == nil {
		return true, nil
	}

	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return false, nil
	}

	return false, err
}

func EnsureCertificateAuthority() (*CertificateAuthority, error) {
	return loadOrCreateCertificateAuthority()
}

func randSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}
	return serial, nil
}

func normalizeCertHost(hostport string) string {
	host := strings.TrimSpace(hostport)
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}

	return host
}

func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".termtap-tmp-*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}
