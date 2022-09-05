package internal_test

import (
	"encoding/json"
	"fmt"
	"github.com/kyma-project/runtime-watcher/skr/internal"
	util "github.com/kyma-project/runtime-watcher/skr/internal/test_util"
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
	operation     admissionv1.Operation
	ownerName     string
	watchedName   string
	moduleName    string
	changeObjType util.ChangeObj
}

//nolint:gochecknoglobals
var baseTestCase = testCase{
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
}

func createTableEntries() []TableEntry {
	tableEntries := make([]TableEntry, 0)
	// add all operations
	for _, operationToTest := range util.OperationsToTest {
		// add all changed object types
		for _, changeType := range util.ChangeObjTypes {
			currentTestCase := baseTestCase
			currentTestCase.params.operation = operationToTest
			currentTestCase.params.changeObjType = changeType

			if changeType == util.NoChange && operationToTest == admissionv1.Update {
				currentTestCase.results.resultMsg = fmt.Sprintf("no change detected on watched resource %s/%s",
					metav1.NamespaceDefault, crName1)
			} else if operationToTest == admissionv1.Connect {
				currentTestCase.results.resultMsg = fmt.Sprintf("operation %s not supported for resource %s",
					admissionv1.Connect, metav1.GroupVersionKind(schema.FromAPIVersionAndKind(
						util.WatchedResourceAPIVersion, util.WatchedResourceKind)))
			}

			description := fmt.Sprintf("when %s operation is triggered on watched resource with %s change",
				operationToTest, changeType)
			tableEntries = append(tableEntries, Entry(description, &currentTestCase))
		}
	}
	return tableEntries
}

var _ = Describe("given watched resource", Ordered, func() {
	BeforeEach(func() {
		kcpRecorder.Flush()
	})
	DescribeTable("should validate admission request and send correct payload to KCP", func(testCase *testCase) {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}
		request, err := util.GetAdmissionHTTPRequest(testCase.params.operation, testCase.params.watchedName,
			testCase.params.moduleName, ownerLabels, testCase.params.changeObjType)
		Expect(err).ShouldNot(HaveOccurred())
		skrRecorder := httptest.NewRecorder()
		handler.Handle(skrRecorder, request)

		// check admission review response
		admissionReview := admissionv1.AdmissionReview{}
		bytes, err := io.ReadAll(skrRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = internal.UniversalDeserializer.Decode(bytes, nil, &admissionReview)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(admissionReview.Response.Allowed).To(BeTrue())
		Expect(admissionReview.Response.Result.Message).To(Equal(testCase.results.resultMsg))
		Expect(admissionReview.Response.Result.Status).To(Equal(testCase.results.resultStatus))

		// check listener event
		Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
		kcpPayload, err := io.ReadAll(kcpRecorder.Body)
		if (testCase.params.changeObjType == util.NoChange && testCase.params.operation == admissionv1.Update) ||
			testCase.params.operation == admissionv1.Connect {
			Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
			// no request was sent to KCP
			Expect(len(kcpPayload)).To(Equal(0))
		} else {
			Expect(err).ShouldNot(HaveOccurred())
			watcherEvt := &internal.WatchEvent{}
			Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
			Expect(watcherEvt).To(Equal(
				&internal.WatchEvent{
					Watched: ctrlClient.ObjectKey{Name: testCase.params.watchedName, Namespace: metav1.NamespaceDefault},
					Owner:   ctrlClient.ObjectKey{Name: ownerName, Namespace: metav1.NamespaceDefault},
					WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(util.WatchedResourceAPIVersion,
						util.WatchedResourceKind)),
				},
			))
		}
	}, createTableEntries())
})
