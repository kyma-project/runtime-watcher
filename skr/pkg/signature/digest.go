package signature

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

// AddDigest add a digest to the given request (RFX 3230). The given body can be nil.
// The Digest is used to verify that the request body is not changed while it is being transmitted.
func AddDigest(request *http.Request) error {
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	request.Body = io.NopCloser(bytes.NewBuffer(data))
	if _, set := request.Header[DigestHeader]; set {
		return fmt.Errorf("digest already set in headers")
	}
	h := crypto.SHA256.New()
	h.Write(data)
	sum := h.Sum(nil)
	request.Header.Add(DigestHeader,
		base64.StdEncoding.EncodeToString(sum))
	return nil
}

// VerifyDigest verifies the given request by calculating a new digest using the body of the request
// and comparing it to the digest given in the header of the request.
func VerifyDigest(request *http.Request) error {
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	request.Body = io.NopCloser(bytes.NewBuffer(data))
	digest := request.Header.Get(DigestHeader)
	if len(digest) == 0 {
		return fmt.Errorf("request does not contain Digest header")
	}
	h := crypto.SHA256.New()
	h.Write(data)
	sum := h.Sum(nil)
	encSum := base64.StdEncoding.EncodeToString(sum)
	if encSum != digest {
		return fmt.Errorf("invalid digest: The header digest does not match the digest of the request body")
	}
	return nil
}
