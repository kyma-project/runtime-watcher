//nolint:gochecknoglobals
package handler_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/madflojo/testcerts"
	"log"
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
	cancel        context.CancelFunc
	kcpRecorder   *httptest.ResponseRecorder
	kcpMockServer *httptest.Server

	managedByLabel = map[string]string{
		internal.ManagedByLabel: "lifecycle-manager",
	}
	ownedByAnnotation = map[string]string{
		internal.OwnedByAnnotation: fmt.Sprintf("%s/%s", metav1.NamespaceDefault, ownerName),
	}
	testEnv   *envtest.Environment
	k8sClient client.Client
)

const (
	moduleName = "kyma"
	crName1    = "kyma-1"
	ownerName  = "ownerName"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	_, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment for skr-watcher tests")

	testEnv = &envtest.Environment{ErrorIfCRDPathMissing: false}
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	kcpTestHandler := BootStrapKcpMockHandlers(moduleName)
	kcpRecorder = kcpTestHandler.Recorder

	ca := testcerts.NewCA()
	err = ca.ToFile("/tmp/ca-cert", "/tmp/ca-key")
	err = os.Setenv("CA_CERT", "/tmp/ca-key")
	Expect(err).NotTo(HaveOccurred())
	certs, err := ca.NewKeyPair("localhost", "127.0.0.1")
	Expect(err).NotTo(HaveOccurred())
	certPath := "/tmp/cert"
	err = os.Setenv("TLS_CERT", certPath)
	//Expect(err).ShouldNot(HaveOccurred())
	keyPath := "/tmp/key"
	err = os.Setenv("TLS_KEY", keyPath)
	//Expect(err).ShouldNot(HaveOccurred())
	err = certs.ToFile(certPath, keyPath)
	Expect(err).NotTo(HaveOccurred())

	// start listener server
	kcpMockServer = httptest.NewTLSServer(kcpTestHandler)
	// Load server certificate and key. These are usually generated specifically for the tests
	cert, err := tls.LoadX509KeyPair("/tmp/cert", "/tmp/key")
	if err != nil {
		log.Fatalf("server: loadkeys: %s", err)
	}

	// start listener server
	kcpMockServer = httptest.NewUnstartedServer(kcpTestHandler)
	kcpMockServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    ca.CertPool(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	kcpMockServer.StartTLS()

	//kcpMockServer.TLS = &tls.Config{
	//	RootCAs: ca.CertPool(),
	//}
	//kcpMockServer.StartTLS()

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
