package internal_test

import (
	"bytes"
	"encoding/json"
	"github.com/kyma-project/kyma-watcher/skr/webhook/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"net/http"
	"net/http/httptest"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv   *envtest.Environment //nolint:gochecknoglobals
	k8sClient client.Client
)

const (
	uid = "uid"
)

func getHttpRequest(operation admissionv1.Operation, crdName string) (*http.Request, *httptest.ResponseRecorder) {
	admissionReview, err := createAdmissionRequest(operation, crdName)
	Expect(err).ShouldNot(HaveOccurred())
	bytesRequest, err := json.Marshal(admissionReview)
	Expect(err).ShouldNot(HaveOccurred())
	req := httptest.NewRequest(http.MethodGet, "/validate", bytes.NewBuffer(bytesRequest))
	w := httptest.NewRecorder()
	return req, w
}

func createAdmissionRequest(operation admissionv1.Operation, crdName string) (*admissionv1.AdmissionReview, error) {
	admissionReview := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       admissionv1.AdmissionReview{}.Kind,
			APIVersion: admissionv1.AdmissionReview{}.APIVersion,
		},
		Request: &admissionv1.AdmissionRequest{
			Name: crdName,
			Kind: metav1.GroupVersionKind{
				Kind:    "Manifest",
				Version: "v1alpha1",
				Group:   "component.kyma-project.io",
			},
			Operation: operation,
			UID:       uid,
		},
	}

	objectWatched := &internal.ObjectWatched{
		Metadata: internal.Metadata{
			Name:      "manifestObj",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: map[string]interface{}{
			"key": "value",
		},
		Kind: "Manifest",
	}

	rawBytes, err := json.Marshal(objectWatched)
	if err != nil {
		return nil, err
	}

	admissionReview.Request.OldObject.Raw = rawBytes
	admissionReview.Request.Object.Raw = rawBytes

	return admissionReview, nil
}

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {

	BeforeAll(func() {
		file, err := os.Open("./../configmap.yaml")
		Expect(err).ShouldNot(HaveOccurred())

		configMap := v1.ConfigMap{}
		if file != nil {
			if err = yaml.NewYAMLOrJSONDecoder(file, 2048).Decode(&configMap); err != nil {
				Expect(err).ShouldNot(HaveOccurred())
			}
			err = file.Close()
		}

		Expect(k8sClient.Create(ctx, &configMap)).Should(Succeed())

		file, err = os.Open("./../kyma.yaml")
		Expect(err).ShouldNot(HaveOccurred())

		kyma := unstructured.Unstructured{}
		if file != nil {
			if err = yaml.NewYAMLOrJSONDecoder(file, 2048).Decode(&kyma); err != nil {
				Expect(err).ShouldNot(HaveOccurred())
			}
			err = file.Close()
		}

		Expect(k8sClient.Create(ctx, &kyma)).Should(Succeed())
	})

	It("Should result in an error state", func() {
		// valid CAPApplication
		//Ca := createManifestResource()
		wh := &internal.Handler{
			Client: k8sClient,
		}

		request, recorder := getHttpRequest(admissionv1.Update, "crdName")

		wh.Handle(recorder, request)

		admissionReview := admissionv1.AdmissionReview{}
		bytes, err := io.ReadAll(recorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		internal.UniversalDeserializer.Decode(bytes, nil, &admissionReview)

	})

})
