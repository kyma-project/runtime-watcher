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
	"net/http"
	"path/filepath"
	"testing"
	"time"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/controllers"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	componentv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
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

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("bootstrapping test environment")

	watcherCrd := &v1.CustomResourceDefinition{}
	res, err := http.DefaultClient.Get(
		"https://raw.githubusercontent.com/kyma-project/runtime-watcher/main/kcp/config/crd/bases/component.kyma-project.io_watchers.yaml") //nolint:lll
	Expect(err).NotTo(HaveOccurred())
	Expect(res.StatusCode).To(BeEquivalentTo(http.StatusOK))
	Expect(yaml.NewYAMLOrJSONDecoder(res.Body, 2048).Decode(watcherCrd)).To(Succeed())

	kymaCrd := &v1.CustomResourceDefinition{}
	res, err = http.DefaultClient.Get(
		"https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/operator/config/crd/bases/operator.kyma-project.io_kymas.yaml") //nolint:lll
	Expect(err).NotTo(HaveOccurred())
	Expect(res.StatusCode).To(BeEquivalentTo(http.StatusOK))
	Expect(yaml.NewYAMLOrJSONDecoder(res.Body, 2048).Decode(kymaCrd)).To(Succeed())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		CRDs:                  []*v1.CustomResourceDefinition{watcherCrd, kymaCrd},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(componentv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(kyma.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.KymaReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManager)

	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.WatcherReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
		Config: &util.WatcherConfig{
			RequeueInterval:          util.DefaultRequeueInterval,
			ListenerIstioGatewayPort: util.DefaultIstioGatewayPort,
		},
	}).SetupWithManager(k8sManager)

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	// Set 4 with random, to avoid `timeout waiting for process kube-apiserver to stop`
	if err != nil {
		time.Sleep(4 * time.Second)
	}
	err = testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
