package signature_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"testing"

	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"github.com/kyma-project/runtime-watcher/skr/pkg/signature"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSign(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName          string
		prvtKey           *rsa.PrivateKey
		sigString         []byte
		expectedSignature []byte
		expectError       bool
	}{
		{
			testName:          "Add sha256 digest",
			prvtKey:           parsePrivateKey(t, rsaPrvtKey),
			sigString:         []byte(fmt.Sprintf("created=%v", 0o0000000000)),
			expectedSignature: validTestSignature,
			expectError:       false,
		},
		{
			testName:    "Private Key is empty",
			expectError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()
	tests := []struct {
		testName          string
		pubKey            crypto.PublicKey
		sigString         []byte
		ReceivedSignature []byte
		expectError       bool
	}{
		{
			testName:          "Verify received signature",
			pubKey:            parsePublicKey(t, rsaPubKey),
			sigString:         []byte(fmt.Sprintf("created=%v", 0o0000000000)),
			ReceivedSignature: validTestSignature,
			expectError:       false,
		},
		{
			testName:    "Public Key is empty",
			expectError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

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
	rsaPubKey = "-----BEGIN PUBLIC KEY-----\n" + //nolint:gochecknoglobals
		"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAiaQuPD8SJZ+kw5NF0Mlf\n" +
		"3FvKfeXxAiXUFnURp/+CgNPOy+2vzeDthLP5zpZrk8OV5GpX7///E448ATSo5lVo\n" +
		"Zz2pdvpHPAMGtf06gdX3B79FCzuidxXz0kGdr0bX1Ozy3OKWCmudPsR9DtOR4Me+\n" +
		"Hg2+neLKgBf7rNWUPc/9CwYkbIt6BmMjhHk1Ijs4e8YUF809WYdJlN8c0ErKJ1b5\n" +
		"vCqs2Ja/xfREveNU6Psadbf2iOKNVr8ipa5YFCfHM7OoFARYaIWB4q6cRavf5fjf\n" +
		"hPTV7kc6twX5liLRsM4kWe2MaTQ0snGCauWA03+wi+FJXYcUAqZSTLj0pJ2UA71S\n" +
		"twIDAQAB\n" +
		"-----END PUBLIC KEY-----"
	rsaPrvtKey = "-----BEGIN RSA PRIVATE KEY-----\n" + //nolint:gochecknoglobals
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

	rsaPrvtKeyEncoded = []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFB" + //nolint:gochecknoglobals
		"S0NBUUVBaWFRdVBEOFNKWitrdzVORjBNbGYzRnZLZmVYeEFpWFVGblVScC8rQ2dOUE95KzJ2CnplRHRoTFA1enBacms" +
		"4T1Y1R3BYNy8vL0U0NDhBVFNvNWxWb1p6MnBkdnBIUEFNR3RmMDZnZFgzQjc5RkN6dWkKZHhYejBrR2RyMGJYMU96eT" +
		"NPS1dDbXVkUHNSOUR0T1I0TWUrSGcyK25lTEtnQmY3ck5XVVBjLzlDd1lrYkl0NgpCbU1qaEhrMUlqczRlOFlVRjgwO" +
		"VdZZEpsTjhjMEVyS0oxYjV2Q3FzMkphL3hmUkV2ZU5VNlBzYWRiZjJpT0tOClZyOGlwYTVZRkNmSE03T29GQVJZYUlX" +
		"QjRxNmNSYXZmNWZqZmhQVFY3a2M2dHdYNWxpTFJzTTRrV2UyTWFUUTAKc25HQ2F1V0EwMyt3aStGSlhZY1VBcVpTVEx" +
		"qMHBKMlVBNzFTdHdJREFRQUJBb0lCQUU3UmxxeG5UYVAvNUdFZQpmN2RNNmJrTlUwcC9GMkVzZW1RVnkvT1JMSkZMT1" +
		"R1c002VklyWnIxV1JMRkxudGlYLzU2S3p0RE5EVmxtTlR6CjY5aGloalBBcXI5NEdMeXoydTd5UU1QQzNBQXl0bjMxT" +
		"zFiSVdtUkhzTjJEU3VzaWVQeW1RRmRkUXFHRDhUMUIKU0dNWTNyVGxHQWZmckNob0UzWG9wRWcxUjJrODF3MTR1M1px" +
		"Q04vYVU4NXQ3aDk4bEduditEUlVuL1RxaGlYNgpyZHpXNnhKR1VBS2lkeWh1RXZNM3RvMTlFWlFVa0pITzJMak5EbmJ" +
		"Nb3dzaWEyNUhkcGhKTC9QM1p4UEt3dXBlCk1adFA2cERhU1FURmtoQUcwZHVBazFLdWZ3TVBBTU83VWFZSGxOcUN0VW" +
		"F4ZXZJUE1HV1hONXkvSUxYVEFIdHYKdm9SaXNxa0NnWUVBdzBNRmRjYjZ1Z2c1VmlSU2FFd21VUW5nM1hrTkRndkpSQ" +
		"lRReVNUVnEwbmk3ZnlDR1I5MwpWd0VHTEVwR29YZk1hdkJmMUVYd0ZjSTBhQ1dCL3FmTkVERXJucFl0S0t4R2FqSEhI" +
		"SnlsbFN3US9ndHhmcmkyClhvRXh1WkczbWF1UWpxa215ZHhLVnp0T1hJbE9qWlN6dWtQTnRxWEhKWXJpQStsa3Z3dkx" +
		"laXNDZ1lFQXRIVEMKRXBRVUVnV3hQUE9LN3ZmWE8vSkdEQzBrN2RtTUZOTktqakJITTlJNk1rSHRlR2JxK2IrTUNJN2" +
		"F5YlZZcWFRMAozQ1RHeWE5cUxrNy9JRWtJQ0Zvai9CSHBSb1ZGTFRUVGwvUVladlV0Q2FlUkRzWVRncGMvYytiSkYvN" +
		"y9zajA4CjdydUtpeVJNRXVSOGxXbENEQjRjNU95dW43WXJrSCs4NkhNVlA2VUNnWUJqVU5XWUlFc3JFRC9KbHRQcmhN" +
		"QUEKZkJ2SnltWmZmSk0wYzduMmRTdlE0ZFh3NG54eHR0V0doVmpVY2pzV3FjNXBualcveklyZkpsWnRtcFpTSnB0Zwo" +
		"zd0dtdWcvaUhpMzZtYk1DMUpKTUc0dlJDNVVBdFliYzdxMlNDNUh0TVp4blU1WU5HbVVkbFdhNEhvYTc4S1N4CjJ3Yn" +
		"BIY3o3UlhiTU1vd3h1QmdZM1FLQmdRQ2hjaWhEU05uZitkbkU1enJ3YXluVUJ3QW1XcWxFWnJLTjJ5OUQKb092QzhCM" +
		"kM0enJhMG5EOU9pTEZjVkZLendUZzJQazF6MjFOK2JNc2RSNkp1dTBGMCtlSDJGcDA3anlpb2pXQQpLREZBdzY4a2lS" +
		"Y2RPWmN3NmJJcU5sckpMaW1EUkloa0tjTmNrdi9BazB6bXU0SU1wMUJBZTRRTGZZYmlRM1kyCkhPZnd4UUtCZ0FQTnF" +
		"XVWdkcWh5OVphM2ZtbHA0STN4ajExb2xuWUIvVDQ0cWZJNmhmejZtM0M0Z0dXckFrQloKKzFLOTJTUEdUNDZGVU1GYz" +
		"J0S3YvWklsRG5kVmNRV3BqL0ZVZ1VLekcrUHBrdGZweSt1QWRlNkE5ZmZWUXNxOQpCdWFzOHV2Y3R0QlhvbFIrR2ZJN" +
		"kduMXNzcVQxZWpqR1RLNllhWTUwOGxpT2xyQ1dCN2h1Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0t")

	malformattedRSAPrvtKeyEncoded = []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFB" + //nolint:gochecknoglobals,lll
		"S0NBUUVBaWFRdVBEOFNKWitrdzVORjBNbGYzRnZLZmVYeEFpWFVGblVScC8rQ2dOUE95KzJ2CnplRHRoTFA1enBacms" +
		"4T1Y1R3BYNy8vL0U0NDhBVFNvNWxWb1p6MnBkdnBIUEFNR3RmMDZnZFgzQjc5RkN6dWkKZHhYejBrR2RyMGJYMU96eT" +
		"NPS1dDbUtTQiZBOUR0T1I0TWUrSGcyK25lTEtnQmY3ck5XVVBjLzlDd1lrYkl0NgpCbU1qaEhrMUlqczRlOFlVRjgwO" +
		"VdZZEpsTjhjMEVyS0oxYjV2Q3FzMkphL3hmUkV2ZU5VNlBzYWRiZjJpT0tOClZyOGlwYTVZRkNmSE03T29GQVJZYUlX" +
		"QjRxNmNSYXZmNWZqZmhQVFY3a2M2dHdYNWxpTFJzTTRrV2UyTWFUUTAKc25HQ2F1V0EwMyt3aStGSlhZY1VBcVpTVEx" +
		"qMHBKMlVBNzFTdHdJREFRQUJBb0lCQUU3UmxxeG5UYVAvNUdFZQpmN2RNNmJrTlUwcC9GMkVzZW1RVnkvT1JMSkZMT1" +
		"R1c002VklyWnIxV1JMRkxudGlYLzU2S3p0RE5EVmxtTlR6CjY5aGloalBBcXI5NEdMeXoydTd5UU1QQzNBQXl0bjMxT" +
		"zFiSVdtUkhzTjJEU3VzaWVQeW1RRmRkUXFHRDhUMUIKU0dNWTNyVGxHQWZmckNob0UzWG9wRWcxUjJrODF3MTR1M1px" +
		"Q04vYVU4NXQ3aDk4bEduditEUlVuL1RxaGlYNgpyZHpXNnhKR1VBS2lkeWh1RXZNM3RvMTlFWlFVa0pITzJMak5EbmJ" +
		"Nb3dzaWEyNUhkcGhKTC9QM1p4UEt3dXBlCk1adFA2cERhU1FURmtoQUcwZHVBazFLdWZ3TVBBTU83VWFZSGxOcUN0VW" +
		"F4ZXZJUE1HV1hONXkvSUxYVEFIdHYKdm9SaXNxa0NnWUVBdzBNRmRjYjZ1Z2c1VmlSU2FFd21VUW5nM1hrTkRndkpSQ" +
		"lRReVNUVnEwbmk3ZnlDR1I5MwpWd0VHTEVwR29YZk1hdkJmMUVYd0ZjSTBhQ1dCL3FmTkVERXJucFl0S0t4R2FqSEhI" +
		"SnlsbFN3US9ndHhmcmkyClhvRXh1WkczbWF1UWpxa215ZHhLVnp0T1hJbE9qWlN6dWtQTnRxWEhKWXJpQStsa3Z3dkx" +
		"laXNDZ1lFQXRIVEMKRXBRVUVnV3hQUE9LN3ZmWE8vSkdEQzBrN2RtTUZOTktqakJITTlJNk1rSHRlR2JxK2IrTUNJN2" +
		"F5YlZZcWFRMAozQ1RHeWE5cUxrNy9JRWtJQ0Zvai9CSHBSb1ZGTFRUVGwvUVladlV0Q2FlUkRzWVRncGMvYytiSkYvN" +
		"y9zajA4CjdydUtpeVJNRXVSOGxXbENEQjRjNU95dW43WXJrSCs4NkhNVlA2VUNnWUJqVU5XWUlFc3JFRC9KbHRQcmhN" +
		"QUEKZkJ2SnltWmZmSk0wYzduMmRTdlE0ZFh3NG54eHR0V0doVmpVY2pzV3FjNXBualcveklyZkpsWnRtcFpTSnB0Zwo" +
		"zd0dtdWcvaUhpMzZtYk1DMUpKTUc0dlJDNVVBdFliYzdxMlNDNUh0TVp4blU1WU5HbVVkbFdhNEhvYTc4S1N4CjJ3Yn" +
		"BIY3o3UlhiTU1vd3h1QmdZM1FLQmdRQ2hjaWhEU05uZitkbkU1enJ3YXluVUJ3QW1XcWxFWnJLTjJ5OUQKb092QzhCM" +
		"kM0enJhMG5EOU9pTEZjVkZLendUZzJQazF6MjFOK2JNc2RSNkp1dTBGMCtlSDJGcDA3anlpb2pXQQpLREZBdzY4a2lS" +
		"Y2RPWmN3NmJJcU5sckpMaW1EUkloa0tjTmNrdi9BazB6bXU0SU1wMUJBZTRRTGZZYmlRM1kyCkhPZnd4UUtCZ0FQTnF" +
		"XVWdkcWh5OVphM2ZtbHA0STN4ajExb2xuWUIvVDQ0cWZJNmhmejZtM0M0Z0dXckFrQloKKzFLOTJTUEdUNDZGVU1GYz" +
		"J0S3YvWklsRG5kVmNRV3BqL0ZVZ1VLekcrUHBrdGZweSt1QWRlNkE5ZmZWUXNxOQpCdWFzOHV2Y3R0QlhvbFIrR2ZJN" +
		"kduMXNzcVQxZWpqR1RLNllhWTUwOGxpT2xyQ1dCN2h1Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0t")

	rsaPubKeyEncoded = []byte("LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtp" + //nolint:gochecknoglobals
		"Rzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUFpYVF1UEQ4U0paK2t3NU5GME1sZgozRnZLZmVYeEFpWFVGblVScC8" +
		"rQ2dOUE95KzJ2emVEdGhMUDV6cFpyazhPVjVHcFg3Ly8vRTQ0OEFUU281bFZvClp6MnBkdnBIUEFNR3RmMDZnZFgzQj" +
		"c5RkN6dWlkeFh6MGtHZHIwYlgxT3p5M09LV0NtdWRQc1I5RHRPUjRNZSsKSGcyK25lTEtnQmY3ck5XVVBjLzlDd1lrY" +
		"kl0NkJtTWpoSGsxSWpzNGU4WVVGODA5V1lkSmxOOGMwRXJLSjFiNQp2Q3FzMkphL3hmUkV2ZU5VNlBzYWRiZjJpT0tO" +
		"VnI4aXBhNVlGQ2ZITTdPb0ZBUllhSVdCNHE2Y1JhdmY1ZmpmCmhQVFY3a2M2dHdYNWxpTFJzTTRrV2UyTWFUUTBzbkd" +
		"DYXVXQTAzK3dpK0ZKWFljVUFxWlNUTGowcEoyVUE3MVMKdHdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t")

	malformattedRSAPubKeyEncoded = []byte("LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtp" + //nolint:gochecknoglobals,lll
		"Rzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUFpYVF1UEQ4U0paK2t3NU5GME1sZgozRnZLZmVYeEFpWFVGblVScC8" +
		"rQ2dOUE95KzJ2emVEdGhMUDV6cFpyazhPVjVHcFg3Ly8vRTQ0OEFUU281bFZvClp6MnBkdnBIUEFNR3RmMDZnZFgzQj" +
		"c5RkN6dWlsam5hc2tHZHIwYlgxT3p5M09LV0NtdWRQc1I5RHRPUjRNZSsKSGcyK25lTEtnQmY3ck5XVVBjLzlDd1lrY" +
		"kl0NkJtTWpoSGsxSWpzNGU4WVVGODA5V1lkSmxOOGMwRXJLSjFiNQp2Q3FzMkphL3hmUkV2ZU5VNlBzYWRiZjJpT0tO" +
		"VnI4aXBhNVlGQ2ZITTdPb0ZBUllhSVdCNHE2Y1JhdmY1ZmpmCmhQVFY3a2M2dHdYNWxpTFJzTTRrV2UyTWFUUTBzbkd" +
		"DYXVXQTAzK3dpK0ZKWFljVUFxWlNUTGowcEoyVUE3MVMKdHdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t")

	rsaPrvtKey2Encoded = []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlDWEFJQkFBS0JnUUNHSXN" + //nolint:gochecknoglobals,lll
		"Xbk1tUjNLbXBOWVEyWU8wK0NOU1FwUWlFZ080K1Vpdk5Xa3R2QnF0WUhkMUFzCk5BeXRhdUlHUDNaR0lBVTBjTmkzSl" +
		"VmNWoyK29mMS9qZEQweFRkbElPRE9sT3hiSVNTaUpoNDMvR2l0azBtWG0KNDhZdEtEeitPVTRnVEM1Y3A5RTREVmI1d" +
		"jI5bmVabG9OaWs5UG9kbmc1elVISVNOYjRKazRob01rd0lEQVFBQgpBb0dBYlpick5YY09iZTNSZS9iWFRKRG9uTkVl" +
		"QWpkdEtSQ0FkalF3SzROQTJESGpPNlpYY2tYME5ac2xuMFVxCk5KRGtyN3VpMDc4NTFzTkJ6c0NDYnlzQThwK2grNVZ" +
		"JRTQ0UGI5N2dGVjgxOUxKVFgwNFM2dGFGM09HS2NMSXEKL1lGZDBYVytialpXQTljK3pZcUlHRW4vUm9yVmF2YWhGWU" +
		"8xTjlkQzdudlNBSUVDUVFESC9yek14cktWZ280WQpydWgwa0x0NEl4V3M2dnZMcnJ4YXNwc0RvQi9MaHdzRk50VHVCN" +
		"0p0WnBlU3BMaXloTXBDOW1qWmpCQy9OL1FCCnpHUXFxSHFKQWtFQXE3SzJUTmpoM1VzYThyUU5vTU1iYnBaYm04L0p6" +
		"NGV2Vjd5MllRbE52cG5YZlVYTXlBRmsKdFc1RGUraU1GZkpTUlEvV3RvMnoyY2xQNUlsOTZVZVhPd0pBWmZrMU94UjF" +
		"LbGFQTFhiQmYrM3NLSzE2OTlnNAoydm9WZ0FsaGtNK3NacEpNeERQWkRpVk9qUW1xYjFNZCthaExtU2thL1JHMTJFb2" +
		"5XR05uRDNrb1FRSkJBSTR1CkdXUWRuVHZoTzltTFhGV3ArNGRpSDA0eGpVNjdiMm5hTGJUQlBZMytXMEd6a1ZaMlFPM" +
		"DA5OUVkeXhOSmJQTWYKb0kvZlcvV1hEUCtWRTUwZjJZMENRRDZRUGhrS0JpekhzSDA4UXVoK2p4TzIwMUpLV3kxdDJB" +
		"ZE14a2hvVzYwNAo0clppNE91cHJ4akk4V3pLaS9Yd3VXY0k0dU5TTWJxMlpTTFZEUWl2SHZnPQotLS0tLUVORCBSU0E" +
		"gUFJJVkFURSBLRVktLS0tLQ==")

	validTestSignature = []byte{ //nolint:gochecknoglobals
		39, 198, 186, 119, 234, 206, 24, 15, 131, 168, 150,
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
		204, 193, 74, 255, 142, 34, 151, 58, 214,
	}
)

func parsePrivateKey(t *testing.T, key string) *rsa.PrivateKey {
	t.Helper()
	block, _ := pem.Decode([]byte(key))
	prvtKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	return prvtKey
}

func parsePublicKey(t *testing.T, key string) crypto.PublicKey {
	t.Helper()
	block, _ := pem.Decode([]byte(key))
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err)
	return pubKey
}

func createPostBody(t *testing.T) []byte {
	t.Helper()
	watcherEvent := &listenerTypes.WatchEvent{
		Owner:      client.ObjectKey{Namespace: "default", Name: "ownerName"},
		Watched:    client.ObjectKey{Namespace: "default", Name: "watchedName"},
		WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind("v1", "watchedKind")),
	}
	postBody, err := json.Marshal(watcherEvent)
	require.NoError(t, err)
	return postBody
}
