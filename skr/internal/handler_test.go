package internal_test

import (
	"encoding/json"
	"github.com/kyma-project/runtime-watcher/skr/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"net/http"
	"net/http/httptest"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv   *envtest.Environment //nolint:gochecknoglobals
	k8sClient client.Client        //nolint:gochecknoglobals
)

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	kyma := unstructured.Unstructured{}
	BeforeAll(func() {
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
		Expect(k8sClient.Delete(ctx, &kyma)).Should(Succeed())
	})

	It("when relevant fields are modified", func() {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}

		admissionHTTPrequest, err := getAdmissionHTTPRequest(admissionv1.Create, "crName", "manifest",
			ownerLabels)
		Expect(err).ShouldNot(HaveOccurred())
		skrRecorder := httptest.NewRecorder()
		handler.Handle(skrRecorder, admissionHTTPrequest)

		Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
		kcpPayload, err := io.ReadAll(kcpRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		watcherEvt := &internal.WatchEvent{}
		Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
		Expect(watcherEvt.Owner).To(Equal("kyma-sample"))
		Expect(watcherEvt.Watched).To(Equal("crName"))
		Expect(watcherEvt.WatchedGvk).To(Equal(metav1.NamespaceDefault))

		admissionReview := admissionv1.AdmissionReview{}
		bytes, err := io.ReadAll(skrRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = internal.UniversalDeserializer.Decode(bytes, nil, &admissionReview)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(admissionReview.Response.Allowed).Should(BeTrue())
		Expect(admissionReview.Response.Result.Message).To(Equal("kcp request succeeded"))
		Expect(admissionReview.Response.Result.Status).To(Equal(metav1.StatusSuccess))
	})
})

type testCase struct {
	params  testCaseParams
	results testCaseResults
}

type testCaseResults struct {
	resultMsg    string
	resultStatus string
}
type testCaseParams struct {
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
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
}

var _ = Describe("kyma CR scenarios", Ordered, func() {
	BeforeAll(func() {
	})

	AfterAll(func() {
	})
	DescribeTable("should validate admission request and send correct payload to KCP", func(testCase *testCase) {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}
		request, err := getAdmissionHTTPRequest(testCase.params.operation, testCase.params.crName,
			testCase.params.moduleName, ownerLabels)
		Expect(err).ShouldNot(HaveOccurred())
		skrRecorder := httptest.NewRecorder()
		handler.Handle(skrRecorder, request)

		// verify KCP payload
		Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
		kcpPayload, err := io.ReadAll(kcpRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		watcherEvt := &internal.WatchEvent{}
		Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
		Expect(watcherEvt.Owner).To(Equal(testCase.params.crName))
		Expect(watcherEvt.Watched).To(Equal(testCase.params.crName))
		Expect(watcherEvt.WatchedGvk).To(Equal(metav1.NamespaceDefault))

		admissionReview := admissionv1.AdmissionReview{}
		bytes, err := io.ReadAll(skrRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = internal.UniversalDeserializer.Decode(bytes, nil, &admissionReview)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(admissionReview.Response.Allowed).To(BeTrue())
		Expect(admissionReview.Response.Result.Message).To(Equal(testCase.results.resultMsg))
		Expect(admissionReview.Response.Result.Status).To(Equal(testCase.results.resultStatus))
	}, kymaCREntries)
})
