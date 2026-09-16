package openshift

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider"
)

const servingSecretAnnotation = "service.beta.openshift.io/serving-cert-secret-name"
const caBundleAnnotation = "service.beta.openshift.io/inject-cabundle"

// Provider configures OpenShift Service CA annotations.
type Provider struct{}

// New returns an OpenShift certificate provider.
func New() provider.Provider {
	return Provider{}
}

func (Provider) ConfigureService(obj *unstructured.Unstructured, target provider.Target) error {
	return provider.SetAnnotation(obj, servingSecretAnnotation, target.SecretName)
}

func (Provider) ConfigureWebhooks(objects []unstructured.Unstructured, target provider.Target) error {
	return provider.ConfigureWebhookAnnotation(objects, target, caBundleAnnotation, "true")
}

func (Provider) EnsureCertificate(objects []unstructured.Unstructured, _ provider.Target) ([]unstructured.Unstructured, error) {
	return objects, nil
}
