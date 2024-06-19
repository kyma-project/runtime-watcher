package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
	. "github.com/kyma-project/runtime-watcher/skr/tests/e2e/utils"
)

var _ = Describe("Watcher Metrics", Ordered, func() {
	kyma := NewKyma(kymaName, controlPlaneNamespace, kymaChannel)
	initEmptyKymaBeforeAll(kyma)
	cleanupKymaAfterAll(kyma)

	watcher := ResourceName{
		Namespace: remoteNamespace,
		Name:      watcherName,
	}

	Context("Given SKR Cluster", func() {
		It("When SKR Webhook metrics endpoint is exposed", func() {
			Expect(ExposeSKRMetricsServiceEndpoint()).To(Succeed())

			By("Runtime Watcher deployment is ready")
			Eventually(deploymentReady).
				WithContext(ctx).
				WithArguments(runtimeClient, watcher).
				Should(Succeed())

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

		It("When kyma does not have owned by annotation", func() {
			Eventually(AddSkipReconciliationLabelToKyma).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			Eventually(RemoveKymaAnnotations).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace).
				Should(Succeed())

			By("And spec of SKR Kyma CR is changed", func() {
				Eventually(changeRemoteKymaChannel).
					WithContext(ctx).
					WithArguments(runtimeClient, "regular").
					Should(Succeed())
			})
		})

		It("Then Watcher Failed Kcp Metric is 1", func() {
			Eventually(GetWatcherFailedKcpTotalMetric).
				WithContext(ctx).
				WithArguments(watchermetrics.ReasonOwner).
				Should(BeNumerically(">=", 1))
		})
	})
})
