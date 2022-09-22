package internal_test

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kyma-project/runtime-watcher/skr/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	ctx           context.Context            //nolint:gochecknoglobals
	cancel        context.CancelFunc         //nolint:gochecknoglobals
	kcpRecorder   *httptest.ResponseRecorder //nolint:gochecknoglobals
	kcpMockServer *httptest.Server           //nolint:gochecknoglobals

	ownerLabels = map[string]string{ //nolint:gochecknoglobals
		internal.ManagedByLabel: "lifecycle-manager",
		internal.OwnedByLabel:   fmt.Sprintf("%s__%s", metav1.NamespaceDefault, ownerName),
	}
	testEnv   *envtest.Environment //nolint:gochecknoglobals
	k8sClient client.Client        //nolint:gochecknoglobals
)

const (
	moduleName = "kyma"
	crName1    = "kyma-1"
	ownerName  = "ownerName"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

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
	_, port, err := net.SplitHostPort(kcpMockServer.Listener.Addr().String())
	Expect(err).ShouldNot(HaveOccurred())

	// set KCP env vars
	err = os.Setenv("KCP_IP", "localhost")
	Expect(err).ShouldNot(HaveOccurred())
	err = os.Setenv("KCP_PORT", port)
	Expect(err).ShouldNot(HaveOccurred())
	err = os.Setenv("KCP_CONTRACT", "v1")
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())

	// clear env variables
	os.Clearenv()

	kcpMockServer.Close()

	cancel()
})
