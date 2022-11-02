package sign_test

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/kyma-project/runtime-watcher/skr/pkg/sign"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSign(t *testing.T) {
	RSAPrvtKey := "-----BEGIN RSA PRIVATE KEY-----\n" +
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

	MalFormattedPrvtKey := "-----BEGIN RSA PRIVATE KEY-----\n" +
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
		"2+kEbxMlksVo0VPsFkn73H00ZdOzW3Yw5VCnYBKuWi2T50leoOs39McCamKs3YOl\n" +
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
		prvtKey           string
		sigString         []byte
		expectedSignature []byte
		expectError       bool
	}{
		{
			testName:  "Add sha256 digest",
			prvtKey:   RSAPrvtKey,
			sigString: []byte(fmt.Sprintf("created=%v", 00000000000)),
			expectedSignature: []byte{56, 235, 150, 44, 254, 118, 164, 73, 201, 122, 78, 89, 222, 106, 175, 28, 242,
				133, 63, 77, 109, 97, 157, 20, 187, 51, 69, 120, 207, 44, 40, 15, 28, 102, 56, 213, 71, 235, 181, 99,
				180, 98, 42, 32, 14, 113, 1, 4, 63, 249, 50, 84, 112, 171, 15, 166, 178, 46, 216, 87, 16, 224, 162,
				103, 104, 49, 42, 4, 2, 116, 99, 188, 43, 112, 21, 114, 110, 113, 105, 241, 191, 243, 23, 171, 135,
				213, 225, 110, 179, 247, 109, 68, 111, 148, 19, 13, 134, 180, 22, 84, 109, 165, 18, 157, 80, 171, 27,
				41, 178, 11, 131, 144, 124, 11, 132, 236, 98, 44, 224, 96, 79, 223, 117, 112, 68, 151, 32, 1, 144,
				197, 125, 83, 247, 30, 41, 172, 213, 174, 23, 178, 23, 111, 232, 239, 220, 123, 71, 192, 113, 187,
				198, 75, 253, 136, 44, 59, 42, 212, 46, 125, 49, 224, 231, 184, 108, 182, 253, 99, 58, 203, 86, 71,
				7, 36, 221, 142, 237, 37, 121, 10, 126, 70, 115, 139, 136, 1, 17, 128, 146, 89, 196, 238, 51, 136,
				32, 27, 158, 221, 58, 200, 73, 35, 3, 165, 164, 231, 65, 33, 57, 117, 154, 70, 229, 40, 179, 141,
				199, 207, 66, 46, 0, 237, 222, 75, 165, 1, 59, 141, 53, 187, 218, 139, 69, 18, 101, 78, 57, 211, 248,
				4, 10, 179, 176, 116, 161, 200, 163, 170, 22, 129, 130, 134, 20, 149, 54, 38},
			expectError: false,
		},
		{
			testName:    "Private Key is empty",
			expectError: true,
		},
		{
			testName:    "Malformatted Private Key",
			prvtKey:     MalFormattedPrvtKey,
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
				return
			}
			require.NoError(t, err)
			fmt.Println(signature)
			require.EqualValues(t, test.expectedSignature, signature)

		})
	}

}
