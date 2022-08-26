package internal_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/kyma-project/runtime-watcher/skr/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv   *envtest.Environment //nolint:gochecknoglobals
	k8sClient client.Client        //nolint:gochecknoglobals
)

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	configMap := v1.ConfigMap{}
	kyma := unstructured.Unstructured{}
	BeforeAll(func() {
		// config map
		configMapContent, err := os.Open("./assets/configmap.yaml")
		Expect(err).ShouldNot(HaveOccurred())

		if configMapContent != nil {
			err = yaml.NewYAMLOrJSONDecoder(configMapContent, DefaultBufferSize).Decode(&configMap)
			Expect(err).ShouldNot(HaveOccurred())
			err = configMapContent.Close()
			Expect(err).ShouldNot(HaveOccurred())
		}

		Expect(k8sClient.Create(ctx, &configMap)).Should(Succeed())

		// base kyma resource
		response, err := http.DefaultClient.Get(
			"https://raw.githubusercontent.com/kyma-project/kyma-operator/main/operator/config/samples/" +
				"component-integration-installed/operator_v1alpha1_kyma.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		body, err := io.ReadAll(response.Body)
		Expect(err).ShouldNot(HaveOccurred())

		_, _, err = internal.UniversalDeserializer.Decode(body,
			nil, &kyma)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(k8sClient.Create(ctx, &kyma)).Should(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &configMap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &kyma)).Should(Succeed())
	})

	It("when relevant fields are modified", func() {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}

		request, err := MockAPIServerHTTPRequest(admissionv1.Create, "crName", "manifest", metav1.GroupVersionKind{
			Kind:    "Manifest",
			Version: "v1alpha1",
			Group:   "component.kyma-project.io",
		})
		Expect(err).ShouldNot(HaveOccurred())
		recorder := httptest.NewRecorder()
		handler.Handle(recorder, request)

		kcpPayload, err := io.ReadAll(kcpResponseRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		watcherEvt := &types.WatcherEvent{}
		Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
		Expect(watcherEvt.KymaCr).To(Equal("kyma-sample"))
		Expect(watcherEvt.Name).To(Equal("crName"))
		Expect(watcherEvt.Namespace).To(Equal(metav1.NamespaceDefault))

		admissionReview := admissionv1.AdmissionReview{}
		bytes, err := io.ReadAll(recorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = internal.UniversalDeserializer.Decode(bytes, nil, &admissionReview)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(admissionReview.Response.Allowed).Should(BeTrue())
		Expect(admissionReview.Response.Result.Message).To(Equal("kcp request succeeded"))
		Expect(admissionReview.Response.Result.Status).To(Equal(metav1.StatusSuccess))
	})
})

type testCase struct {
	// description string
	params  testCaseParams
	results testCaseResults
}

type testCaseResults struct {
	// kymaCRName      string
	// objectName      string
	// objectNamespace string
	// allowed         bool
	resultMsg    string
	resultStatus string
}
type testCaseParams struct {
	// handler    *internal.Handler
	operation  admissionv1.Operation
	crName     string
	moduleName string
	crGVK      metav1.GroupVersionKind
}

//nolint:gochecknoglobals
var kymaCREntries = []TableEntry{
	Entry("kyma CR CREATE event", &testCase{
		params: testCaseParams{
			operation:  admissionv1.Create,
			crName:     "kyma-1",
			moduleName: "kyma",
			crGVK: metav1.GroupVersionKind{
				Kind:    "Kyma",
				Version: "v1alpha1",
				Group:   "operator.kyma-project.io",
			},
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("kyma CR DELETE event", &testCase{
		params: testCaseParams{
			operation:  admissionv1.Delete,
			crName:     "kyma-1",
			moduleName: "kyma",
			crGVK: metav1.GroupVersionKind{
				Kind:    "Kyma",
				Version: "v1alpha1",
				Group:   "operator.kyma-project.io",
			},
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("kyma CR UPDATE event", &testCase{
		params: testCaseParams{
			operation:  admissionv1.Update,
			crName:     "kyma-1",
			moduleName: "kyma",
			crGVK: metav1.GroupVersionKind{
				Kind:    "Kyma",
				Version: "v1alpha1",
				Group:   "operator.kyma-project.io",
			},
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
}

var _ = Context("kyma CR scenarios", Ordered, func() {
	configMap := v1.ConfigMap{}
	BeforeAll(func() {
		configMapContent, err := os.Open("./assets/configmap.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		if configMapContent != nil {
			err = yaml.NewYAMLOrJSONDecoder(configMapContent, DefaultBufferSize).Decode(&configMap)
			Expect(err).ShouldNot(HaveOccurred())
			err = configMapContent.Close()
			Expect(err).ShouldNot(HaveOccurred())
		}
		Expect(k8sClient.Create(ctx, &configMap)).Should(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &configMap)).Should(Succeed())
	})
	DescribeTable("should validate admission request and send correct payload to KCP", func(testCase *testCase) {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}
		request, err := MockAPIServerHTTPRequest(testCase.params.operation, testCase.params.crName,
			testCase.params.moduleName, testCase.params.crGVK)
		Expect(err).ShouldNot(HaveOccurred())
		recorder := httptest.NewRecorder()
		handler.Handle(recorder, request)

		// verify KCP payload
		kcpPayload, err := io.ReadAll(kcpResponseRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		watcherEvt := &types.WatcherEvent{}
		Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
		Expect(watcherEvt.KymaCr).To(Equal(testCase.params.crName))
		Expect(watcherEvt.Name).To(Equal(testCase.params.crName))
		Expect(watcherEvt.Namespace).To(Equal(metav1.NamespaceDefault))

		admissionReview := admissionv1.AdmissionReview{}
		bytes, err := io.ReadAll(recorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = internal.UniversalDeserializer.Decode(bytes, nil, &admissionReview)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(admissionReview.Response.Allowed).To(BeTrue())
		Expect(admissionReview.Response.Result.Message).To(Equal(testCase.results.resultMsg))
		Expect(admissionReview.Response.Result.Status).To(Equal(testCase.results.resultStatus))
	}, kymaCREntries)
})
