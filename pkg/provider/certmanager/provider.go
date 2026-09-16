package certmanager

import (
	"fmt"
	"slices"

	"github.com/k8s-manifest-kit/pkg/util"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider"
)

const (
	injectCAAnnotation = "cert-manager.io/inject-ca-from"
	apiVersion         = "cert-manager.io/v1"
	kind               = "Certificate"
)

// Provider configures cert-manager Certificate and cainjector resources.
type Provider struct{}

// New returns a cert-manager certificate provider.
func New() provider.Provider {
	return Provider{}
}

func (Provider) ConfigureService(_ *unstructured.Unstructured, _ provider.Target) error {
	return nil
}

func (Provider) ConfigureWebhooks(objects []unstructured.Unstructured, target provider.Target) error {
	return provider.ConfigureWebhookAnnotation(objects, target, injectCAAnnotation, provider.CertificateReference(target))
}

func (Provider) EnsureCertificate(objects []unstructured.Unstructured, target provider.Target) ([]unstructured.Unstructured, error) {
	if target.Certificate.ExistingRef != nil {
		return objects, nil
	}

	spec := target.Certificate.Generated
	desired := generatedCertificate(target, *spec)

	for i := range objects {
		if !provider.SameObject(objects[i], desired) {
			continue
		}
		if err := validateExistingCertificate(objects[i], desired, target.SecretName); err != nil {
			return nil, err
		}

		return objects, nil
	}

	return append(objects, desired), nil
}

func generatedCertificate(target provider.Target, spec provider.CertificateSpec) unstructured.Unstructured {
	dnsNames := provider.ServiceDNSNames(target)
	for _, name := range spec.DNSNames {
		if !slices.Contains(dnsNames, name) {
			dnsNames = append(dnsNames, name)
		}
	}

	issuerRef := map[string]any{
		"name":  spec.IssuerRef.Name,
		"kind":  spec.IssuerRef.Kind,
		"group": spec.IssuerRef.Group,
	}

	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name":      spec.Name,
			"namespace": target.Service.Namespace,
		},
		"spec": map[string]any{
			"secretName": target.SecretName,
			"dnsNames":   util.ToAnySlice(dnsNames),
			"issuerRef":  issuerRef,
		},
	}}
}

func validateExistingCertificate(obj unstructured.Unstructured, desired unstructured.Unstructured, secretName string) error {
	desiredSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
	actualSpec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("%w: Certificate %s/%s has no spec", provider.ErrConflict, obj.GetNamespace(), obj.GetName())
	}

	actualSecret, _, _ := unstructured.NestedString(actualSpec, "secretName")
	desiredSecret, _, _ := unstructured.NestedString(desiredSpec, "secretName")
	if actualSecret != desiredSecret || actualSecret != secretName {
		return fmt.Errorf("%w: Certificate %s/%s uses Secret %q, requested %q", provider.ErrConflict, obj.GetNamespace(), obj.GetName(), actualSecret, secretName)
	}

	actualIssuer, _, _ := unstructured.NestedMap(actualSpec, "issuerRef")
	desiredIssuer, _, _ := unstructured.NestedMap(desiredSpec, "issuerRef")
	for _, field := range []string{"name", "kind", "group"} {
		actualValue, _, _ := unstructured.NestedString(actualIssuer, field)
		desiredValue, _, _ := unstructured.NestedString(desiredIssuer, field)
		if actualValue != desiredValue {
			return fmt.Errorf("%w: Certificate %s/%s issuerRef.%s is %q, requested %q", provider.ErrConflict, obj.GetNamespace(), obj.GetName(), field, actualValue, desiredValue)
		}
	}

	actualDNSNames, _, _ := unstructured.NestedStringSlice(actualSpec, "dnsNames")
	desiredDNSNames, _, _ := unstructured.NestedStringSlice(desiredSpec, "dnsNames")
	actualDNSNames = slices.Clone(actualDNSNames)
	desiredDNSNames = slices.Clone(desiredDNSNames)
	slices.Sort(actualDNSNames)
	slices.Sort(desiredDNSNames)
	if !slices.Equal(actualDNSNames, desiredDNSNames) {
		return fmt.Errorf("%w: Certificate %s/%s dnsNames differ from requested configuration", provider.ErrConflict, obj.GetNamespace(), obj.GetName())
	}

	return nil
}
