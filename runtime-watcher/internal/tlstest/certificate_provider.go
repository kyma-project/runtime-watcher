package tlstest

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
	"os"
	"time"
)

// CertProvider generates certificates for envTest mTLS setup.
type CertProvider struct {
	RootCert   *tls.Certificate
	ServerCert *tls.Certificate

	RootCertFile   *os.File
	ClientCertFile *os.File
	ClientKeyFile  *os.File
}

const (
	privateKeyBits             = 2048
	certSerialNumberUpperLimit = 128
	tempFilePermission         = 0o600
)

const (
	errMsgCreatingNewCertProvider = "creation of CertProvider for TLS setup failed"
	errMsgCreatingTempFiles       = "creation of temp files failed"
	errMsgDeletingTempFiles       = "deletion of temp files failed"
	errMsgCreatingPrivateKey      = "creation of private key failed"
	errMsgCreatingTLSCertificates = "creation of certificate failed"
	errMsgWritingTempFiles        = "writing of temp files failed"
)

// NewCertProvider creates a new CertProvider with TLS certificates generated on the fly.
// Use the CleanUp() function to remove temporary certificate files when done.
func NewCertProvider() (*CertProvider, error) {
	provider := CertProvider{}
	err := provider.createTempFiles()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsgCreatingNewCertProvider, err)
	}
	err = provider.GenerateCerts()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsgCreatingNewCertProvider, err)
	}
	return &provider, nil
}

func (p *CertProvider) createTempFiles() error {
	var err error
	p.RootCertFile, err = os.CreateTemp("", "rootCA.*.pem")
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgCreatingTempFiles, err)
	}
	p.ClientCertFile, err = os.CreateTemp("", "client.*.pem")
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgCreatingTempFiles, err)
	}
	p.ClientKeyFile, err = os.CreateTemp("", "client.*.key")
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgCreatingTempFiles, err)
	}
	return nil
}

func (p *CertProvider) removeTempFiles() error {
	if p.RootCertFile != nil {
		err := os.Remove(p.RootCertFile.Name())
		if err != nil {
			return fmt.Errorf("%s: %w", errMsgDeletingTempFiles, err)
		}
	}
	if p.ClientCertFile != nil {
		err := os.Remove(p.ClientCertFile.Name())
		if err != nil {
			return fmt.Errorf("%s: %w", errMsgDeletingTempFiles, err)
		}
	}
	if p.ClientKeyFile != nil {
		err := os.Remove(p.ClientKeyFile.Name())
		if err != nil {
			return fmt.Errorf("%s: %w", errMsgDeletingTempFiles, err)
		}
	}
	return nil
}

func (p *CertProvider) CleanUp() error {
	return p.removeTempFiles()
}

func createCertTemplate(isCA bool) (*x509.Certificate, error) {
	sn, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), certSerialNumberUpperLimit))
	if err != nil {
		return nil, fmt.Errorf("serial number generation failed: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	if isCA {
		template.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
	}
	return template, nil
}

func CreateCert(template, parent *x509.Certificate, privateKey *rsa.PrivateKey, rootKey *rsa.PrivateKey) (
	*tls.Certificate, error,
) {
	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &privateKey.PublicKey, rootKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsgCreatingTLSCertificates, err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsgCreatingTLSCertificates, err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsgCreatingTLSCertificates, err)
	}
	return &cert, nil
}

func GenerateRootKey() (*rsa.PrivateKey, error) {
	rootKey, err := rsa.GenerateKey(rand.Reader, privateKeyBits)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsgCreatingPrivateKey, err)
	}
	return rootKey, nil
}

func (p *CertProvider) GenerateCerts() error {
	rootKey, err := GenerateRootKey()
	if err != nil {
		return err
	}
	rootTemplate, err := createCertTemplate(true)
	if err != nil {
		return err
	}
	p.RootCert, err = CreateCert(rootTemplate, rootTemplate, rootKey, rootKey)
	if err != nil {
		return err
	}
	err = writeCertToFile(p.RootCert, p.RootCertFile.Name())
	if err != nil {
		return err
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, privateKeyBits)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgCreatingPrivateKey, err)
	}
	serverTemplate, err := createCertTemplate(false)
	if err != nil {
		return err
	}
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	p.ServerCert, err = CreateCert(serverTemplate, rootTemplate, serverKey, rootKey)
	if err != nil {
		return err
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, privateKeyBits)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgCreatingPrivateKey, err)
	}
	clientTemplate, err := createCertTemplate(false)
	if err != nil {
		return err
	}
	clientTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	clientCert, err := CreateCert(clientTemplate, rootTemplate, clientKey, rootKey)
	if err != nil {
		return err
	}

	err = writeCertToFile(clientCert, p.ClientCertFile.Name())
	if err != nil {
		return err
	}
	return writeKeyToFile(clientKey, p.ClientKeyFile.Name())
}

func writeCertToFile(cert *tls.Certificate, fileName string) error {
	certBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})
	err := os.WriteFile(fileName, certBytes, tempFilePermission)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgWritingTempFiles, err)
	}
	return nil
}

func writeKeyToFile(key *rsa.PrivateKey, fileName string) error {
	keyBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	err := os.WriteFile(fileName, keyBytes, tempFilePermission)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsgWritingTempFiles, err)
	}
	return nil
}
