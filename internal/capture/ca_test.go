package capture

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestAuthoritySignsHostCertificate(t *testing.T) {
	authority, err := NewAuthority()
	if err != nil {
		t.Fatalf("NewAuthority returned an error: %v", err)
	}
	certificate, err := authority.Certificate("api.example.test:443")
	if err != nil {
		t.Fatalf("Certificate returned an error: %v", err)
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf certificate: %v", err)
	}
	rootBlock, _ := pem.Decode(authority.PEM())
	if rootBlock == nil {
		t.Fatal("authority PEM did not contain a certificate")
	}
	root, err := x509.ParseCertificate(rootBlock.Bytes)
	if err != nil {
		t.Fatalf("parse root certificate: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	if _, err := leaf.Verify(x509.VerifyOptions{
		DNSName: "api.example.test",
		Roots:   roots,
	}); err != nil {
		t.Fatalf("verify leaf certificate: %v", err)
	}
}
