package auth

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/status"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	"github.com/openshift/cluster-kube-apiserver-operator/pkg/operator/configobservation"
	"github.com/openshift/cluster-kube-apiserver-operator/pkg/operator/operatorclient"
)

var (
	webhookTokenAuthenticatorPath        = []string{"apiServerArguments", "authentication-token-webhook-config-file"}
	webhookTokenAuthenticatorFile        = []interface{}{"/etc/kubernetes/static-pod-resources/secrets/webhook-authenticator/kubeConfig"}
	webhookTokenAuthenticatorVersionPath = []string{"apiServerArguments", "authentication-token-webhook-version"}
	webhookTokenAuthenticatorVersion     = []interface{}{"v1"}
)

// TODO in 4.9, this is removed because the webhook is always available
// We determine readiness by...
// 1. if the webhook is not set, it is not ready.
// 2. if webhook is set and does not point to secret/openshift-oauth-apiserver, the webhook authenticator configuration is ready
// 3. if webhook is set and points to secret/openshift-oauth-apiserver and the existing observedConfig shows a webhook configuration
//    the webhook authenticator configuration is ready.  This allows stickiness after being set once.
// 4. if webhook is set and points to secret/openshift-oauth-apiserver and the authentication operator is at the same level as the
//    kube-apiserver, the webhook authenticator configuration is ready.  This ensures, the oauth-apiserver is ready to
//    answer webhook requests
// 5. if webhook is set and points to secret/openshift-oauth-apiserver and the clusterversion indicates this is an initial
//    install, the webhook authenticator is ready.  This avoids a late stage revision during install because some people care.
// overall, it is better to return true than false if the choice is ambiguous
func IsWebhookAuthenticatorReady(listers configobservation.Listers, existingConfig map[string]interface{}) (bool, error) {
	auth, err := listers.AuthConfigLister.Get("cluster")
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return true, err
	}

	webhookSecretName := ""
	if auth.Spec.WebhookTokenAuthenticator != nil {
		webhookSecretName = auth.Spec.WebhookTokenAuthenticator.KubeConfig.Name
	}
	// case 1
	if len(webhookSecretName) == 0 {
		return false, nil
	}
	// case 2
	// if the value is custom set, then it is always considered ready and authoritative
	if webhookSecretName != "openshift-oauth-apiserver" {
		return true, nil
	}

	// at this point, we know we're configured for internal oauth, *but* it is possible that this is an update the
	// authentication operator isn't available yet.  However, once we start using internal  oauth as a webhook, we
	// should never stop.

	existingWebhookAuthenticator, _, err := unstructured.NestedSlice(existingConfig, webhookTokenAuthenticatorPath...)
	if err != nil {
		return true, err
	}
	// case 3
	existingWebhookConfigured := len(existingWebhookAuthenticator) > 0
	if existingWebhookConfigured {
		return true, nil
	}

	authenticationOperator, err := listers.ClusterOperatorLister.Get("authentication")
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return true, err
	}
	expectedVersion := status.VersionForOperandFromEnv()
	// case 4 - if oauth-apiserver is serving at a level that includes our webhook, use it.
	for _, authVersion := range authenticationOperator.Status.Versions {
		if authVersion.Name == "operator" && authVersion.Version == expectedVersion {
			return true, nil
		}
	}

	clusterVersion, err := listers.ClusterVersionLister.Get("version")
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return true, err
	}
	// case 5 - if this is the first version ever installed, we're ready to use the webhook.
	if len(clusterVersion.Status.History) <= 1 {
		return true, nil
	}

	return false, nil
}

