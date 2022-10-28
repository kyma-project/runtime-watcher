package sign

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	pubKeyKey          = "publicKey"
	pvtKeyKey          = "privateKey"
	pubKeyNamespaceKey = "publicKeyNamespace"
	pubKeyNameKey      = "publicKeyName"

	keyBitSize = 2048
)

type rsaAlgorithm struct {
	hash.Hash
	kind crypto.Hash
}

func (r *rsaAlgorithm) Sign(rand io.Reader, privateKey crypto.PrivateKey, sig []byte) ([]byte, error) {
	defer r.Reset()

	if err := r.setSignature(sig); err != nil {
		return nil, err
	}
	rsaK, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("given PrivatKey cannot be converted to *rsa.PrivateKey")
	}
	return rsa.SignPKCS1v15(rand, rsaK, r.kind, r.Sum(nil))
}

func (r *rsaAlgorithm) Verify(pub crypto.PublicKey, toHash, signature []byte) error {
	defer r.Reset()
	rsaK, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("given PublicKey cannot be converted to *rsa.PublicKey")
	}
	if err := r.setSignature(toHash); err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(rsaK, r.kind, r.Sum(nil), signature)
}

func (r *rsaAlgorithm) setSignature(b []byte) error {
	n, err := r.Write(b)
	if err != nil {
		r.Reset()
		return err
	} else if n != len(b) {
		r.Reset()
		return fmt.Errorf("only %d of %d bytes could be written to hash", n, len(b))
	}
	return nil
}

// getPublicKeyReference fetches the Namespace and the Name of the Secret the Public Key is stored in the KCP.
// Should be called in the Watcher when sending a request to the KCP.
func getPublicKeyReference(ctx context.Context, keysSecret types.NamespacedName, k8sClient client.Client) (types.NamespacedName, error) {
	var pubKeySecret v1.Secret
	err := k8sClient.Get(ctx, keysSecret, &pubKeySecret)
	if err != nil {
		return types.NamespacedName{}, err
	}
	pubKeyNamespace := pubKeySecret.Data[pubKeyNamespaceKey] // TODO maybe decryption is needed
	pubKeyName := pubKeySecret.Data[pubKeyNameKey]           // TODO maybe decryption is needed

	pubKeyReference := types.NamespacedName{
		Namespace: string(pubKeyNamespace),
		Name:      string(pubKeyName),
	}
	return pubKeyReference, nil
}

// GetPublicKey fetches the PublicKey using the given publicKeyReference and the given k8sCLient.
// Should be called in the listener to Verify the incoming request
func GetPublicKey(ctx context.Context, publicKeyReference types.NamespacedName, k8sClient client.Client) (crypto.PublicKey, error) {
	return getKey(ctx, publicKeyReference, pubKeyKey, k8sClient)
}

// GetPrivateKey fetches the PrivayeKey using the given privateKeyReference and the given k8sCLient.
// Should be called in the watcher for signing the request.
func GetPrivateKey(ctx context.Context, privateKeyReference types.NamespacedName, k8sClient client.Client) (crypto.PublicKey, error) {
	return getKey(ctx, privateKeyReference, pvtKeyKey, k8sClient)
}

func getKey(ctx context.Context, keySecretReference types.NamespacedName, key string, k8sClient client.Client) (crypto.PublicKey, error) {
	var pKeySecret v1.Secret
	err := k8sClient.Get(ctx, keySecretReference, &pKeySecret)
	if err != nil {
		return nil, err
	}
	pKey := pKeySecret.Data[key] // TODO maybe decryption is needed

	block, _ := pem.Decode(pKey)
	if block == nil {
		panic("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("failed to parse DER encoded public key: " + err.Error())
	}

	switch pub := pub.(type) {
	case *rsa.PrivateKey:
		return pub, nil
	case *rsa.PublicKey:
		return pub, nil
	default:
		return nil, errors.New("unknown type of public key")
	}
}

// generateRSAKeys generates a private/public RSA Key pair and returns them as encoded PEM blocks(RFC 1421).
func generateRSAKeys() (encodedPvtKey, encodedPubKey []byte, err error) {
	// generate key
	privatekey, err := rsa.GenerateKey(rand.Reader, keyBitSize)
	if err != nil {
		fmt.Printf("Cannot generate RSA keyn")
		os.Exit(1)
	}

	// encode private key to []byte
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	encodedPvtKey = pem.EncodeToMemory(privateKeyBlock)

	// encode public key to []byte
	publickey := &privatekey.PublicKey
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publickey)
	if err != nil {
		fmt.Printf("error when converting public key: %s n", err)
		return
	}
	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	encodedPubKey = pem.EncodeToMemory(publicKeyBlock)
	return
}
