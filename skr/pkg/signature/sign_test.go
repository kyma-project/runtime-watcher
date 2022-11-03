package signature_test

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/kyma-project/runtime-watcher/skr/pkg/signature"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSignRequest(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		testName     string
		keyreference types.NamespacedName
		r            func() *http.Request
		k8sclient    client.Client
		expectError  bool
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
						signature.PrvtKeyKey:         rsaPrvtKeyEncoded,
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
						signature.PrvtKeyKey:         malformattedRSAPrvtKeyEncoded,
						signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
						signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
					},
				},
			).Build()),
			expectError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()
			req := test.r()

			err := signature.SignRequest(req, test.keyreference, test.k8sclient)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			sigHeader := req.Header.Get(signature.SignatureHeader)
			r, _ := regexp.Compile(fmt.Sprintf("pubKeySecretName=\"%s\",pubKeySecretNamespace=\"%s\","+
				"created=[0-9]*,Signature=\"*\"", test.keyreference.Name, test.keyreference.Namespace))
			require.True(t, r.Match([]byte(sigHeader)))

			digestHeader := req.Header.Get(signature.DigestHeader)
			require.NotEmpty(t, digestHeader)
		})
	}
}
