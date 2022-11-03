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

func (r *RSAAlgorithm) Sign(rand io.Reader, privateKey *rsa.PrivateKey, sig []byte) ([]byte, error) {
	defer r.Reset()

	if privateKey == nil {
		return nil, errors.New("private key must not be empty")
	}

	if err := r.setSignature(sig); err != nil {
		return nil, err
	}
	return rsa.SignPKCS1v15(rand, privateKey, r.Kind, r.Sum(nil))
}

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

func (r *RSAAlgorithm) setSignature(toHash []byte) error {
	bytesWritten, err := r.Write(toHash)
	if err != nil {
		r.Reset()
		return err
	} else if bytesWritten != len(toHash) {
		r.Reset()
		return fmt.Errorf("only %d of %d bytes could be written to hash", bytesWritten, len(toHash))
	}
	return nil
}
