package signature_test

import (
	"bytes"
	"github.com/kyma-project/runtime-watcher/skr/pkg/signature"
	"github.com/stretchr/testify/require"
	"io"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestVerifyRequest(t *testing.T) {

	tests := []struct {
		testName     string
		r            func() *http.Request
		k8sclient    client.Client
		verifySucces bool
		expectError  bool
	}{
		{
			testName: "Verify signed given request with a RSA Public Key",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				require.NoError(t, signature.SignRequest(r, types.NamespacedName{
					Namespace: "default",
					Name:      "kyma-1",
				}, client.Client(fake.NewClientBuilder().WithObjects(
					&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
						Data: map[string][]byte{
							signature.PvtKeyKey:          RSAPrvtKeyEncoded,
							signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
							signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
						},
					},
				).Build())))
				return r
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PubKeyKey: RSAPubKeyEncoded,
					},
				},
			).Build()),
			verifySucces: true,
			expectError:  false,
		},
		{
			testName: "Request was signed with wrong private key",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				require.NoError(t, signature.SignRequest(r, types.NamespacedName{
					Namespace: "default",
					Name:      "kyma-1",
				}, client.Client(fake.NewClientBuilder().WithObjects(
					&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
						Data: map[string][]byte{
							signature.PvtKeyKey:          RSAPrvtKey2Encoded,
							signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
							signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
						},
					},
				).Build())))
				return r
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PubKeyKey: RSAPubKeyEncoded,
					},
				},
			).Build()),
			verifySucces: false,
			expectError:  true,
		},
		{
			testName: "Request Body was altered during transfer",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				require.NoError(t, signature.SignRequest(r, types.NamespacedName{
					Namespace: "default",
					Name:      "kyma-1",
				}, client.Client(fake.NewClientBuilder().WithObjects(
					&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
						Data: map[string][]byte{
							signature.PvtKeyKey:          RSAPrvtKeyEncoded,
							signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
							signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
						},
					},
				).Build())))
				r.Body = io.NopCloser(bytes.NewBuffer([]byte("New Random data")))
				return r
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PubKeyKey: RSAPubKeyEncoded,
					},
				},
			).Build()),
			verifySucces: false,
			expectError:  true,
		},
		{
			testName: "Malformatted Public Key",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				require.NoError(t, signature.SignRequest(r, types.NamespacedName{
					Namespace: "default",
					Name:      "kyma-1",
				}, client.Client(fake.NewClientBuilder().WithObjects(
					&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
						Data: map[string][]byte{
							signature.PvtKeyKey:          RSAPrvtKeyEncoded,
							signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
							signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
						},
					},
				).Build())))
				return r
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PubKeyKey: MalformattedRSAPubKeyEncoded,
					},
				},
			).Build()),
			verifySucces: false,
			expectError:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			test := test
			req := test.r()

			verified, err := signature.VerifyRequest(req, test.k8sclient)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.verifySucces {
				require.True(t, verified)
			} else {
				require.False(t, verified)
			}
		})
	}
}
