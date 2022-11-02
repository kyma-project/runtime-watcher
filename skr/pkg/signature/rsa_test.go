package signature_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/kyma-project/runtime-watcher/skr/pkg/signature"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSign(t *testing.T) {

	tests := []struct {
		testName          string
		prvtKey           *rsa.PrivateKey
		sigString         []byte
		expectedSignature []byte
		expectError       bool
	}{
		{
			testName:  "Add sha256 digest",
			prvtKey:   parsePrivateKey(t, RSAPrvtKey),
			sigString: []byte(fmt.Sprintf("created=%v", 00000000000)),
			expectedSignature: []byte{39, 198, 186, 119, 234, 206, 24, 15, 131, 168, 150,
				33, 75, 248, 120, 148, 205, 120, 210, 255, 153, 240, 84, 8, 167, 45, 50,
				210, 223, 189, 26, 191, 224, 152, 0, 96, 148, 35, 24, 148, 82, 145, 196,
				226, 181, 224, 5, 77, 85, 96, 96, 175, 164, 237, 77, 53, 122, 33, 169, 40,
				121, 199, 137, 47, 47, 63, 68, 7, 69, 90, 152, 226, 254, 232, 147, 107, 154,
				55, 165, 70, 226, 227, 30, 56, 73, 149, 112, 8, 131, 125, 30, 37, 218, 134,
				227, 75, 64, 167, 119, 113, 35, 38, 117, 208, 141, 184, 170, 117, 49, 216,
				21, 100, 154, 78, 41, 53, 20, 43, 114, 138, 25, 74, 233, 56, 249, 21, 66, 74,
				105, 27, 142, 179, 62, 174, 124, 5, 202, 0, 192, 194, 204, 242, 150, 37, 158,
				29, 64, 104, 226, 147, 169, 38, 182, 170, 180, 131, 68, 136, 94, 48, 53, 110,
				6, 222, 119, 58, 181, 94, 224, 22, 95, 39, 3, 120, 64, 226, 139, 9, 26, 252,
				102, 72, 117, 176, 250, 28, 31, 252, 162, 245, 172, 0, 143, 162, 206, 8, 255,
				140, 251, 158, 234, 82, 88, 103, 90, 172, 141, 93, 41, 159, 30, 99, 121, 46,
				58, 245, 144, 78, 208, 79, 245, 210, 249, 123, 138, 47, 252, 163, 135, 90,
				62, 26, 96, 117, 195, 45, 144, 115, 223, 187, 147, 1, 236, 187, 36, 152, 58,
				204, 193, 74, 255, 142, 34, 151, 58, 214},
			expectError: false,
		},
		{
			testName:    "Private Key is empty",
			expectError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			test := test

			rsa := &signature.RSAAlgorithm{
				Hash: sha256.New(),
				Kind: crypto.SHA256,
			}
			signature, err := rsa.Sign(rand.Reader, test.prvtKey, test.sigString)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, test.expectedSignature, signature)

		})
	}
}

