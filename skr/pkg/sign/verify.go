package sign

import (
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"
)

const (
	hostHeader = "Host"

	// Maximum clock offset between sender and receiver
	signatureClockOffset = 5
)

// TODO next Step: Check digest in verify
// Implement test

func VerifyRequest(r *http.Request, k8sClient client.Client) (bool, error) {
	// TODO: Maybe remove host treatment, since maybe not really needed here
	h := r.Header
	if _, hasHostHeader := h[hostHeader]; len(r.Host) > 0 && !hasHostHeader {
		h[hostHeader] = []string{r.Host}
	}

	// Get parameters from signature
	signature, err := getSignature(h)
	if err != nil {
		return false, err
	}
	publicKeySecret, sig, created, err := getSignatureParameters(signature)
	if err != nil {
		return false, err
	}

	// Fetch publicKey using the Secret Reference from the Parameters
	publicKey, err := GetPublicKey(r.Context(), publicKeySecret, k8sClient)
	if err != nil {
		return false, err
	}

	// Create verifier for verifying the signature
	v := &verifier{
		header:          h,
		publicKeySecret: publicKey,
		signature:       sig,
		created:         created,
	}
	// Create new signer to Verify
	signer := &rsaAlgorithm{
		Hash: sha256.New(),
		kind: crypto.SHA256,
	}
	if err := v.verify(signer, publicKey); err != nil {
		return false, errors.New("incomming request could not be verified")
	}
	return true, nil
}

func getSignature(h http.Header) (string, error) {
	s := h.Get(SignatureHeader)
	neededParameters := []string{pubKeySecretNameParameter, pubKeySecretNamespaceParameter, signatureParameter, createdParameter}
	ok := true
	for _, p := range neededParameters {
		if !strings.Contains(s, p) {
			ok = false
			break
		}
	}
	if ok {
		return s, nil
	} else {
		return "", fmt.Errorf("signature header `%s` does not contain needed parameters (%v): %s", SignatureHeader, neededParameters, s)
	}
}
func getSignatureParameters(s string) (secretReference types.NamespacedName, sig string, created int64, err error) {
	params := strings.Split(s, parameterSeparater)
	var secretName string
	var secretNamespace string
	for _, p := range params {
		kv := strings.SplitN(p, parameterKVSeparater, 2)
		if len(kv) != 2 {
			err = fmt.Errorf("signature parameter has wrong format (`<key>=<value>`): %v", p)
			return
		}
		key := kv[0]
		value := strings.Trim(kv[1], parameterValueDelimiter)
		switch key {
		// TODO: Add expired
		case createdParameter:
			created, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return
			}
		case pubKeySecretNameParameter:
			secretName = value
		case pubKeySecretNamespaceParameter:
			secretNamespace = value
		case signatureParameter:
			sig = value
		default:
			// Ignore other parameters
		}
	}
	secretReference = types.NamespacedName{
		Namespace: secretNamespace,
		Name:      secretName,
	}

	errFmt := "missing %q parameter in http signature"
	now := time.Now().Unix()
	if created-now > signatureClockOffset {
		// maximum offset of 5 seconds
		err = fmt.Errorf("`created`-parameter is in the future")
	} else if len(secretReference.Name) == 0 {
		err = fmt.Errorf(errFmt, pubKeySecretNameParameter)
	} else if len(secretReference.Namespace) == 0 {
		err = fmt.Errorf(errFmt, pubKeySecretNamespaceParameter)
	} else if len(sig) == 0 {
		err = fmt.Errorf(errFmt, signatureParameter)
	}
	return
}

type verifier struct {
	header          http.Header
	publicKeySecret crypto.PublicKey
	signature       string
	created         int64
}

func (v *verifier) verify(signer *rsaAlgorithm, publicKey crypto.PublicKey) error {

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
