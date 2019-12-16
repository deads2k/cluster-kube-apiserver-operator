package e2e

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	operatorv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/cluster-kube-apiserver-operator/pkg/operator/operatorclient"
	test "github.com/openshift/cluster-kube-apiserver-operator/test/library"
	configv1helpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/operator/genericoperatorclient"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestCertRotationTimeUpgradeable(t *testing.T) {
	kubeConfig, err := test.NewClientConfigForTest()
	require.NoError(t, err)
	operatorClient, _, err := genericoperatorclient.NewStaticPodOperatorClient(kubeConfig, operatorv1.GroupVersion.WithResource("kubeapiservers"))
	require.NoError(t, err)
	configClient, err := configclient.NewForConfig(kubeConfig)
	require.NoError(t, err)

	_, operatorStatus, _, err := operatorClient.GetStaticPodOperatorStateWithQuorum()
	require.NoError(t, err)
	require.True(t, v1helpers.IsOperatorConditionTrue(operatorStatus.Conditions, "CertRotationTimeUpgradeable"))

	kubeClient := kubernetes.NewForConfigOrDie(kubeConfig)
	_, err = kubeClient.CoreV1().ConfigMaps(operatorclient.GlobalUserSpecifiedConfigNamespace).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: operatorclient.GlobalUserSpecifiedConfigNamespace, Name: "unsupported-cert-rotation-config"},
		Data:       map[string]string{"base": "2y"},
	})
	require.NoError(t, err)
	defer func() {
		kubeClient.CoreV1().ConfigMaps(operatorclient.GlobalUserSpecifiedConfigNamespace).Delete("unsupported-cert-rotation-config", nil)
	}()

	// TODO better detection maybe someday
	time.Sleep(5 * time.Second)

	_, operatorStatus, _, err = operatorClient.GetStaticPodOperatorStateWithQuorum()
	require.NoError(t, err)
	require.True(t, v1helpers.IsOperatorConditionFalse(operatorStatus.Conditions, "CertRotationTimeUpgradeable"))
	clusteroperator, err := configClient.ClusterOperators().Get("kube-apiserver", metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, configv1helpers.IsStatusConditionFalse(clusteroperator.Status.Conditions, "Upgradeable"))

	err = kubeClient.CoreV1().ConfigMaps(operatorclient.GlobalUserSpecifiedConfigNamespace).Delete("unsupported-cert-rotation-config", nil)
	require.NoError(t, err)
	// TODO better detection maybe someday
	time.Sleep(5 * time.Second)

	_, operatorStatus, _, err = operatorClient.GetStaticPodOperatorStateWithQuorum()
	require.NoError(t, err)
	require.True(t, v1helpers.IsOperatorConditionTrue(operatorStatus.Conditions, "CertRotationTimeUpgradeable"))
	clusteroperator, err = configClient.ClusterOperators().Get("kube-apiserver", metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, configv1helpers.IsStatusConditionTrue(clusteroperator.Status.Conditions, "Upgradeable"))
}

func TestCertStompOnBadType(t *testing.T) {
	kubeConfig, err := test.NewClientConfigForTest()
	require.NoError(t, err)
	kubeClient := kubernetes.NewForConfigOrDie(kubeConfig)

	// this is fundamentally racy, we're trying to screw over a controller.
	// set the secret to an incompatible type.
	wait.PollImmediate(50*time.Millisecond, 5*time.Second, func() (bool, error) {
		err = kubeClient.CoreV1().Secrets(operatorclient.OperatorNamespace).Delete("aggregator-client-signer", nil)
		if err != nil {
			return false, err
		}
		_, err = kubeClient.CoreV1().Secrets(operatorclient.OperatorNamespace).Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: operatorclient.OperatorNamespace, Name: "aggregator-client-signer"},
			Type:       "Opaque",
		})
		if err != nil {
			t.Log(err)
			return false, nil
		}
		return true, nil
	})
	require.NoError(t, err)

	// wait for the controller to put things right.
	wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		actual, err := kubeClient.CoreV1().Secrets(operatorclient.OperatorNamespace).Get("aggregator-client-signer", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if actual.Type == "kubernetes.io/tls" {
			return true, nil
		}
		t.Log(actual.Type)
		return false, nil
	})
	require.NoError(t, err)
}