// ObserveWebhookTokenAuthenticator observes the webhookTokenAuthenticator field of
// the authentication.config/cluster resource and if kubeConfig secret reference is
// set it uses the contents of this secret as a webhhook token authenticator
// for the API server. It also takes care of synchronizing this secret to the
// openshift-kube-apiserver NS.
func ObserveWebhookTokenAuthenticator(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (ret map[string]interface{}, _ []error) {
	defer func() {
		configobserver.Pruned(ret, webhookTokenAuthenticatorPath)
	}()

	listers := genericListers.(configobservation.Listers)
	resourceSyncer := genericListers.ResourceSyncer()

	errs := []error{}
	existingWebhookAuthenticator, _, err := unstructured.NestedSlice(existingConfig, webhookTokenAuthenticatorPath...)
	if err != nil {
		// keep going on read error from existing config
		errs = append(errs, err)
	}

	if isReady, err := IsWebhookAuthenticatorReady(listers, existingConfig); err != nil {
		// if we had an error determining, just show the error and keep the existing config.
		return existingConfig, append(errs, err)
	} else if !isReady {
		// if we aren't yet ready to use the webhook configuration, clear the webhook configuration
		// don't sync anything and remove whatever we synced
		resourceSyncer.SyncSecret(
			resourcesynccontroller.ResourceLocation{Namespace: operatorclient.TargetNamespace, Name: "webhook-authenticator"},
			resourcesynccontroller.ResourceLocation{Namespace: "", Name: ""},
		)
		return map[string]interface{}{}, nil
	}

	existingWebhookConfigured := len(existingWebhookAuthenticator) > 0
	observedConfig := map[string]interface{}{}

	auth, err := listers.AuthConfigLister.Get("cluster")
	if errors.IsNotFound(err) {
		return observedConfig, nil
	} else if err != nil {
		return existingConfig, append(errs, err)
	}

	var webhookSecretName string
	if auth.Spec.WebhookTokenAuthenticator != nil {
		webhookSecretName = auth.Spec.WebhookTokenAuthenticator.KubeConfig.Name
	}

	observedWebhookConfigured := len(webhookSecretName) > 0
	if observedWebhookConfigured {
		// retrieve the secret from config and validate it, don't proceed on failure
		kubeconfigSecret, err := listers.ConfigSecretLister().Secrets("openshift-config").Get(webhookSecretName)
		if err != nil {
			return existingConfig, append(errs, fmt.Errorf("failed to get secret openshift-config/%s: %w", webhookSecretName, err))
		}

		if secretErrors := validateKubeconfigSecret(kubeconfigSecret); len(secretErrors) > 0 {
			return existingConfig, append(errs,
				fmt.Errorf("secret openshift-config/%s is invalid: %w", webhookSecretName, utilerrors.NewAggregate(secretErrors)))
		}

		if err := unstructured.SetNestedField(observedConfig, webhookTokenAuthenticatorVersion, webhookTokenAuthenticatorVersionPath...); err != nil {
			return existingConfig, append(errs, err)
		}

		if err := unstructured.SetNestedField(observedConfig, webhookTokenAuthenticatorFile, webhookTokenAuthenticatorPath...); err != nil {
			return existingConfig, append(errs, err)
		}

		resourceSyncer.SyncSecret(
			resourcesynccontroller.ResourceLocation{Namespace: operatorclient.TargetNamespace, Name: "webhook-authenticator"},
			resourcesynccontroller.ResourceLocation{Namespace: operatorclient.GlobalUserSpecifiedConfigNamespace, Name: webhookSecretName},
		)
	} else {
		// don't sync anything and remove whatever we synced
		resourceSyncer.SyncSecret(
			resourcesynccontroller.ResourceLocation{Namespace: operatorclient.TargetNamespace, Name: "webhook-authenticator"},
			resourcesynccontroller.ResourceLocation{Namespace: "", Name: ""},
		)
	}

	if observedWebhookConfigured != existingWebhookConfigured {
		recorder.Eventf(
			"ObserveWebhookTokenAuthenticator",
			"authentication-token webhook configuration status changed from %v to %v",
			existingWebhookConfigured, observedWebhookConfigured,
		)
	}

	return observedConfig, errs
}

func validateKubeconfigSecret(secret *corev1.Secret) []error {
	kubeconfigRaw, ok := secret.Data["kubeConfig"]
	if !ok {
		return []error{fmt.Errorf("missing required 'kubeConfig' key")}
	}

	if len(kubeconfigRaw) == 0 {
		return []error{fmt.Errorf("the 'kubeConfig' key is empty")}
	}

	kubeconfig, err := clientcmd.Load(kubeconfigRaw)
	if err != nil {
		return []error{fmt.Errorf("failed to load kubeconfig: %w", err)}
	}

	errs := validateClusters(kubeconfig.Clusters)
	errs = append(errs, validateUsers(kubeconfig.AuthInfos)...)
	return append(errs, validateContexts(kubeconfig)...)
}

func validateClusters(clusters map[string]*clientcmdapi.Cluster) []error {
	errs := []error{}

	clustersPath := field.NewPath("clusters")
	if len(clusters) != 1 {
		errs = append(errs, field.Invalid(clustersPath, clusters, "expected a single cluster"))
	}

	for clusterName, cluster := range clusters {
		currentClusterPath := clustersPath.Key(clusterName)

		if len(cluster.Server) == 0 {
			errs = append(errs, field.Required(currentClusterPath.Child("server"), ""))
		}

		if caFilePath := cluster.CertificateAuthority; len(caFilePath) > 0 {
			errs = append(errs, fieldRedirect(currentClusterPath, caFilePath, "certificate-authority", "certificate-authority-data"))
		}
	}
	return errs
}

func validateUsers(users map[string]*clientcmdapi.AuthInfo) []error {
	errs := []error{}

	usersPath := field.NewPath("users")
	if len(users) != 1 {
		errs = append(errs, field.Invalid(usersPath, users, "expected a single user"))
	}

	for userName, user := range users {
		currentField := usersPath.Key(userName)

		// check that authentication is configured
		switch {
		case len(user.Username) > 0:
			if len(user.Password) == 0 {
				errs = append(errs, field.Required(currentField.Child("password"), "required when 'username' is set"))
			}

		case len(user.ClientCertificateData) > 0:
			if len(user.ClientKeyData) == 0 {
				errs = append(errs, field.Required(currentField.Child("client-key-data"), "required when 'client-certificate-data' is set"))
			}
		case len(user.Token) > 0:
		default:
			errs = append(errs, field.Required(currentField, "at least one authentication mechanism needs to be configured"))
		}

		if clientCert := user.ClientCertificate; len(clientCert) > 0 {
			errs = append(errs, fieldRedirect(currentField, clientCert, "client-certificate", "client-certificate-data"))
		}
		if clientKey := user.ClientKey; len(clientKey) > 0 {
			errs = append(errs, fieldRedirect(currentField, clientKey, "client-key", "client-key-data"))
		}
		if tokenFile := user.TokenFile; len(tokenFile) > 0 {
			errs = append(errs, fieldRedirect(currentField, tokenFile, "tokenFile", "token"))
		}
	}

	return errs
}

func validateContexts(kubeconfig *clientcmdapi.Config) []error {
	errs := []error{}
	if len(kubeconfig.CurrentContext) == 0 {
		errs = append(errs, field.Required(field.NewPath("current-context"), ""))
	}

	contextsPath := field.NewPath("contexts")
	if len(kubeconfig.Contexts) != 1 {
		errs = append(errs, field.Invalid(contextsPath, kubeconfig.Contexts, "expected a single value"))
	}

	if selectedContext, ok := kubeconfig.Contexts[kubeconfig.CurrentContext]; !ok {
		errs = append(errs, field.Invalid(
			field.NewPath("current-context"),
			kubeconfig.CurrentContext,
			"does not appear to be present in the 'contexts' field"),
		)
	} else {
		currentPath := contextsPath.Key(kubeconfig.CurrentContext)
		if _, ok := kubeconfig.AuthInfos[selectedContext.AuthInfo]; !ok {
			errs = append(errs, field.Invalid(currentPath.Child("user"), selectedContext.AuthInfo, "this value cannot be found in 'users'"))
		}

		if _, ok := kubeconfig.Clusters[selectedContext.Cluster]; !ok {
			errs = append(errs, field.Invalid(currentPath.Child("cluster"), selectedContext.Cluster, "this value cannot be found in 'clusters'"))
		}
	}

	return errs
}

func fieldRedirect(fld *field.Path, value interface{}, origField, newField string) error {
	return field.Invalid(fld.Child(origField), value, fmt.Sprintf("use %q with the direct content of the file instead", fld.Child(newField).String()))
}