func TestVerify(t *testing.T) {
	tests := []struct {
		testName          string
		pubKey            crypto.PublicKey
		sigString         []byte
		ReceivedSignature []byte
		expectError       bool
	}{
		{
			testName:  "Verify received signature",
			pubKey:    parsePublicKey(t, RSAPubKey),
			sigString: []byte(fmt.Sprintf("created=%v", 00000000000)),
			ReceivedSignature: []byte{39, 198, 186, 119, 234, 206, 24, 15, 131, 168, 150,
				33, 75, 248, 120, 148, 205, 120, 210, 255, 153, 240, 84, 8, 167, 45, 50,
				210, 223, 189, 26, 191, 224, 152, 0, 96, 148, 35, 24, 148, 82, 145, 196,
				226, 181, 224, 5, 77, 85, 96, 96, 175, 164, 237, 77, 53, 122, 33, 169, 40,
				121, 199, 137, 47, 47, 63, 68, 7, 69, 90, 152, 226, 254, 232, 147, 107, 154,
				55, 165, 70, 226, 227, 30, 56, 73, 149, 112, 8, 131, 125, 30, 37, 218, 134,
				227, 75, 64, 167, 119, 113, 35, 38, 117, 208, 141, 184, 170, 117, 49, 216,
				21, 100, 154, 78, 41, 53, 20, 43, 114, 138, 25, 74, 233, 56, 249, 21, 66, 74,
				105, 27, 142, 179, 62, 174, 124, 5, 202, 0, 192, 194, 204, 242, 150, 37, 158,
				29, 64, 104, 226, 147, 169, 38, 182, 170, 180, 131, 68, 136, 94, 48, 53, 110,
				6, 222, 119, 58, 181, 94, 224, 22, 95, 39, 3, 120, 64, 226, 139, 9, 26, 252,
				102, 72, 117, 176, 250, 28, 31, 252, 162, 245, 172, 0, 143, 162, 206, 8, 255,
				140, 251, 158, 234, 82, 88, 103, 90, 172, 141, 93, 41, 159, 30, 99, 121, 46,
				58, 245, 144, 78, 208, 79, 245, 210, 249, 123, 138, 47, 252, 163, 135, 90,
				62, 26, 96, 117, 195, 45, 144, 115, 223, 187, 147, 1, 236, 187, 36, 152, 58,
				204, 193, 74, 255, 142, 34, 151, 58, 214},
			expectError: false,
		},
		{
			testName:    "Public Key is empty",
			expectError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			test := test

			rsa := &signature.RSAAlgorithm{
				Hash: sha256.New(),
				Kind: crypto.SHA256,
			}

			err := rsa.Verify(test.pubKey, test.sigString, test.ReceivedSignature)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

		})
	}
}

