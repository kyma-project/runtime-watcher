package signature

const (
	pubKeyKey          = "publicKey"
	PvtKeyKey          = "privateKey"
	PubKeyNamespaceKey = "publicKeyNamespace"
	PubKeyNameKey      = "publicKeyName"

	keyBitSize = 2048

	// Headers
	digestHeader    = "Digest"
	SignatureHeader = "Signature"

	// Signature String Construction
	headerFieldDelimiter = ": "
	headersDelimiter     = "\n"

	// Signature Parameters
	createdParameter               = "created"
	pubKeySecretNameParameter      = "pubKeySecretName"
	pubKeySecretNamespaceParameter = "pubKeySecretNamespace"
	signatureParameter             = "Signature"
	parameterKVSeparater           = "="
	parameterValueDelimiter        = "\""
	parameterSeparater             = ","
)
