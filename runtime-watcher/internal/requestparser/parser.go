package requestparser

import (
	"errors"
	"io"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RequestParser struct {
	deserializer runtime.Decoder
}

func NewRequestParser(decoder runtime.Decoder) *RequestParser {
	return &RequestParser{deserializer: decoder}
}

var (
	errReadRequestBody             = errors.New("io failed to read bytes from request body")
	errDeserializeRequestBody      = errors.New("serializer failed to decode admission review body")
	errEmptyAdmissionReviewRequest = errors.New("admission request was empty")
)

func (rp *RequestParser) ParseAdmissionReview(request *http.Request) (*admissionv1.AdmissionReview, error) {
	defer request.Body.Close()

	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, errors.Join(errReadRequestBody, err)
	}

	admissionReview := &admissionv1.AdmissionReview{}
	if _, _, err = rp.deserializer.Decode(bodyBytes, nil, admissionReview); err != nil {
		return nil, errors.Join(errDeserializeRequestBody, err)
	}
	if admissionReview.Request == nil {
		return nil, errors.Join(errEmptyAdmissionReviewRequest, err)
	}
	return admissionReview, nil
}
