package signature

import (
	"crypto"
	"crypto/rsa"
	"errors"
	"fmt"
	"hash"
	"io"
)

type RSAAlgorithm struct {
	hash.Hash
	Kind crypto.Hash
}

// Sign signs the receiver with the given private key and signature string.
func (r *RSAAlgorithm) Sign(rand io.Reader, privateKey *rsa.PrivateKey, signatureString []byte) ([]byte, error) {
	defer r.Reset()

	if privateKey == nil {
		return nil, errors.New("private key must not be empty")
	}

	if err := r.setSignature(signatureString); err != nil {
		return nil, err
	}
	return rsa.SignPKCS1v15(rand, privateKey, r.Kind, r.Sum(nil))
}

// Verify verifies the receiver with the given public key, the signature string and the signature from the request.
func (r *RSAAlgorithm) Verify(publicKey crypto.PublicKey, toHash, signature []byte) error {
	defer r.Reset()

	if publicKey == nil {
		return errors.New("public key must not be empty")
	}

	rsaPubKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("public key cannot be converted to RSA Public Key")
	}

	if err := r.setSignature(toHash); err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(rsaPubKey, r.Kind, r.Sum(nil), signature)
}

func (r *RSAAlgorithm) setSignature(signatureString []byte) error {
	bytesWritten, err := r.Write(signatureString)
	if err != nil {
		r.Reset()
		return err
	} else if bytesWritten != len(signatureString) {
		r.Reset()
		return fmt.Errorf("only %d of %d bytes could be written to hash", bytesWritten, len(signatureString))
	}
	return nil
}
