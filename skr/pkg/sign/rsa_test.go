package sign_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/kyma-project/runtime-watcher/skr/pkg/sign"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"testing"
	"time"
)

// TODO Fix tests, only private Key PEM blocks is not sufficient enough to encrypt signature, since SignPKCS1v15() needs
// a fully rsa.PrivtaeKey incl the publicKey -> Search for applicabale solution
func TestAign(t *testing.T) {
	prvtKey := "-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEpAIBAAKCAQEAsFfQ7fGgyVxvw5bSyi/V/7TimRAaZie9ulE6z23yaKuSCYcm\n" +
		"4bavVasnLj6hx7TidpbpHlU/Y0AmYBPXb7y05CvB+95mffehmatmZgtoIk3FFTry\n" +
		"VNz7oM8O7gMPMEoq4gTyulwById03jx0wHyZ6cZGRnfoT63sd8rnO2W0tSasnK2p\n" +
		"bopP4D5udPL0kTadoZ/MQm0Sd3PfSCJWAeSeK6tKS80YwKiMq+Mj6uDVPbZQ947S\n" +
		"42h2b2sQVEOsr681u6biqw3hIdWNFPSEpDHlqYkmZbpTAqEjI9uKA8wa3wMmLgns\n" +
		"oOLZtaqwOLwBa0oPc21qDFoVqBoCRPDO8ZEOaQIDAQABAoIBAFk9B2jyZyifU5vK\n" +
		"Hn/c91HAqw9EW+eoYtX/t2AzRoH7mRqjP2Tn+xDCXUCEx3/1pMjYk74a68oBM6pZ\n" +
		"QCO2fmAdWLxqDrneb/QBDf/D8/2wF3Un8GxLrDbzsZ13BN+uGMdqM59lYi2lhtnU\n" +
		"BE1IgOcRxIxyr6hq3oi8sImZbXph8aq/Crua/Y+ZsMpcMTV6ijU3koyuMB4EgVQS\n" +
		"cxWpZKuxJSdQ5PXFKsoc4417yUQYHE5IqpDvMJgLJPUz3rAkceHSaGqXK8ilZ6CF\n" +
		"cP7OctQ8jUrHLytSA5Vr5xRrtKdGK1Ht+KuPVhfzEA0WS6xGhd1MnmIFrLZ/3AD9\n" +
		"PP5AkEUCgYEAy8vkMgjnrEIxtr64D10Nf7hxlfciORBiALBqOuYN+HrGJKrsKrZx\n" +
		"YCzNaYThguVe0dIAw44bvuBkhzdl77XJOPzNTW97Jk9ryGu6kcbotK7DxZpiODnp\n" +
		"2+kEbxMlksVo0VPsFkn73H00ZdOzW3Yw5VCnYBKuWi2T50leoOs39McCgYEA3YOl\n" +
		"TpboXwGrPUPnhKo9Y6lUr28QbQLXfPlVybwA7eLOtYejHzByV9wIF1zqznmVo5ut\n" +
		"v1oqHdFlNXmUhkWLGpPKjV/yvE4thvTBjT/0RxLf25vEtgor5bhmspOxRqC1pPN8\n" +
		"kVPiVld5yYxb+/cyRADmIw6Ytv4W5uF4pUGQU08CgYARdeucOdUXlihKPvboIhHZ\n" +
		"AoWA0sa02ul6o6LGXxWNV3+IfrhzRGRcWBpVUxQ7Mcm48mQsXQ2VggY6640pR4rw\n" +
		"/f/dBZMoih9y8X/vo3ommN6fHIYTySp3M/S0S5CpjY5YePc+RaJ1lqiZnNS+Hlc8\n" +
		"HnforFER2tvUMh4QbXbC2wKBgQChh5QN4QGF9kOWo2O6XCH0ANCeNVE3DPFyUqd6\n" +
		"Ojw7PD8cJNKQtdVLuEm2L62R7xtteOKUPP2lTMKO4h+qYh/zu33i5eqt4hxU4zoY\n" +
		"9F//TAYtsEMbtAMauwM4iXamWB7dMCjQGOldqOBIVq/k5veimz02pzg5iMPOjPBb\n" +
		"IZpLBQKBgQCT7DI71WNjG5l4ZZ8cmYTlfj3Qb5Id5eMtyhULrSjYADY45swIR+AS\n" +
		"UOCbWMYY34bsZsvFyLl8gfEAi3K1FddiW2tE8cnKmVwJ/EBOGn1q2zt+fUOAPI8F\n" +
		"tZ6CLBf93xdJL1hfp+YkB1H6KysC1R/dM4F63jrPo6f5Byyso5EFzg==\n" +
		"-----END RSA PRIVATE KEY-----"

	tests := []struct {
		testName          string
		r                 func() *http.Request
		prvtKey           crypto.PrivateKey
		pubKeySecret      types.NamespacedName
		sigString         []byte
		expectedSignature []byte
		expectError       bool
	}{
		{
			testName: "Add sha256 digest",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			prvtKey: prvtKey,
			pubKeySecret: types.NamespacedName{
				Namespace: "default",
				Name:      "kyma-1",
			},
			sigString:         []byte(fmt.Sprintf("created=%v", time.Now().Unix())),
			expectedSignature: []byte(""),
			expectError:       false,
		},
		{
			testName: "Digest is already set",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				r.Header.Set("Digest", "should fail")
				return r
			},
			expectError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			test := test

			rsa := &sign.RSAAlgorithm{
				Hash: sha256.New(),
				Kind: crypto.SHA256,
			}
			signature, err := rsa.Sign(rand.Reader, test.prvtKey, test.sigString)
			if test.expectError {
				require.Error(t, err)
			}
			require.NoError(t, err)
			require.EqualValues(t, test.expectedSignature, signature)

		})
	}
}
