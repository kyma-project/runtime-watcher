package signature

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SignRequest signs the request using the RSA-SHA-256 algorithm.
// The function uses the private key secret to determine which public key to use when verifying a signed request.
// Furthermore, a Digest will be attached to the request (RFC 3230). The given body may be nil
// but can still be verified.
// The Digest verifies that the request body is not changed while it is being transmitted,
// and the HTTP Signature verifies that neither the Digest nor the body have been
// fraudulently altered to falsely represent different information.
func SignRequest(request *http.Request, keySecretReference types.NamespacedName, k8sClient client.Client) error {
	rsa := &RSAAlgorithm{
		Hash: sha256.New(),
		Kind: crypto.SHA256,
	}

	// Get Private Key
	prvtKey, err := getPrivateKey(request.Context(), keySecretReference, k8sClient)
	if err != nil {
		return err
	}

	// Get Public Key Reference
	pubKeyReference, err := getPublicKeyReference(request.Context(), keySecretReference, k8sClient)
	if err != nil {
		return nil
	}

	// Add Digest
	if err := AddDigest(request); err != nil {
		return err
	}

	// Create Signature String
	created := time.Now().Unix()
	sigString, err := signatureString(created)
	if err != nil {
		return err
	}

	// Create Signature
	sig, err := rsa.Sign(rand.Reader, prvtKey, []byte(sigString))
	if err != nil {
		return err
	}
	encSig := base64.StdEncoding.EncodeToString(sig)

	// Sign request with Signature
	setSignatureHeader(request.Header, pubKeyReference, SignatureHeader, encSig, created)
	return nil
}

func signatureString(created int64) (string, error) {
	var buffer bytes.Buffer
	for n, header := range defaultHeaders { //nolint:varnamelen
		header = strings.ToLower(header)

		if header == createdParameter {
			if created == 0 {
				return "", fmt.Errorf("empty created value")
			}
			buffer.WriteString(header)
			buffer.WriteString(headerFieldDelimiter)
			buffer.WriteString(strconv.FormatInt(created, base))
		} // TODO else if: add more headers if needed
		if n < len(defaultHeaders)-1 {
			buffer.WriteString(headersDelimiter)
		}
	}
	return buffer.String(), nil
}

func setSignatureHeader(header http.Header, pubKeySecretReference types.NamespacedName,
	targetHeader, enc string, created int64,
) {
	var buffer bytes.Buffer
	// Public Key Secret Name
	buffer.WriteString(pubKeySecretNameParameter)
	buffer.WriteString(parameterKVSeparater)
	buffer.WriteString(parameterValueDelimiter)
	buffer.WriteString(pubKeySecretReference.Name)
	buffer.WriteString(parameterValueDelimiter)
	buffer.WriteString(parameterSeparater)
	// Public Key Secret Namespace
	buffer.WriteString(pubKeySecretNamespaceParameter)
	buffer.WriteString(parameterKVSeparater)
	buffer.WriteString(parameterValueDelimiter)
	buffer.WriteString(pubKeySecretReference.Namespace)
	buffer.WriteString(parameterValueDelimiter)
	buffer.WriteString(parameterSeparater)
	// Created
	buffer.WriteString(createdParameter)
	buffer.WriteString(parameterKVSeparater)
	buffer.WriteString(strconv.FormatInt(created, base))
	buffer.WriteString(parameterSeparater)
	// Signature
	buffer.WriteString(signatureParameter)
	buffer.WriteString(parameterKVSeparater)
	buffer.WriteString(parameterValueDelimiter)
	buffer.WriteString(enc)
	buffer.WriteString(parameterValueDelimiter)
	header.Add(targetHeader, buffer.String())
}
