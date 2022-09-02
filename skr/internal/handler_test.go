package internal_test

import (
	"encoding/json"
	"github.com/kyma-project/runtime-watcher/skr/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net/http"
	"net/http/httptest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type testCase struct {
	params  testCaseParams
	results testCaseResults
}

type testCaseResults struct {
	resultMsg    string
	resultStatus string
}
type testCaseParams struct {
	operation   admissionv1.Operation
	ownerName   string
	watchedName string
	moduleName  string
	subResource bool
}

//nolint:gochecknoglobals
var kymaCREntries = []TableEntry{
	Entry("watched resource CREATE event", &testCase{
		params: testCaseParams{
			operation:   admissionv1.Create,
			watchedName: crName1,
			moduleName:  moduleName,
			ownerName:   ownerName,
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("watched resource DELETE event", &testCase{
		params: testCaseParams{
			operation:   admissionv1.Delete,
			watchedName: crName1,
			moduleName:  moduleName,
			ownerName:   ownerName,
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("watched resource UPDATE event", &testCase{
		params: testCaseParams{
			operation:   admissionv1.Update,
			watchedName: crName1,
			moduleName:  moduleName,
			ownerName:   ownerName,
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("watched resource CREATE event on subresource", &testCase{
		params: testCaseParams{
			operation:   admissionv1.Create,
			watchedName: crName1,
			moduleName:  moduleName,
			ownerName:   ownerName,
			subResource: true,
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("watched resource DELETE event on subresource", &testCase{
		params: testCaseParams{
			operation:   admissionv1.Delete,
			watchedName: crName1,
			moduleName:  moduleName,
			ownerName:   ownerName,
			subResource: true,
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
	Entry("watched resource UPDATE event on subresource", &testCase{
		params: testCaseParams{
			operation:   admissionv1.Update,
			watchedName: crName1,
			moduleName:  moduleName,
			ownerName:   ownerName,
			subResource: true,
		},
		results: testCaseResults{
			resultMsg:    internal.KcpReqSucceededMsg,
			resultStatus: metav1.StatusSuccess,
		},
	}),
}

var _ = Describe("given watched resource", Ordered, func() {
	DescribeTable("should validate admission request and send correct payload to KCP", func(testCase *testCase) {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}
		request, err := getAdmissionHTTPRequest(testCase.params.operation, testCase.params.watchedName,
			testCase.params.moduleName, ownerLabels, testCase.params.subResource)
		Expect(err).ShouldNot(HaveOccurred())
		skrRecorder := httptest.NewRecorder()
		handler.Handle(skrRecorder, request)

		// check listener event
		Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
		kcpPayload, err := io.ReadAll(kcpRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		watcherEvt := &internal.WatchEvent{}
		Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
		Expect(watcherEvt).To(Equal(
			&internal.WatchEvent{
				Watched: ctrlClient.ObjectKey{Name: testCase.params.watchedName, Namespace: metav1.NamespaceDefault},
				Owner:   ctrlClient.ObjectKey{Name: ownerName, Namespace: metav1.NamespaceDefault},
				WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(watchedResourceAPIVersion,
					watchedResourceKind)),
			},
		))

		// check admission review response
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
