package signature

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"
)

var (
	// TODO include 'expires Header
	defaultHeaders = []string{createdParameter}
)

// SignRequest signs the request using the RSA-SHA-256 algorithm.
// The HTTP server uses the public key secret to determine which public key to use when verifying a signed request.
// Furthermore, a Digest will be attached to the request (RFC 3230). The given body may be nil
// but must match the body specified in the request.
// The Digest verifies that the request body is not changed while it is being transmitted,
// and the HTTP Signature verifies that neither the Digest nor the body have been
// fraudulently altered to falsely represent different information.
func SignRequest(r *http.Request, keySecretReference types.NamespacedName, k8sClient client.Client) error {

	rsa := &RSAAlgorithm{
		Hash: sha256.New(),
		Kind: crypto.SHA256,
	}

	// Get Private Key
	prvtKey, err := GetPrivateKey(r.Context(), keySecretReference, k8sClient)
	if err != nil {
		return err
	}

	// Get Public Key Reference
	pubKeyReference, err := getPublicKeyReference(r.Context(), keySecretReference, k8sClient)
	if err != nil {
		return nil
	}

	// Add Digest
	AddDigest(r)

	// Create Signature String
	created := time.Now().Unix()
	sigString, err := signatureString(created)

	// Create Signature
	sig, err := rsa.Sign(rand.Reader, prvtKey, []byte(sigString))
	if err != nil {
		return err
	}
	encSig := base64.StdEncoding.EncodeToString(sig)

	// Sign request with Signature
	setSignatureHeader(r.Header, pubKeyReference, SignatureHeader, encSig, created)
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

func setSignatureHeader(h http.Header, pubKeySecretReference types.NamespacedName, targetHeader, enc string, created int64) {
	var b bytes.Buffer
	// Public Key Secret Name
	b.WriteString(pubKeySecretNameParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeySecretReference.Name)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Public Key Secret Namespace
	b.WriteString(pubKeySecretNamespaceParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeySecretReference.Namespace)
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
