package signature_test

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"github.com/kyma-project/runtime-watcher/skr/pkg/signature"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestSignRequest(t *testing.T) {

	tests := []struct {
		testName            string
		prvtKey             *rsa.PrivateKey
		keyreference        types.NamespacedName
		r                   func() *http.Request
		k8sclient           client.Client
		expectedHeaderValue string
		expectError         bool
	}{
		{
			testName: "Sign given request with a RSA Private Key",
			keyreference: types.NamespacedName{
				Namespace: "default",
				Name:      "kyma-1",
			},
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PvtKeyKey:          RSAPrvtKeyEncoded,
						signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
						signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
					},
				},
			).Build()),
			expectError: false,
		},
		{
			testName: "Private Key is empty",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			k8sclient:   client.Client(fake.NewClientBuilder().Build()),
			expectError: true,
		},
		{
			testName: "Secret does not exist",
			keyreference: types.NamespacedName{
				Namespace: "default",
				Name:      "kyma-1",
			},
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			k8sclient:   client.Client(fake.NewClientBuilder().Build()),
			expectError: true,
		},
		{
			testName: "Malformatted Private Key",
			keyreference: types.NamespacedName{
				Namespace: "default",
				Name:      "kyma-1",
			},
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PvtKeyKey:          MalformattedRSAPrvtKeyEncoded,
						signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
						signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
					},
				},
			).Build()),
			expectError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			test := test
			req := test.r()

			pod := &v1.SecretList{}
			x := test.k8sclient.List(req.Context(), pod)
			fmt.Println(x)

			err := signature.SignRequest(req, test.keyreference, test.k8sclient)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			headerValue := req.Header.Get(signature.SignatureHeader)

			r, _ := regexp.Compile(fmt.Sprintf("pubKeySecretName=\"%s\",pubKeySecretNamespace=\"%s\","+
				"created=[0-9]*,Signature=\"*\"", test.keyreference.Name, test.keyreference.Namespace))
			require.True(t, r.Match([]byte(headerValue)))

		})
	}
}

