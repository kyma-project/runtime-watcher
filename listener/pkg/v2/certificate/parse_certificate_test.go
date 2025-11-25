package certificate_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/certificate"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/certificate/utils"
	"github.com/stretchr/testify/require"
)

func TestGetCertificateFromHeader_Success(t *testing.T) {
	pemCert := utils.NewPemCertificateBuilder(t).Build()
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, certificate.CertificateKey+pemCert)
	cert, err := certificate.GetCertificateFromHeader(r)
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func TestGetCertificateFromHeader_MissingHeader(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Equal(t, certificate.ErrHeaderMissing, err)
}

func TestGetCertificateFromHeader_TooLong(t *testing.T) {
	longValue := make([]byte, certificate.Limit32KiB+1)
	for i := range longValue {
		longValue[i] = 'A'
	}
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, certificate.CertificateKey+string(longValue))
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Equal(t, certificate.ErrHeaderValueTooLong, err)
}

func TestGetCertificateFromHeader_EmptyCert(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, "Cert=;Other=foo")
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Equal(t, certificate.ErrEmptyCert, err)
}

func TestGetCertificateFromHeader_NoCertToken(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, "Other=foo;Stuff=bar")
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Equal(t, certificate.ErrEmptyCert, err)
}

func TestGetCertificateFromHeader_InvalidURLFormat(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, "Cert=%ZZ;Other=foo")
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Contains(t, err.Error(), "could not decode certificate URL format")
}

func TestGetCertificateFromHeader_PEMDecodeError(t *testing.T) {
	invalid := url.QueryEscape("not a pem block")
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, certificate.CertificateKey+invalid)
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Equal(t, certificate.ErrPemDecode, err)
}

func TestGetCertificateFromHeader_CertificateParseError(t *testing.T) {
	pemInvalid := "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"
	escaped := url.QueryEscape(pemInvalid)
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header.Set(certificate.XFCCHeader, certificate.CertificateKey+escaped)
	cert, err := certificate.GetCertificateFromHeader(r)
	require.Error(t, err)
	require.Nil(t, cert)
	require.Contains(t, err.Error(), "failed to parse PEM block into x509 certificate")
}

func TestGetCertificateFromHeader_MultipleValuesFirstHasCert(t *testing.T) {
	pemCert := utils.NewPemCertificateBuilder(t).Build()
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	r.Header[certificate.XFCCHeader] = []string{certificate.CertificateKey + pemCert + ";Other=foo", "Cert=ignored"}
	cert, err := certificate.GetCertificateFromHeader(r)
	require.NoError(t, err)
	require.NotNil(t, cert)
}
