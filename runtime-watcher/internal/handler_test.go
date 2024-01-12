package internal_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"

	"github.com/kyma-project/runtime-watcher/skr/internal/requestparser"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"github.com/kyma-project/runtime-watcher/skr/internal"
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
	changeObjType ChangeObj
}

const (
	crName    = "kyma-1"
	ownerName = "ownerName"
)

var baseTestCase = testCase{
	params: testCaseParams{
		operation:   admissionv1.Create,
		watchedName: crName,
		moduleName:  moduleName,
		ownerName:   ownerName,
	},
	results: testCaseResults{
		resultMsg:    "kcp request succeeded",
		resultStatus: metav1.StatusSuccess,
	},
}

func createTableEntries() []TableEntry {
	tableEntries := make([]TableEntry, 0)
	// add all operations
	for _, operationToTest := range operationsToTest {
		// add all changed object types
		for _, changeType := range changeObjTypes {
			currentTestCase := baseTestCase
			currentTestCase.params.operation = operationToTest
			currentTestCase.params.changeObjType = changeType

			if changeType == NoChange && operationToTest == admissionv1.Update {
				currentTestCase.results.resultMsg = fmt.Sprintf("no change detected on watched resource %s/%s",
					metav1.NamespaceDefault, crName)
			} else if operationToTest == admissionv1.Connect {
				currentTestCase.results.resultMsg = fmt.Sprintf("operation %s not supported for %s",
					admissionv1.Connect, metav1.GroupVersionKind(schema.FromAPIVersionAndKind(
						WatchedResourceAPIVersion, WatchedResourceKind)))
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
		logger := ctrl.Log.WithName("skr-watcher-test")
		config, err := serverconfig.ParseFromEnv(logger)
		Expect(err).ShouldNot(HaveOccurred())

		managedByLabel := map[string]string{"operator.kyma-project.io/managed-by": "lifecycle-manager"}
		namespacedName := fmt.Sprintf("%s/%s", metav1.NamespaceDefault, ownerName)
		ownedByAnnotation := map[string]string{"operator.kyma-project.io/owned-by": namespacedName}
		request, err := GetAdmissionHTTPRequest(testCase.params.operation, testCase.params.watchedName,
			testCase.params.moduleName, managedByLabel, ownedByAnnotation, testCase.params.changeObjType)
		Expect(err).ShouldNot(HaveOccurred())

		decoder := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
		requestParser := requestparser.NewRequestParser(decoder)
		metrics := watchermetrics.NewMetrics()
		handler := internal.NewHandler(k8sClient, logger, config, *requestParser, *metrics)
		skrRecorder := httptest.NewRecorder()
		handler.Handle(skrRecorder, request)

		bytes, err := io.ReadAll(skrRecorder.Body)
		Expect(err).ShouldNot(HaveOccurred())

		admissionReview := admissionv1.AdmissionReview{}
		_, _, err = decoder.Decode(bytes, nil, &admissionReview)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(admissionReview.Response.Allowed).To(BeTrue())
		Expect(admissionReview.Response.Result.Message).To(Equal(testCase.results.resultMsg))
		Expect(admissionReview.Response.Result.Status).To(Equal(testCase.results.resultStatus))

		// check listener event
		Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
		kcpPayload, err := io.ReadAll(kcpRecorder.Body)
		if (testCase.params.changeObjType == NoChange && testCase.params.operation == admissionv1.Update) ||
			testCase.params.operation == admissionv1.Connect {
			Expect(kcpRecorder.Code).To(BeEquivalentTo(http.StatusOK))
			// no request was sent to KCP
			Expect(kcpPayload).To(BeEmpty())
		} else {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(kcpPayload).NotTo(BeEmpty())
			watcherEvt := &listenerTypes.WatchEvent{}
			Expect(json.Unmarshal(kcpPayload, watcherEvt)).To(Succeed())
			Expect(watcherEvt).To(Equal(
				&listenerTypes.WatchEvent{
					Watched: ctrlClient.ObjectKey{Name: testCase.params.watchedName, Namespace: metav1.NamespaceDefault},
					Owner:   ctrlClient.ObjectKey{Name: ownerName, Namespace: metav1.NamespaceDefault},
					WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(WatchedResourceAPIVersion,
						WatchedResourceKind)),
				},
			))
		}
	}, createTableEntries())
})
