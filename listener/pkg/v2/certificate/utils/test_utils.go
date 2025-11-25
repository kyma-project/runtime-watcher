package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"time"
)

type CertificateBuilder struct {
	commonName   string
	serialNumber *big.Int
	notBefore    time.Time
	notAfter     time.Time
	dnsNames     []string
}

func NewPemCertificateBuilder() *CertificateBuilder {
	return &CertificateBuilder{
		commonName:   "test-cert",
		serialNumber: big.NewInt(1),
		notBefore:    time.Now().Add(-time.Hour),
		notAfter:     time.Now().Add(time.Hour),
		dnsNames:     []string{"example.com"},
	}
}

func (builder *CertificateBuilder) WithCommonName(commonName string) *CertificateBuilder {
	builder.commonName = commonName
	return builder
}

func (builder *CertificateBuilder) Build() (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("unable to generate ecdsa key: %w", err)
	}
	tmpl := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: builder.commonName,
		},
		SerialNumber:          builder.serialNumber,
		NotBefore:             builder.notBefore,
		NotAfter:              builder.notAfter,
		DNSNames:              builder.dnsNames,
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", fmt.Errorf("unable to create certificate: %w", err)
	}
	block := &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	return url.QueryEscape(string(pem.EncodeToMemory(block))), nil
}
