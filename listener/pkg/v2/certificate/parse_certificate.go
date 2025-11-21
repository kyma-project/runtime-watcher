package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	XFCCHeader     = "X-Forwarded-Client-Cert"
	CertificateKey = "Cert="
	Limit32KiB     = 32 * 1024
)

var (
	ErrPemDecode          = errors.New("failed to decode PEM block")
	ErrEmptyCert          = errors.New("empty certificate")
	ErrHeaderValueTooLong = errors.New(XFCCHeader + " header value too long (over 32KiB)")
	ErrHeaderMissing      = fmt.Errorf("request does not contain '%s' header", XFCCHeader)
)

// GetCertificateFromHeader extracts the XFCC header and pareses it into a valid x509 certificate.
func GetCertificateFromHeader(r *http.Request) (*x509.Certificate, error) {
	// Fetch XFCC-Header data
	xfccValues, ok := r.Header[XFCCHeader]
	if !ok {
		return nil, ErrHeaderMissing
	}

	xfccVal := xfccValues[0]

	// Limit the length of the data (prevent resource exhaustion attack)
	if len(xfccVal) > Limit32KiB {
		return nil, ErrHeaderValueTooLong
	}

	// Extract raw certificate from the first header value
	cert := getCertTokenFromXFCCHeader(xfccVal)
	if cert == "" {
		return nil, ErrEmptyCert
	}

	// Decode URL-format
	decodedValue, err := url.QueryUnescape(cert)
	if err != nil {
		return nil, fmt.Errorf("could not decode certificate URL format: %w", err)
	}
	decodedValue = strings.Trim(decodedValue, "\"")

	// Decode PEM block and parse certificate
	block, _ := pem.Decode([]byte(decodedValue))
	if block == nil {
		return nil, ErrPemDecode
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PEM block into x509 certificate: %w", err)
	}

	return certificate, nil
}

// getCertTokenFromXFCCHeader returns the first certificate embedded in the XFFC Header,
// if exists. Otherwise an empty string is returned.
func getCertTokenFromXFCCHeader(hVal string) string {
	certStartIdx := strings.Index(hVal, CertificateKey)
	if certStartIdx >= 0 {
		tokenWithCert := hVal[(certStartIdx + len(CertificateKey)):]
		// we shouldn't have "," here but it's safer to add it anyway
		certEndIdx := strings.IndexAny(tokenWithCert, ";,")
		if certEndIdx == -1 {
			// no suffix, the entire token is the cert value
			return tokenWithCert
		}

		// there's some data after the cert value, return just the cert part
		return tokenWithCert[:certEndIdx]
	}
	return ""
}
