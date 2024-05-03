package cacertificatehandler

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/go-logr/logr"
)

// TODO: Remove logger after debugging
func GetCertificatePool(certPath string, logger logr.Logger) (*x509.CertPool, error) {
	certBytes, err := getCertBytes(certPath)
	if err != nil {
		return nil, err
	}

	certificate, err := parseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	rootCertPool := x509.NewCertPool()
	// rootCertPool.AppendCertsFromPEM(certBytes)
	rootCertPool.AddCert(certificate)

	logger.Info("Certificate in root:" + string(len(rootCertPool.Subjects())))
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

func parseCertificate(certBytes []byte) (*x509.Certificate, error) {
	publicPemBlock, _ := pem.Decode(certBytes)
	rootPubCrt, errParse := x509.ParseCertificate(publicPemBlock.Bytes)
	if errParse != nil {
		msg := "failed to parse public key"
		return nil, fmt.Errorf("%s :%w", msg, errParse)
	}

	return rootPubCrt, nil
}
