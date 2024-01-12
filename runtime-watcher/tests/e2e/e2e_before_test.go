package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	errSampleCRState     = errors.New("resource not in expected state")
	errFetchingStatus    = errors.New("could not fetch status from resource")
	errKymaState         = errors.New("unexpected state of Kyma CR")
	errModuleNotExisting = errors.New("module does not exists in Kyma CR")
)

const (
	defaultRemoteKymaName = "default"
)

func initEmptyKymaBeforeAll(kyma *v1beta2.Kyma) {
	kymaName := types.NamespacedName{
		Namespace: kyma.Namespace,
		Name:      kyma.Name,
	}
	BeforeAll(func() {
		By("Given KCP Kyma cluster", func() {
			By("When a Kyma Secret is created", func() {
				Eventually(createKymaSecret).
					WithContext(ctx).
					WithArguments(controlPlaneClient, kymaName).
					Should(Succeed())
				By("And a Kyma CR is created", func() {
					Eventually(controlPlaneClient.Create).
						WithContext(ctx).
						WithArguments(kyma).
						Should(Succeed())
				})
			})
			By(`Then the Kyma CR is in a "Ready" State on the KCP cluster `, func() {
				Eventually(kymaIsInState).
					WithContext(ctx).
					WithArguments(controlPlaneClient, kymaName, shared.StateReady).
					Should(Succeed())
				By(`And the Kyma CR is in a "Ready" State on the SKR cluster `, func() {
					Eventually(checkRemoteKymaCR).
						WithContext(ctx).
						WithArguments(runtimeClient, []v1beta2.Module{}, shared.StateReady).
						Should(Succeed())
				})
			})
		})
	})
}

func createKymaSecret(ctx context.Context, k8sClient client.Client, name types.NamespacedName) error {
	patchedRuntimeConfig := strings.ReplaceAll(string(*runtimeConfig), localHostname, k3dHostname)
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels: map[string]string{
				shared.KymaName: name.Name,
			},
		},
		Data: map[string][]byte{"config": []byte(patchedRuntimeConfig)},
	}
	return k8sClient.Create(ctx, secret)
}

func kymaIsInState(ctx context.Context, clnt client.Client, name types.NamespacedName, state shared.State) error {
	gvk := schema.GroupVersionKind{
		Group:   v1beta2.GroupVersion.Group,
		Version: v1beta2.GroupVersion.Version,
		Kind:    string(shared.KymaKind),
	}

	return CRIsInState(ctx, clnt, name, gvk, []string{"status", "state"}, string(state))
}

func CRIsInState(ctx context.Context, clnt client.Client, name types.NamespacedName, gvk schema.GroupVersionKind, statusPath []string, expectedState string) error {
	resourceCR, err := GetCR(ctx, clnt, name, gvk)
	if err != nil {
		return err
	}

	stateFromCR, stateExists, err := unstructured.NestedString(resourceCR.Object, statusPath...)
	if err != nil || !stateExists {
		return errFetchingStatus
	}

	if stateFromCR != expectedState {
		return fmt.Errorf("%w: expect %s, but in %s",
			errSampleCRState, expectedState, stateFromCR)
	}
	return nil
}

func GetCR(ctx context.Context, clnt client.Client, name types.NamespacedName, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := clnt.Get(ctx, name, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func checkRemoteKymaCR(ctx context.Context, clnt client.Client, modules []v1beta2.Module, expected shared.State) error {
	kyma := &v1beta2.Kyma{}
	err := clnt.Get(ctx, client.ObjectKey{Name: defaultRemoteKymaName, Namespace: remoteNamespace}, kyma)
	if err != nil {
		return err
	}

	for _, wantedModule := range modules {
		exists := false
		for _, givenModule := range kyma.Spec.Modules {
			if givenModule.Name == wantedModule.Name &&
				givenModule.Channel == wantedModule.Channel {
				exists = true
				break
			}
		}
		if !exists {
			return fmt.Errorf("%w: %s/%s", errModuleNotExisting, wantedModule.Name, wantedModule.Channel)
		}
	}
	if kyma.Status.State != expected {
		return fmt.Errorf("%w: expect %s, but in %s",
			errKymaState, expected, kyma.Status.State)
	}
	return nil
}
