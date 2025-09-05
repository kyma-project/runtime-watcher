package cacertificatehandler

import (
	"crypto/x509"
	"fmt"
	"os"
)

func GetCertificatePool(certPath string) (*x509.CertPool, error) {
	certBytes, err := getCertBytes(certPath)
	if err != nil {
		return nil, err
	}
	rootCertPool := x509.NewCertPool()
	ok := rootCertPool.AppendCertsFromPEM(certBytes)
	if !ok {
		msg := "failed to append certificate to pool"
		return nil, fmt.Errorf("%s :%w", msg, err)
	}
	return rootCertPool, nil
}

func getCertBytes(certPath string) ([]byte, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		msg := "could not load CA certificate"
		return nil, fmt.Errorf("%s :%w", msg, err)
	}

	return certBytes, nil
}
