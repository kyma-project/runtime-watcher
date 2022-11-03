package signature

import (
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Maximum clock offset between sender and receiver.
	signatureClockOffset = 5

	base    = 10
	bitsize = 64
)

// Implement test

func VerifyRequest(request *http.Request, k8sClient client.Client) (bool, error) {
	header := request.Header

	// Verify Digest
	if err := VerifyDigest(request); err != nil {
		return false, err
	}

	// Get parameters from ReceivedSignature
	signature, err := getSignature(header)
	if err != nil {
		return false, err
	}
	publicKeySecret, sig, created, err := getSignatureParameters(signature)
	if err != nil {
		return false, err
	}

	// Fetch publicKey using the Secret Reference from the Parameters
	publicKey, err := GetPublicKey(request.Context(), publicKeySecret, k8sClient)
	if err != nil {
		return false, err
	}

	// Create verifier for verifying the ReceivedSignature
	v := &verifier{ //nolint:varnamelen
		header:          header,
		publicKeySecret: publicKey,
		signature:       sig,
		created:         created,
	}
	// Create new signer to Verify
	signer := &RSAAlgorithm{
		Hash: sha256.New(),
		Kind: crypto.SHA256,
	}
	if err := v.verify(signer, publicKey); err != nil {
		return false, errors.New("incomming request could not be verified")
	}
	return true, nil
}

func getSignature(h http.Header) (string, error) {
	signature := h.Get(SignatureHeader)
	neededParameters := []string{
		pubKeySecretNameParameter,
		pubKeySecretNamespaceParameter,
		signatureParameter, createdParameter,
	}
	valid := true
	for _, p := range neededParameters {
		if !strings.Contains(signature, p) {
			valid = false
			break
		}
	}
	if valid {
		return signature, nil
	}
	return "", fmt.Errorf("ReceivedSignature header `%s` does not contain needed parameters (%v): %s",
		SignatureHeader, neededParameters, signature)
}

func getSignatureParameters(s string) (types.NamespacedName, string, int64, error) { //nolint:cyclop
	params := strings.Split(s, parameterSeparater)
	var secretName, secretNamespace, signature string
	var secretReference types.NamespacedName
	var created int64

	for _, p := range params {
		keyValue := strings.SplitN(p, parameterKVSeparater, 2) //nolint:gomnd
		if len(keyValue) != 2 {                                //nolint:gomnd
			err := fmt.Errorf("ReceivedSignature parameter has wrong format (`<key>=<value>`): %v", p)
			return types.NamespacedName{}, "", 0, err
		}
		key := keyValue[0]
		value := strings.Trim(keyValue[1], parameterValueDelimiter)
		var err error
		switch key {
		case createdParameter:
			created, err = strconv.ParseInt(value, base, bitsize)
			if err != nil {
				return types.NamespacedName{}, "", 0, err
			}
		case pubKeySecretNameParameter:
			secretName = value
		case pubKeySecretNamespaceParameter:
			secretNamespace = value
		case signatureParameter:
			signature = value
		default:
			// Ignore other parameters
		}
	}
	secretReference = types.NamespacedName{
		Namespace: secretNamespace,
		Name:      secretName,
	}

	errFmt := "%w\nmissing %q parameter in http ReceivedSignature"
	now := time.Now().Unix()
	var err error
	if created-now > signatureClockOffset {
		// maximum offset of 5 seconds
		err = fmt.Errorf("`created`-parameter is in the future")
	}
	if len(secretReference.Name) == 0 {
		err = fmt.Errorf(errFmt, err, pubKeySecretNameParameter)
	}
	if len(secretReference.Namespace) == 0 {
		err = fmt.Errorf(errFmt, err, pubKeySecretNamespaceParameter)
	}
	if len(signature) == 0 {
		err = fmt.Errorf(errFmt, err, signatureParameter)
	}
	if err != nil {
		return types.NamespacedName{}, "", 0, err
	}
	return secretReference, signature, created, err
}

type verifier struct {
	header          http.Header
	publicKeySecret crypto.PublicKey
	signature       string
	created         int64
}

func (v *verifier) verify(signer *RSAAlgorithm, publicKey crypto.PublicKey) error {
	toHash, err := signatureString(v.created)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(v.signature)
	if err != nil {
		return err
	}
	err = signer.Verify(publicKey, []byte(toHash), signature)
	if err != nil {
		return err
	}
	return nil
}
