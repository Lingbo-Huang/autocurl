package capture

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"
)

type Authority struct {
	root        *x509.Certificate
	rootKey     *rsa.PrivateKey
	leafKey     *rsa.PrivateKey
	caPEM       []byte
	certificate map[string]*tls.Certificate
	mu          sync.Mutex
}

func NewAuthority() (*Authority, error) {
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate root key: %w", err)
	}
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}

	now := time.Now()
	root := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Subject:               pkix.Name{CommonName: "autocurl ephemeral local CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, fmt.Errorf("create root certificate: %w", err)
	}

	return &Authority{
		root:        root,
		rootKey:     rootKey,
		leafKey:     leafKey,
		caPEM:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		certificate: make(map[string]*tls.Certificate),
	}, nil
}

func (a *Authority) PEM() []byte {
	return append([]byte(nil), a.caPEM...)
}

func (a *Authority) Certificate(hostPort string) (*tls.Certificate, error) {
	host := hostPort
	if parsedHost, _, err := net.SplitHostPort(hostPort); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")

	a.mu.Lock()
	defer a.mu.Unlock()
	if certificate := a.certificate[host]; certificate != nil {
		return certificate, nil
	}

	now := time.Now()
	leaf := &x509.Certificate{
		SerialNumber: randomSerial(),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		leaf.IPAddresses = []net.IP{ip}
	} else {
		leaf.DNSNames = []string{host}
	}

	der, err := x509.CreateCertificate(rand.Reader, leaf, a.root, &a.leafKey.PublicKey, a.rootKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate for %s: %w", host, err)
	}
	keyDER := x509.MarshalPKCS1PrivateKey(a.leafKey)
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}),
	)
	if err != nil {
		return nil, fmt.Errorf("load certificate for %s: %w", host, err)
	}
	a.certificate[host] = &certificate
	return &certificate, nil
}

func randomSerial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return serial
}
