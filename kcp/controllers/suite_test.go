/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers_test

// TODO:test pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg        *rest.Config         //nolint:gochecknoglobals
	k8sClient  client.Client        //nolint:gochecknoglobals
	k8sManager manager.Manager      //nolint:gochecknoglobals
	testEnv    *envtest.Environment //nolint:gochecknoglobals
	ctx        context.Context      //nolint:gochecknoglobals
	cancel     context.CancelFunc   //nolint:gochecknoglobals
)

const (
	webhookChartPath = "../pkg/deploy/assets/sample-chart"
	requeueInterval  = 500
	vsName           = "kcp-events"
	vsNamespace      = metav1.NamespaceDefault
	releaseName      = "watcher"
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("verifying local chart path is correct")
	fileInfo, err := os.Stat(webhookChartPath)
	Expect(err).NotTo(HaveOccurred())
	Expect(fileInfo.IsDir()).To(BeTrue())

	By("preparing required CRDs")
	//nolint:lll
	requiredCrds, err := prepareRequiredCRDs([]string{
		"https://raw.githubusercontent.com/istio/istio/master/manifests/charts/base/crds/crd-all.gen.yaml",
		"https://raw.githubusercontent.com/kyma-project/runtime-watcher/main/kcp/config/crd/bases/operator.kyma-project.io_watchers.yaml",
		"https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/operator/config/crd/bases/operator.kyma-project.io_kymas.yaml",
	})
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		CRDs:                  requiredCrds,
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(watcherv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextv1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(kyma.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	watcherReconciler := &controllers.WatcherReconciler{
		Client:     k8sManager.GetClient(),
		RestConfig: k8sManager.GetConfig(),
		Scheme:     scheme.Scheme,
		Config: &controllers.WatcherConfig{
			VirtualServiceObjKey: client.ObjectKey{
				Name:      vsName,
				Namespace: vsNamespace,
			},
			RequeueInterval:         requeueInterval,
			WebhookChartPath:        webhookChartPath,
			WebhookChartReleaseName: releaseName,
		},
	}
	err = watcherReconciler.SetIstioClient()
	Expect(err).ToNot(HaveOccurred())
	err = watcherReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("cancelling the context for the manager to shutdown")
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
