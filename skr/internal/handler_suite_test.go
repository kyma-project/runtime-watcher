package internal_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kyma-project/runtime-watcher/skr/internal"
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

	managedbyLabel = map[string]string{ //nolint:gochecknoglobals
		internal.ManagedByLabel: "lifecycle-manager",
	}
	ownedbyAnnotation = map[string]string{ //nolint:gochecknoglobals
		internal.OwnedByAnnotation: fmt.Sprintf("%s/%s", metav1.NamespaceDefault, ownerName),
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

	// set KCP env vars
	err = os.Setenv("KCP_ADDR", kcpMockServer.Listener.Addr().String())
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
