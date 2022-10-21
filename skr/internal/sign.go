package internal

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTP Signatures can be applied to different HTTP headers, depending on the
// expected application behavior.
type SignatureScheme string

const (
	// Signature will place the HTTP Signature into the 'Signature' HTTP
	// header.
	Signature SignatureScheme = "Signature"
	// Authorization will place the HTTP Signature into the 'Authorization'
	// HTTP header.
	Authorization SignatureScheme = "Authorization"
)

const (
	algorithm = "SHA-256"

	digestHeader = "Digest"
	digestDelim  = "="

	createdHeader = "created"
	dateHeader    = "date"

	// Signature String Construction
	headerFieldDelimiter = ": "
	headersDelimiter     = "\n"

	// Signature Parameters
	keyIdParameter            = "keyId"
	algorithmParameter        = "algorithm"
	headersParameter          = "headers"
	signatureParameter        = "signature"
	prefixSeparater           = " "
	parameterKVSeparater      = "="
	parameterValueDelimiter   = "\""
	parameterSeparater        = ","
	headerParameterValueDelim = " "
)

const (
	// The HTTP Signatures specification uses the "Signature" auth-scheme
	// for the Authorization header. This is coincidentally named, but not
	// semantically the same, as the "Signature" HTTP header value.
	signatureAuthScheme = "Signature"
)

var (
	defaultHeaders = []string{dateHeader, createdHeader}
)

func sign(pKey crypto.PrivateKey, pubKeyId string, r *http.Request, body []byte) error {

	// Add Digest
	var h = crypto.SHA256.New()
	h.Write(body)
	sum := h.Sum(nil)
	r.Header.Add(digestHeader,
		fmt.Sprintf("%s%s%s",
			algorithm,
			digestDelim,
			base64.StdEncoding.EncodeToString(sum[:])))

	// Create Signature String
	created := time.Now().Unix()
	sigString, err := signatureString(created)

	// Sign
	pKeyBytes, ok := pKey.([]byte)
	if !ok {
		return fmt.Errorf("private key for MAC signing must be of type []byte")
	}
	sig, err := SignHMAC([]byte(sigString), pKeyBytes)
	if err != nil {
		return err
	}
	encSig := base64.StdEncoding.EncodeToString(sig)

	setSignatureHeader(r.Header, Signature, pubKeyId, algorithm, encSig, created)
	return nil
}

func signatureString(created int64) (string, error) {
	var b bytes.Buffer
	for n, i := range defaultHeaders {
		i := strings.ToLower(i)

		if i == createdHeader {
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

func SignHMAC(sig, key []byte) ([]byte, error) {
	hs := hmac.New(
		func() hash.Hash {
			h := sha256.New()
			return h
		}, key)
	if err := setSignature(hs, sig); err != nil {
		return nil, err
	}
	return hs.Sum(nil), nil
}

func setSignature(h hash.Hash, b []byte) error {
	n, err := h.Write(b)
	if err != nil {
		h.Reset()
		return err
	} else if n != len(b) {
		h.Reset()
		return fmt.Errorf("only %d of %d bytes could be written to hash", n, len(b))
	}
	return nil
}

func setSignatureHeader(h http.Header, targetHeader SignatureScheme, pubKeyId, algo, enc string, created int64) {
	var b bytes.Buffer
	// KeyId
	b.WriteString(signatureAuthScheme)
	b.WriteString(prefixSeparater)
	b.WriteString(keyIdParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeyId)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Algorithm
	b.WriteString(algorithmParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(algo) //real algorithm is hidden, see newest version of spec draft
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Created
	b.WriteString(createdHeader)
	b.WriteString(parameterKVSeparater)
	b.WriteString(strconv.FormatInt(created, 10))
	b.WriteString(parameterSeparater)
	// Signature
	b.WriteString(signatureParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(enc)
	b.WriteString(parameterValueDelimiter)
	h.Add(string(targetHeader), b.String())
}
