package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type CertificateBuilder struct {
	testingFramework *testing.T
	commonName       string
	serialNumber     *big.Int
	notBefore        time.Time
	notAfter         time.Time
	dnsNames         []string
}

func NewPemCertificateBuilder(testingFramework *testing.T) *CertificateBuilder {
	testingFramework.Helper()
	return &CertificateBuilder{
		testingFramework: testingFramework,
		commonName:       "test-cert",
		serialNumber:     big.NewInt(1),
		notBefore:        time.Now().Add(-time.Hour),
		notAfter:         time.Now().Add(time.Hour),
		dnsNames:         []string{"example.com"},
	}
}

func (builder *CertificateBuilder) WithCommonName(commonName string) *CertificateBuilder {
	builder.testingFramework.Helper()
	builder.commonName = commonName
	return builder
}

func (builder *CertificateBuilder) Build() string {
	builder.testingFramework.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(builder.testingFramework, err)
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
	require.NoError(builder.testingFramework, err)
	block := &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	return url.QueryEscape(string(pem.EncodeToMemory(block)))
}