var (
	RSAPubKey = "-----BEGIN PUBLIC KEY-----\n" +
		"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAiaQuPD8SJZ+kw5NF0Mlf\n" +
		"3FvKfeXxAiXUFnURp/+CgNPOy+2vzeDthLP5zpZrk8OV5GpX7///E448ATSo5lVo\n" +
		"Zz2pdvpHPAMGtf06gdX3B79FCzuidxXz0kGdr0bX1Ozy3OKWCmudPsR9DtOR4Me+\n" +
		"Hg2+neLKgBf7rNWUPc/9CwYkbIt6BmMjhHk1Ijs4e8YUF809WYdJlN8c0ErKJ1b5\n" +
		"vCqs2Ja/xfREveNU6Psadbf2iOKNVr8ipa5YFCfHM7OoFARYaIWB4q6cRavf5fjf\n" +
		"hPTV7kc6twX5liLRsM4kWe2MaTQ0snGCauWA03+wi+FJXYcUAqZSTLj0pJ2UA71S\n" +
		"twIDAQAB\n" +
		"-----END PUBLIC KEY-----"
	RSAPrvtKey = "-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEowIBAAKCAQEAiaQuPD8SJZ+kw5NF0Mlf3FvKfeXxAiXUFnURp/+CgNPOy+2v\n" +
		"zeDthLP5zpZrk8OV5GpX7///E448ATSo5lVoZz2pdvpHPAMGtf06gdX3B79FCzui\n" +
		"dxXz0kGdr0bX1Ozy3OKWCmudPsR9DtOR4Me+Hg2+neLKgBf7rNWUPc/9CwYkbIt6\n" +
		"BmMjhHk1Ijs4e8YUF809WYdJlN8c0ErKJ1b5vCqs2Ja/xfREveNU6Psadbf2iOKN\n" +
		"Vr8ipa5YFCfHM7OoFARYaIWB4q6cRavf5fjfhPTV7kc6twX5liLRsM4kWe2MaTQ0\n" +
		"snGCauWA03+wi+FJXYcUAqZSTLj0pJ2UA71StwIDAQABAoIBAE7RlqxnTaP/5GEe\n" +
		"f7dM6bkNU0p/F2EsemQVy/ORLJFLOTusM6VIrZr1WRLFLntiX/56KztDNDVlmNTz\n" +
		"69hihjPAqr94GLyz2u7yQMPC3AAytn31O1bIWmRHsN2DSusiePymQFddQqGD8T1B\n" +
		"SGMY3rTlGAffrChoE3XopEg1R2k81w14u3ZqCN/aU85t7h98lGnv+DRUn/TqhiX6\n" +
		"rdzW6xJGUAKidyhuEvM3to19EZQUkJHO2LjNDnbMowsia25HdphJL/P3ZxPKwupe\n" +
		"MZtP6pDaSQTFkhAG0duAk1KufwMPAMO7UaYHlNqCtUaxevIPMGWXN5y/ILXTAHtv\n" +
		"voRisqkCgYEAw0MFdcb6ugg5ViRSaEwmUQng3XkNDgvJRBTQySTVq0ni7fyCGR93\n" +
		"VwEGLEpGoXfMavBf1EXwFcI0aCWB/qfNEDErnpYtKKxGajHHHJyllSwQ/gtxfri2\n" +
		"XoExuZG3mauQjqkmydxKVztOXIlOjZSzukPNtqXHJYriA+lkvwvLeisCgYEAtHTC\n" +
		"EpQUEgWxPPOK7vfXO/JGDC0k7dmMFNNKjjBHM9I6MkHteGbq+b+MCI7aybVYqaQ0\n" +
		"3CTGya9qLk7/IEkICFoj/BHpRoVFLTTTl/QYZvUtCaeRDsYTgpc/c+bJF/7/sj08\n" +
		"7ruKiyRMEuR8lWlCDB4c5Oyun7YrkH+86HMVP6UCgYBjUNWYIEsrED/JltPrhMAA\n" +
		"fBvJymZffJM0c7n2dSvQ4dXw4nxxttWGhVjUcjsWqc5pnjW/zIrfJlZtmpZSJptg\n" +
		"3wGmug/iHi36mbMC1JJMG4vRC5UAtYbc7q2SC5HtMZxnU5YNGmUdlWa4Hoa78KSx\n" +
		"2wbpHcz7RXbMMowxuBgY3QKBgQChcihDSNnf+dnE5zrwaynUBwAmWqlEZrKN2y9D\n" +
		"oOvC8B2C4zra0nD9OiLFcVFKzwTg2Pk1z21N+bMsdR6Juu0F0+eH2Fp07jyiojWA\n" +
		"KDFAw68kiRcdOZcw6bIqNlrJLimDRIhkKcNckv/Ak0zmu4IMp1BAe4QLfYbiQ3Y2\n" +
		"HOfwxQKBgAPNqWUgdqhy9Za3fmlp4I3xj11olnYB/T44qfI6hfz6m3C4gGWrAkBZ\n" +
		"+1K92SPGT46FUMFc2tKv/ZIlDndVcQWpj/FUgUKzG+Ppktfpy+uAde6A9ffVQsq9\n" +
		"Buas8uvcttBXolR+GfI6Gn1ssqT1ejjGTK6YaY508liOlrCWB7hu\n" +
		"-----END RSA PRIVATE KEY-----"

	MalformattedRSAPrvtKey = "-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEowIBAAKCAQEAiaQuPD8SJZ+kw5NF0Mlf3FvKfeXxAiXUFnURp/+CgNPOy+2v\n" +
		"zeDthLP5zpZrk8OV5GpX7///E448ATSo5lVoZz2pdvpHPAMGtf06gdX3B79FCzui\n" +
		"dxXz0kGdr0bX1Ozy3OKWCmKSB&A9DtOR4Me+Hg2+neLKgBf7rNWUPc/9CwYkbIt6\n" +
		"BmMjhHk1Ijs4e8YUF809WYdJlN8c0ErKJ1b5vCqs2Ja/xfREveNU6Psadbf2iOKN\n" +
		"Vr8ipa5YFCfHM7OoFARYaIWB4q6cRavf5fjfhPTV7kc6twX5liLRsM4kWe2MaTQ0\n" +
		"snGCauWA03+wi+FJXYcUAqZSTLj0pJ2UA71StwIDAQABAoIBAE7RlqxnTaP/5GEe\n" +
		"f7dM6bkNU0p/F2EsemQVy/ORLJFLOTusM6VIrZr1WRLFLntiX/56KztDNDVlmNTz\n" +
		"69hihjPAqr94GLyz2u7yQMPC3AAytn31O1bIWmRHsN2DSusiePymQFddQqGD8T1B\n" +
		"SGMY3rTlGAffrChoE3XopEg1R2k81w14u3ZqCN/aU85t7h98lGnv+DRUn/TqhiX6\n" +
		"rdzW6xJGUAKidyhuEvM3to19EZQUkJHO2LjNDnbMowsia25HdphJL/P3ZxPKwupe\n" +
		"MZtP6pDaSQTFkhAG0duAk1KufwMPAMO7UaYHlNqCtUaxevIPMGWXN5y/ILXTAHtv\n" +
		"voRisqkCgYEAw0MFdcb6ugg5ViRSaEwmUQng3XkNDgvJRBTQySTVq0ni7fyCGR93\n" +
		"VwEGLEpGoXfMavBf1EXwFcI0aCWB/qfNEDErnpYtKKxGajHHHJyllSwQ/gtxfri2\n" +
		"XoExuZG3mauQjqkmydxKVztOXIlOjZSzukPNtqXHJYriA+lkvwvLeisCgYEAtHTC\n" +
		"EpQUEgWxPPOK7vfXO/JGDC0k7dmMFNNKjjBHM9I6MkHteGbq+b+MCI7aybVYqaQ0\n" +
		"3CTGya9qLk7/IEkICFoj/BHpRoVFLTTTl/QYZvUtCaeRDsYTgpc/c+bJF/7/sj08\n" +
		"7ruKiyRMEuR8lWlCDB4c5Oyun7YrkH+86HMVP6UCgYBjUNWYIEsrED/JltPrhMAA\n" +
		"fBvJymZffJM0c7n2dSvQ4dXw4nxxttWGhVjUcjsWqc5pnjW/zIrfJlZtmpZSJptg\n" +
		"3wGmug/iHi36mbMC1JJMG4vRC5UAtYbc7q2SC5HtMZxnU5YNGmUdlWa4Hoa78KSx\n" +
		"2wbpHcz7RXbMMowxuBgY3QKBgQChcihDSNnf+dnE5zrwaynUBwAmWqlEZrKN2y9D\n" +
		"oOvC8B2C4zra0nD9OiLFcVFKzwTg2Pk1z21N+bMsdR6Juu0F0+eH2Fp07jyiojWA\n" +
		"KDFAw68kiRcdOZcw6bIqNlrJLimDRIhkKcNckv/Ak0zmu4IMp1BAe4QLfYbiQ3Y2\n" +
		"HOfwxQKBgAPNqWUgdqhy9Za3fmlp4I3xj11olnYB/T44qfI6hfz6m3C4gGWrAkBZ\n" +
		"+1K92SPGT46FUMFc2tKv/ZIlDndVcQWpj/FUgUKzG+Ppktfpy+uAde6A9ffVQsq9\n" +
		"Buas8uvcttBXolR+GfI6Gn1ssqT1ejjGTK6YaY508liOlrCWB7hu\n" +
		"-----END RSA PRIVATE KEY-----"
)

func parsePrivateKey(t *testing.T, key string) *rsa.PrivateKey {
	block, _ := pem.Decode([]byte(key))
	prvtKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	return prvtKey
}

func parsePublicKey(t *testing.T, key string) crypto.PublicKey {
	block, _ := pem.Decode([]byte(key))
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err)
	return pubKey
}
