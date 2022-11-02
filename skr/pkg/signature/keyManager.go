package signature

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getPublicKeyReference fetches the Namespace and the Name of the Secret the Public Key is stored in the KCP.
// Should be called in the Watcher when sending a request to the KCP.
func getPublicKeyReference(ctx context.Context, keysSecret types.NamespacedName, k8sClient client.Client) (types.NamespacedName, error) {
	var pubKeySecret v1.Secret
	err := k8sClient.Get(ctx, keysSecret, &pubKeySecret)
	if err != nil {
		return types.NamespacedName{}, err
	}
	encPubKeyNamespace, ok := pubKeySecret.Data[PubKeyNamespaceKey]
	if !ok {
		return types.NamespacedName{}, fmt.Errorf("secret does not contain key '%s'", PvtKeyKey)
	}
	pubKeyNamespace, err := base64.StdEncoding.DecodeString(string(encPubKeyNamespace))
	if err != nil {
		return types.NamespacedName{}, err
	}
	encPubKeyName, ok := pubKeySecret.Data[PubKeyNameKey]
	if !ok {
		return types.NamespacedName{}, fmt.Errorf("secret does not contain key '%s'", PvtKeyKey)
	}
	pubKeyName, err := base64.StdEncoding.DecodeString(string(encPubKeyName))
	if err != nil {
		return types.NamespacedName{}, err
	}

	pubKeyReference := types.NamespacedName{
		Namespace: string(pubKeyNamespace),
		Name:      string(pubKeyName),
	}
	return pubKeyReference, nil
}

// GetPublicKey fetches the PublicKey using the given publicKeyReference and the given k8sCLient.
// Should be called in the listener to Verify the incoming request
func GetPublicKey(ctx context.Context, publicKeyReference types.NamespacedName, k8sClient client.Client) (crypto.PublicKey, error) {
	var pKeySecret v1.Secret
	err := k8sClient.Get(ctx, publicKeyReference, &pKeySecret)
	if err != nil {
		return nil, err
	}
	pKey := pKeySecret.Data[pubKeyKey] // TODO maybe decryption is needed

	block, _ := pem.Decode(pKey)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the public key")
	}

	var pub any
	pub, err = x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		pub, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DER encoded public key: %w", err)
		}
	}
	return pub, nil
}

// GetPrivateKey fetches the PrivateKey using the given privateKeyReference and the given k8sCLient.
// Should be called in the watcher for signing the request.
func GetPrivateKey(ctx context.Context, privateKeyReference types.NamespacedName, k8sClient client.Client) (*rsa.PrivateKey, error) {
	var pKeySecret v1.Secret
	err := k8sClient.Get(ctx, privateKeyReference, &pKeySecret)
	if err != nil {
		return nil, err
	}
	encodedPrvtKey, ok := pKeySecret.Data[PvtKeyKey] // TODO maybe decryption is needed
	if !ok {
		return nil, fmt.Errorf("secret does not contain key '%s'", PvtKeyKey)
	}
	prvtKey, err := base64.StdEncoding.DecodeString(string(encodedPrvtKey))
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(prvtKey)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the private key")
	}

	prvt, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DER encoded private key: %w", err)
	}
	return prvt, nil
}

// generateRSAKeys generates a private/public RSA Key pair and returns them as encoded PEM blocks(RFC 1421).
func GenerateRSAKeys() (encodedPvtKey, encodedPubKey []byte, err error) {
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
