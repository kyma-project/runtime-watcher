package signature_test

import (
	"bytes"
	"net/http"
	"testing"

)

func TestAddDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName       string
		r              func() *http.Request
		expectedDigest string
		expectError    bool
	}{
		{
			testName: "Add sha256 digest",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			expectedDigest: "OVSxhOsmCNoNR95oI/Hb4b5fioAHnr7TT47SUq4Qjig=",
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
		test := test
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			req := test.r()
			err := signature.AddDigest(req)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			d := req.Header.Get("Digest")
			require.EqualValues(t, test.expectedDigest, d)
		})
	}
}

func TestVerifyDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		r           func() *http.Request
		expectError bool
	}{
		{
			name: "Verify sha256 digest",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				r.Header.Set("Digest", "OVSxhOsmCNoNR95oI/Hb4b5fioAHnr7TT47SUq4Qjig=")
				return r
			},
			expectError: false,
		},
		{
			name: "No digest header set",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				return r
			},
			expectError: true,
		},
		{
			name: "Digest cannot be verified",
			r: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(createPostBody(t)))
				r.Header.Set("Digest", "OVSxhOsmCNoNR95oI/IuBa7abfoaAadnja8jlasn=")
				return r
			},
			expectError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			req := test.r()
			err := signature.VerifyDigest(req)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
