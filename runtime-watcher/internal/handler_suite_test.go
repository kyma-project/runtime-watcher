//nolint:gochecknoglobals
package internal_test

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	cancel        context.CancelFunc
	kcpRecorder   *httptest.ResponseRecorder
	kcpMockServer *httptest.Server
	testEnv       *envtest.Environment
	k8sClient     client.Client
)

const moduleName = "kyma"

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	_, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment for skr-watcher tests")

	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: false,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	kcpTestHandler := BootStrapKcpMockHandlers(moduleName)
	kcpRecorder = kcpTestHandler.Recorder

	// start listener server
	kcpMockServer = httptest.NewServer(kcpTestHandler)

	// set KCP env vars
	err = os.Setenv("KCP_ADDR", kcpMockServer.Listener.Addr().String())
	Expect(err).ShouldNot(HaveOccurred())
	err = os.Setenv("KCP_CONTRACT", "v1")
	Expect(err).ShouldNot(HaveOccurred())

	_ = os.Setenv("CA_CERT", "tmp")
	_ = os.Setenv("TLS_CERT", "tmp")
	_ = os.Setenv("TLS_KEY", "tmp")
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())

	// clear env variables
	os.Clearenv()

	kcpMockServer.Close()

	cancel()
})
