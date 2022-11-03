package signature_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/kyma-project/runtime-watcher/skr/pkg/signature"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVerifyRequest(t *testing.T) { //nolint:funlen
	t.Parallel()
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
				return createRequest(t, rsaPrvtKeyEncoded)
			},
			k8sclient:    createValidK8sClient(t),
			verifySucces: true,
			expectError:  false,
		},
		{
			testName: "Request was signed with wrong private key",
			r: func() *http.Request {
				return createRequest(t, rsaPrvtKey2Encoded)
			},
			k8sclient:    createValidK8sClient(t),
			verifySucces: false,
			expectError:  true,
		},
		{
			testName: "Request Body was altered during transfer",
			r: func() *http.Request {
				request, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				require.NoError(t, signature.SignRequest(request, types.NamespacedName{
					Namespace: "default",
					Name:      "kyma-1",
				}, client.Client(fake.NewClientBuilder().WithObjects(
					&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
						Data: map[string][]byte{
							signature.PrvtKeyKey:         rsaPrvtKeyEncoded,
							signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
							signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
						},
					},
				).Build())))
				request.Body = io.NopCloser(bytes.NewBuffer([]byte("New Random data")))
				return request
			},
			k8sclient:    createValidK8sClient(t),
			verifySucces: false,
			expectError:  true,
		},
		{
			testName: "Malformatted Public Key",
			r: func() *http.Request {
				return createRequest(t, rsaPrvtKeyEncoded)
			},
			k8sclient: client.Client(fake.NewClientBuilder().WithObjects(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
					Data: map[string][]byte{
						signature.PubKeyKey: malformattedRSAPubKeyEncoded,
					},
				},
			).Build()),
			verifySucces: false,
			expectError:  true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()
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

func createRequest(t *testing.T, prvtKeyEncoded []byte) *http.Request {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
	require.NoError(t, signature.SignRequest(request, types.NamespacedName{
		Namespace: "default",
		Name:      "kyma-1",
	}, client.Client(fake.NewClientBuilder().WithObjects(
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
			Data: map[string][]byte{
				signature.PrvtKeyKey:         prvtKeyEncoded,
				signature.PubKeyNamespaceKey: []byte("ZGVmYXVsdA=="), // "default"
				signature.PubKeyNameKey:      []byte("a3ltYS0x"),     // "kyma-1"
			},
		},
	).Build())))
	return request
}

func createValidK8sClient(t *testing.T) client.Client { //nolint:ireturn
	t.Helper()
	return client.Client(fake.NewClientBuilder().WithObjects(
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "kyma-1", Namespace: "default"},
			Data: map[string][]byte{
				signature.PubKeyKey: rsaPubKeyEncoded,
			},
		},
	).Build())
}
