package signature

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

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