var (
	RSAPrvtKeyEncoded             = []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBaWFRdVBEOFNKWitrdzVORjBNbGYzRnZLZmVYeEFpWFVGblVScC8rQ2dOUE95KzJ2CnplRHRoTFA1enBacms4T1Y1R3BYNy8vL0U0NDhBVFNvNWxWb1p6MnBkdnBIUEFNR3RmMDZnZFgzQjc5RkN6dWkKZHhYejBrR2RyMGJYMU96eTNPS1dDbXVkUHNSOUR0T1I0TWUrSGcyK25lTEtnQmY3ck5XVVBjLzlDd1lrYkl0NgpCbU1qaEhrMUlqczRlOFlVRjgwOVdZZEpsTjhjMEVyS0oxYjV2Q3FzMkphL3hmUkV2ZU5VNlBzYWRiZjJpT0tOClZyOGlwYTVZRkNmSE03T29GQVJZYUlXQjRxNmNSYXZmNWZqZmhQVFY3a2M2dHdYNWxpTFJzTTRrV2UyTWFUUTAKc25HQ2F1V0EwMyt3aStGSlhZY1VBcVpTVExqMHBKMlVBNzFTdHdJREFRQUJBb0lCQUU3UmxxeG5UYVAvNUdFZQpmN2RNNmJrTlUwcC9GMkVzZW1RVnkvT1JMSkZMT1R1c002VklyWnIxV1JMRkxudGlYLzU2S3p0RE5EVmxtTlR6CjY5aGloalBBcXI5NEdMeXoydTd5UU1QQzNBQXl0bjMxTzFiSVdtUkhzTjJEU3VzaWVQeW1RRmRkUXFHRDhUMUIKU0dNWTNyVGxHQWZmckNob0UzWG9wRWcxUjJrODF3MTR1M1pxQ04vYVU4NXQ3aDk4bEduditEUlVuL1RxaGlYNgpyZHpXNnhKR1VBS2lkeWh1RXZNM3RvMTlFWlFVa0pITzJMak5EbmJNb3dzaWEyNUhkcGhKTC9QM1p4UEt3dXBlCk1adFA2cERhU1FURmtoQUcwZHVBazFLdWZ3TVBBTU83VWFZSGxOcUN0VWF4ZXZJUE1HV1hONXkvSUxYVEFIdHYKdm9SaXNxa0NnWUVBdzBNRmRjYjZ1Z2c1VmlSU2FFd21VUW5nM1hrTkRndkpSQlRReVNUVnEwbmk3ZnlDR1I5MwpWd0VHTEVwR29YZk1hdkJmMUVYd0ZjSTBhQ1dCL3FmTkVERXJucFl0S0t4R2FqSEhISnlsbFN3US9ndHhmcmkyClhvRXh1WkczbWF1UWpxa215ZHhLVnp0T1hJbE9qWlN6dWtQTnRxWEhKWXJpQStsa3Z3dkxlaXNDZ1lFQXRIVEMKRXBRVUVnV3hQUE9LN3ZmWE8vSkdEQzBrN2RtTUZOTktqakJITTlJNk1rSHRlR2JxK2IrTUNJN2F5YlZZcWFRMAozQ1RHeWE5cUxrNy9JRWtJQ0Zvai9CSHBSb1ZGTFRUVGwvUVladlV0Q2FlUkRzWVRncGMvYytiSkYvNy9zajA4CjdydUtpeVJNRXVSOGxXbENEQjRjNU95dW43WXJrSCs4NkhNVlA2VUNnWUJqVU5XWUlFc3JFRC9KbHRQcmhNQUEKZkJ2SnltWmZmSk0wYzduMmRTdlE0ZFh3NG54eHR0V0doVmpVY2pzV3FjNXBualcveklyZkpsWnRtcFpTSnB0Zwozd0dtdWcvaUhpMzZtYk1DMUpKTUc0dlJDNVVBdFliYzdxMlNDNUh0TVp4blU1WU5HbVVkbFdhNEhvYTc4S1N4CjJ3YnBIY3o3UlhiTU1vd3h1QmdZM1FLQmdRQ2hjaWhEU05uZitkbkU1enJ3YXluVUJ3QW1XcWxFWnJLTjJ5OUQKb092QzhCMkM0enJhMG5EOU9pTEZjVkZLendUZzJQazF6MjFOK2JNc2RSNkp1dTBGMCtlSDJGcDA3anlpb2pXQQpLREZBdzY4a2lSY2RPWmN3NmJJcU5sckpMaW1EUkloa0tjTmNrdi9BazB6bXU0SU1wMUJBZTRRTGZZYmlRM1kyCkhPZnd4UUtCZ0FQTnFXVWdkcWh5OVphM2ZtbHA0STN4ajExb2xuWUIvVDQ0cWZJNmhmejZtM0M0Z0dXckFrQloKKzFLOTJTUEdUNDZGVU1GYzJ0S3YvWklsRG5kVmNRV3BqL0ZVZ1VLekcrUHBrdGZweSt1QWRlNkE5ZmZWUXNxOQpCdWFzOHV2Y3R0QlhvbFIrR2ZJNkduMXNzcVQxZWpqR1RLNllhWTUwOGxpT2xyQ1dCN2h1Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0t")
	MalformattedRSAPrvtKeyEncoded = []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBaWFRdVBEOFNKWitrdzVORjBNbGYzRnZLZmVYeEFpWFVGblVScC8rQ2dOUE95KzJ2CnplRHRoTFA1enBacms4T1Y1R3BYNy8vL0U0NDhBVFNvNWxWb1p6MnBkdnBIUEFNR3RmMDZnZFgzQjc5RkN6dWkKZHhYejBrR2RyMGJYMU96eTNPS1dDbUtTQiZBOUR0T1I0TWUrSGcyK25lTEtnQmY3ck5XVVBjLzlDd1lrYkl0NgpCbU1qaEhrMUlqczRlOFlVRjgwOVdZZEpsTjhjMEVyS0oxYjV2Q3FzMkphL3hmUkV2ZU5VNlBzYWRiZjJpT0tOClZyOGlwYTVZRkNmSE03T29GQVJZYUlXQjRxNmNSYXZmNWZqZmhQVFY3a2M2dHdYNWxpTFJzTTRrV2UyTWFUUTAKc25HQ2F1V0EwMyt3aStGSlhZY1VBcVpTVExqMHBKMlVBNzFTdHdJREFRQUJBb0lCQUU3UmxxeG5UYVAvNUdFZQpmN2RNNmJrTlUwcC9GMkVzZW1RVnkvT1JMSkZMT1R1c002VklyWnIxV1JMRkxudGlYLzU2S3p0RE5EVmxtTlR6CjY5aGloalBBcXI5NEdMeXoydTd5UU1QQzNBQXl0bjMxTzFiSVdtUkhzTjJEU3VzaWVQeW1RRmRkUXFHRDhUMUIKU0dNWTNyVGxHQWZmckNob0UzWG9wRWcxUjJrODF3MTR1M1pxQ04vYVU4NXQ3aDk4bEduditEUlVuL1RxaGlYNgpyZHpXNnhKR1VBS2lkeWh1RXZNM3RvMTlFWlFVa0pITzJMak5EbmJNb3dzaWEyNUhkcGhKTC9QM1p4UEt3dXBlCk1adFA2cERhU1FURmtoQUcwZHVBazFLdWZ3TVBBTU83VWFZSGxOcUN0VWF4ZXZJUE1HV1hONXkvSUxYVEFIdHYKdm9SaXNxa0NnWUVBdzBNRmRjYjZ1Z2c1VmlSU2FFd21VUW5nM1hrTkRndkpSQlRReVNUVnEwbmk3ZnlDR1I5MwpWd0VHTEVwR29YZk1hdkJmMUVYd0ZjSTBhQ1dCL3FmTkVERXJucFl0S0t4R2FqSEhISnlsbFN3US9ndHhmcmkyClhvRXh1WkczbWF1UWpxa215ZHhLVnp0T1hJbE9qWlN6dWtQTnRxWEhKWXJpQStsa3Z3dkxlaXNDZ1lFQXRIVEMKRXBRVUVnV3hQUE9LN3ZmWE8vSkdEQzBrN2RtTUZOTktqakJITTlJNk1rSHRlR2JxK2IrTUNJN2F5YlZZcWFRMAozQ1RHeWE5cUxrNy9JRWtJQ0Zvai9CSHBSb1ZGTFRUVGwvUVladlV0Q2FlUkRzWVRncGMvYytiSkYvNy9zajA4CjdydUtpeVJNRXVSOGxXbENEQjRjNU95dW43WXJrSCs4NkhNVlA2VUNnWUJqVU5XWUlFc3JFRC9KbHRQcmhNQUEKZkJ2SnltWmZmSk0wYzduMmRTdlE0ZFh3NG54eHR0V0doVmpVY2pzV3FjNXBualcveklyZkpsWnRtcFpTSnB0Zwozd0dtdWcvaUhpMzZtYk1DMUpKTUc0dlJDNVVBdFliYzdxMlNDNUh0TVp4blU1WU5HbVVkbFdhNEhvYTc4S1N4CjJ3YnBIY3o3UlhiTU1vd3h1QmdZM1FLQmdRQ2hjaWhEU05uZitkbkU1enJ3YXluVUJ3QW1XcWxFWnJLTjJ5OUQKb092QzhCMkM0enJhMG5EOU9pTEZjVkZLendUZzJQazF6MjFOK2JNc2RSNkp1dTBGMCtlSDJGcDA3anlpb2pXQQpLREZBdzY4a2lSY2RPWmN3NmJJcU5sckpMaW1EUkloa0tjTmNrdi9BazB6bXU0SU1wMUJBZTRRTGZZYmlRM1kyCkhPZnd4UUtCZ0FQTnFXVWdkcWh5OVphM2ZtbHA0STN4ajExb2xuWUIvVDQ0cWZJNmhmejZtM0M0Z0dXckFrQloKKzFLOTJTUEdUNDZGVU1GYzJ0S3YvWklsRG5kVmNRV3BqL0ZVZ1VLekcrUHBrdGZweSt1QWRlNkE5ZmZWUXNxOQpCdWFzOHV2Y3R0QlhvbFIrR2ZJNkduMXNzcVQxZWpqR1RLNllhWTUwOGxpT2xyQ1dCN2h1Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0t")
)
