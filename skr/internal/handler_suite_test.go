package internal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook handlers suite")
}

var (
	ctx                 context.Context                           //nolint:gochecknoglobals
	cancel              context.CancelFunc                        //nolint:gochecknoglobals
	kcpModulesList      = []string{"kyma", "manifest", "compass"} //nolint:gochecknoglobals
	kcpResponseRecorder *httptest.ResponseRecorder                //nolint:gochecknoglobals
	kcpMockServer       *http.Server                              //nolint:gochecknoglobals
)

const DefaultBufferSize = 2048

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment for skr-watcher tests")

	kymaCrd := &v1.CustomResourceDefinition{}
	res, err := http.DefaultClient.Get(
		"https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/operator/" +
			"config/crd/bases/operator.kyma-project.io_kymas.yaml")
	Expect(err).NotTo(HaveOccurred())
	Expect(res.StatusCode).To(BeEquivalentTo(http.StatusOK))
	Expect(yaml.NewYAMLOrJSONDecoder(res.Body, DefaultBufferSize).Decode(kymaCrd)).To(Succeed())

	testEnv = &envtest.Environment{
		CRDs:                  []*v1.CustomResourceDefinition{kymaCrd},
		ErrorIfCRDPathMissing: false,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// set env variables
	Expect(os.Setenv("WEBHOOK_SIDE_CAR", "false")).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	// set KCP env vars
	err = os.Setenv("KCP_IP", "localhost")
	Expect(err).ShouldNot(HaveOccurred())
	err = os.Setenv("KCP_PORT", "10080")
	Expect(err).ShouldNot(HaveOccurred())
	err = os.Setenv("KCP_CONTRACT", "v1")
	Expect(err).ShouldNot(HaveOccurred())

	kcpTestHandler := BootStrapKcpMockHandlers()
	kcpResponseRecorder = kcpTestHandler.Recorder

	kcpMockServer = &http.Server{
		Addr:    ":10080",
		Handler: kcpTestHandler,
	}

	go func() {
		_ = kcpMockServer.ListenAndServe()
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())

	// clear env variables
	os.Clearenv()

	Expect(kcpMockServer.Shutdown(context.Background())).Should(Succeed())

	cancel()
})
