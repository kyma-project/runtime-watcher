package signature

const (
	// PubKeyKey key used in KCP secret, whichs value is the Public Key.
	PubKeyKey = "publicKey"
	// PrvtKeyKey key used in SKR secret, whichs value is the Private Key.
	PrvtKeyKey = "privateKey"
	// PubKeyNamespaceKey key used in SKR secret, which value is the namespace of Public Key Secret in KCP.
	PubKeyNamespaceKey = "publicKeyNamespace"
	// PubKeyNameKey key used in SKR secret, which value is the name of Public Key Secret in KCP.
	PubKeyNameKey = "publicKeyName"

	// keyBitSize is the bit size of the generated RSA key pair.
	keyBitSize = 2048

	// DigestHeader header which stores the digest.
	DigestHeader = "Digest"
	// SignatureHeader header which stores the signature.
	SignatureHeader = "Signature"

	// Signature String Construction.
	headerFieldDelimiter = ": "
	headersDelimiter     = "\n"

	// Signature Parameters.
	createdParameter               = "created"
	pubKeySecretNameParameter      = "pubKeySecretName"
	pubKeySecretNamespaceParameter = "pubKeySecretNamespace"
	signatureParameter             = "Signature"
	parameterKVSeparater           = "="
	parameterValueDelimiter        = "\""
	parameterSeparater             = ","
)

// TODO include expires header.
var defaultHeaders = []string{createdParameter} //nolint:gochecknoglobals
