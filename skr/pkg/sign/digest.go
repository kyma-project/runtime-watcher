package sign

import (
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/http"
)

func AddDigest(r *http.Request) error {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	_, set := r.Header[digestHeader]
	if set {
		return fmt.Errorf("digest already set in headers")

	}
	var h = crypto.SHA256.New()
	h.Write(bytes)
	sum := h.Sum(nil)
	r.Header.Add(digestHeader,
		base64.StdEncoding.EncodeToString(sum[:]))
	return nil
}

func VerifyDigest(r *http.Request) error {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	digest := r.Header.Get(digestHeader)
	if len(digest) == 0 {
		return fmt.Errorf("request does not contain Digest header")
	}
	var h hash.Hash
	h = crypto.SHA256.New()
	h.Write(bytes)
	sum := h.Sum(nil)
	encSum := base64.StdEncoding.EncodeToString(sum[:])
	if encSum != digest {
		return fmt.Errorf("invalid digest: The header digest does not match the digest of the request body")
	}
	return nil
}
