package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/runtime-watcher/skr/tests/e2e/utils"
)

var _ = Describe("Watcher Metrics", Ordered, func() {
	kyma := NewKyma(kymaName, controlPlaneNamespace, kymaChannel,
		v1beta2.SyncStrategyLocalSecret)
	initEmptyKymaBeforeAll(kyma)
	cleanupKymaAfterAll(kyma)

	watcher := ResourceName{
		Namespace: remoteNamespace,
		Name:      watcherName,
	}

	Context("Given SKR Cluster", func() {
		It("When Metrics Endpoint is exposed", func() {
			Expect(ExposeSKRMetricsServiceEndpoint()).To(Succeed())

			By("And Runtime Watcher deployment is ready", func() {
				Eventually(deploymentReady).
					WithContext(ctx).
					WithArguments(runtimeClient, watcher).
					Should(Succeed())
			})

			By("And spec of SKR Kyma CR is changed", func() {
				Eventually(changeRemoteKymaChannel).
					WithContext(ctx).
					WithArguments(runtimeClient, "fast").
					Should(Succeed())
			})
		})

		It("Then Watcher Request Duration Metric is recorded", func() {
			Eventually(GetWatcherRequestDurationMetric).
				WithContext(ctx).
				Should(BeNumerically(">", float64(0)))

			By("And kcp requests metric is incremented", func() {
				Eventually(GetKcpRequestsMetric).
					WithContext(ctx).
					Should(BeNumerically(">", 0))
			})

			By("And admission requests metric is incremented", func() {
				Eventually(GetAdmissionRequestsMetric).
					WithContext(ctx).
					Should(BeNumerically(">", 0))
			})
		})
	})
})
