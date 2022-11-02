package sign

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	//Authentication scheme hte HTTP Signatures specification uses for the Authorization header.
	signatureAuthScheme = "Signature"

	// Headers
	digestHeader    = "Digest"
	SignatureHeader = "Signature"

	// Signature String Construction
	headerFieldDelimiter = ": "
	headersDelimiter     = "\n"

	// Signature Parameters
	createdParameter               = "created"
	pubKeySecretNameParameter      = "pubKeySecretName"
	pubKeySecretNamespaceParameter = "pubKeySecretNamespace"
	signatureParameter             = "signature"
	prefixSeparater                = " "
	parameterKVSeparater           = "="
	parameterValueDelimiter        = "\""
	parameterSeparater             = ","
)

var (
	// TODO include expires Header
	defaultHeaders = []string{createdParameter}
)

// SignRequest signs the request using the RSA-SHA-256 algorithm.
// The HTTP server uses the public key secret to determine which public key to use when verifying a signed request.
// Furthermore, a Digest will be attached to the request (RFC 3230). The given body may be nil
// but must match the body specified in the request.
// The Digest verifies that the request body is not changed while it is being transmitted,
// and the HTTP Signature verifies that neither the Digest nor the body have been
// fraudulently altered to falsely represent different information.
func SignRequest(pKey string, pubKeySecret types.NamespacedName, r *http.Request) error {

	rsa := &RSAAlgorithm{
		Hash: sha256.New(),
		Kind: crypto.SHA256,
	}

	// Add Digest
	AddDigest(r)

	// Create Signature String
	created := time.Now().Unix()
	sigString, err := signatureString(created)

	// Create Signature
	sig, err := rsa.Sign(rand.Reader, pKey, []byte(sigString))
	if err != nil {
		return err
	}
	encSig := base64.StdEncoding.EncodeToString(sig)

	// Sign request with Signature
	setSignatureHeader(r.Header, SignatureHeader, pubKeySecret.Name, pubKeySecret.Namespace, encSig, created)
	return nil
}

func signatureString(created int64) (string, error) {
	var b bytes.Buffer
	for n, i := range defaultHeaders {
		i := strings.ToLower(i)

		if i == createdParameter {
			if created == 0 {
				return "", fmt.Errorf("empty created value")
			}
			b.WriteString(i)
			b.WriteString(headerFieldDelimiter)
			b.WriteString(strconv.FormatInt(created, 10))
		} // TODO else if: add more headers if needed
		if n < len(defaultHeaders)-1 {
			b.WriteString(headersDelimiter)
		}
	}
	return b.String(), nil
}

func setSignatureHeader(h http.Header, targetHeader, pubKeySecretName, pubKeySecretNamespace, enc string, created int64) {
	var b bytes.Buffer
	// Public Key Secret Name
	b.WriteString(signatureAuthScheme)
	b.WriteString(prefixSeparater)
	b.WriteString(pubKeySecretNameParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeySecretName)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Public Key Secret Namespace
	b.WriteString(pubKeySecretNamespaceParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeySecretNamespace)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Created
	b.WriteString(createdParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(strconv.FormatInt(created, 10))
	b.WriteString(parameterSeparater)
	// Signature
	b.WriteString(signatureParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(enc)
	b.WriteString(parameterValueDelimiter)
	h.Add(targetHeader, b.String())
}
