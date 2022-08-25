package internal_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/kyma-project/kyma-watcher/webhook/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv       *envtest.Environment //nolint:gochecknoglobals
	k8sClient     client.Client        //nolint:gochecknoglobals
	kcpTestServer *http.Server         //nolint:gochecknoglobals
)

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	configMap := v1.ConfigMap{}
	kyma := unstructured.Unstructured{}
	BeforeAll(func() {
		// config map
		configMapContent, err := os.Open("./assets/configmap.yaml")
		Expect(err).ShouldNot(HaveOccurred())

		if configMapContent != nil {
			err = yaml.NewYAMLOrJSONDecoder(configMapContent, 2048).Decode(&configMap)
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

		//set KCP env vars
		err = os.Setenv("KCP_IP", "localhost")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Setenv("KCP_PORT", "9080")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Setenv("KCP_CONTRACT", "v1")
		Expect(err).ShouldNot(HaveOccurred())

		kcpTestHandler := http.NewServeMux()
		kcpTestHandler.HandleFunc("/v1/manifest/event", func(w http.ResponseWriter, r *http.Request) {
			reqBytes, err := io.ReadAll(r.Body)
			Expect(err).ShouldNot(HaveOccurred())
			watcherEvt := &types.WatcherEvent{}
			Expect(json.Unmarshal(reqBytes, watcherEvt)).To(Succeed())
			Expect(watcherEvt.KymaCr).To(Equal("kyma-sample"))
			Expect(watcherEvt.Name).To(Equal("crName"))
			Expect(watcherEvt.Namespace).To(Equal(metav1.NamespaceDefault))
		})
		kcpTestServer = &http.Server{
			Addr:    ":9080",
			Handler: kcpTestHandler,
		}
		//start KCP server
		go func() {
			kcpTestServer.ListenAndServe()
		}()

	})

	AfterAll(func() {
		os.Clearenv()
		//shutdown KCP server
		Expect(kcpTestServer.Shutdown(context.Background())).Should(Succeed())

		Expect(k8sClient.Delete(ctx, &configMap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &kyma)).Should(Succeed())
	})

	It("when relevant fields are modified", func() {
		handler := &internal.Handler{
			Client: k8sClient,
			Logger: ctrl.Log.WithName("skr-watcher-test"),
		}

		request, err := MockApiServerHTTPRequest(admissionv1.Create, "crName", "manifest", metav1.GroupVersionKind{
			Kind:    "Manifest",
			Version: "v1alpha1",
			Group:   "component.kyma-project.io",
		})
		Expect(err).ShouldNot(HaveOccurred())
		recorder := httptest.NewRecorder()
		handler.Handle(recorder, request)

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
