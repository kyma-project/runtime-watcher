package cacertificatehandler_test

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/runtime-watcher/skr/internal/cacertificatehandler"
	"github.com/kyma-project/runtime-watcher/skr/internal/tlstest"
)

func TestGetCertificatePool1(t *testing.T) {
	certPath := "ca.cert"
	tests := []struct {
		name             string
		certificateCount int
	}{
		{
			name:             "certificate pool with one certificate",
			certificateCount: 1,
		},
		{
			name:             "certificate pool with two certificates",
			certificateCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := writeCertificatesToFile(certPath, tt.certificateCount)
			require.NoError(t, err)

			got, err := cacertificatehandler.GetCertificatePool(certPath)
			require.NoError(t, err)
			require.False(t, got.Equal(x509.NewCertPool()))

			certificates, err := getCertificates(certPath)
			require.NoError(t, err)
			err = os.Remove(certPath)
			require.NoError(t, err)
			expectedCertPool := x509.NewCertPool()
			for _, certificate := range certificates {
				expectedCertPool.AddCert(certificate)
			}
			require.True(t, got.Equal(expectedCertPool))
		})
	}
}

func getCertificates(certPath string) ([]*x509.Certificate, error) {
	caCertBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("could not load CA certificate :%w", err)
	}
	var certs []*x509.Certificate
	remainingCert := caCertBytes
	for len(remainingCert) > 0 {
		var publicPemBlock *pem.Block
		publicPemBlock, remainingCert = pem.Decode(remainingCert)
		rootPubCrt, errParse := x509.ParseCertificate(publicPemBlock.Bytes)
		if errParse != nil {
			msg := "failed to parse public key"
			return nil, fmt.Errorf("%s :%w", msg, errParse)
		}
		certs = append(certs, rootPubCrt)
	}

	return certs, nil
}

func createCertificate() *x509.Certificate {
	sn, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	cert := &x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	return cert
}

func writeCertificatesToFile(certPath string, certificateCount int) error {
	for i := 0; i < certificateCount; i++ {
		rootKey, err := tlstest.GenerateRootKey()
		if err != nil {
			return fmt.Errorf("failed to generate root key: %w", err)
		}

		certificate := createCertificate()
		cert, err := tlstest.CreateCert(certificate, certificate, rootKey, rootKey)
		if err != nil {
			return fmt.Errorf("failed to create certificate: %w", err)
		}
		certBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Certificate[0],
		})
		file, err := os.OpenFile(certPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}

		_, err = file.Write(certBytes)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}

		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
	}
	return nil
}
