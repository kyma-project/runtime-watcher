package signature

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/http"
)

func AddDigest(r *http.Request) error {
	req := r
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(data))
	_, set := r.Header[DigestHeader]
	if set {
		return fmt.Errorf("digest already set in headers")

	}
	h := crypto.SHA256.New()
	h.Write(data)
	sum := h.Sum(nil)
	r.Header.Add(DigestHeader,
		base64.StdEncoding.EncodeToString(sum[:]))
	return nil
}

func VerifyDigest(r *http.Request) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(data))
	digest := r.Header.Get(DigestHeader)
	if len(digest) == 0 {
		return fmt.Errorf("request does not contain Digest header")
	}
	var h hash.Hash
	h = crypto.SHA256.New()
	h.Write(data)
	sum := h.Sum(nil)
	encSum := base64.StdEncoding.EncodeToString(sum[:])
	if encSum != digest {
		return fmt.Errorf("invalid digest: The header digest does not match the digest of the request body")
	}
	return nil
}
