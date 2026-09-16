package provider

import (
	"fmt"
	"maps"

	k8sutil "github.com/k8s-manifest-kit/pkg/util/k8s"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func SameObject(left unstructured.Unstructured, right unstructured.Unstructured) bool {
	return left.GetAPIVersion() == right.GetAPIVersion() &&
		left.GetKind() == right.GetKind() &&
		left.GetNamespace() == right.GetNamespace() &&
		left.GetName() == right.GetName()
}

func SetAnnotation(obj *unstructured.Unstructured, key string, value string) error {
	annotations := maps.Clone(obj.GetAnnotations())
	if current, exists := annotations[key]; exists && current != value {
		return fmt.Errorf("%w: %s annotation is %q, requested %q", ErrConflict, key, current, value)
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)

	return nil
}

func ConfigureWebhookAnnotation(objects []unstructured.Unstructured, target Target, key string, value string) error {
	for _, webhook := range target.Webhooks {
		index := findWebhook(objects, webhook)
		if index < 0 {
			continue
		}

		obj := &objects[index]
		if err := validateWebhookEntries(*obj, webhook, target.Service); err != nil {
			return err
		}
		if err := SetAnnotation(obj, key, value); err != nil {
			return err
		}
	}

	return nil
}

func CertificateReference(target Target) string {
	if target.Certificate.ExistingRef != nil {
		return target.Certificate.ExistingRef.Namespace + "/" + target.Certificate.ExistingRef.Name
	}

	return target.Service.Namespace + "/" + target.Certificate.Generated.Name
}

func ServiceDNSNames(target Target) []string {
	name := target.Service.Name
	namespace := target.Service.Namespace

	return []string{
		name,
		name + "." + namespace,
		name + "." + namespace + ".svc",
		name + "." + namespace + ".svc.cluster.local",
	}
}

func findWebhook(objects []unstructured.Unstructured, ref WebhookReference) int {
	for i := range objects {
		if objects[i].GetAPIVersion() != "admissionregistration.k8s.io/v1" || objects[i].GetKind() != string(ref.Kind) || objects[i].GetName() != ref.Name || objects[i].GetNamespace() != "" {
			continue
		}
		return i
	}

	return -1
}

func validateWebhookEntries(obj unstructured.Unstructured, ref WebhookReference, serviceRef k8stypes.NamespacedName) error {
	entries, found, err := unstructured.NestedSlice(obj.Object, "webhooks")
	if err != nil {
		return fmt.Errorf("%w: webhook %s has invalid entries: %v", ErrInvalidConfiguration, k8sutil.DescribeObject(&obj), err)
	}
	if !found || len(entries) == 0 {
		return fmt.Errorf("%w: webhook %s has no webhook entries", ErrInvalidConfiguration, k8sutil.DescribeObject(&obj))
	}

	selected := make(map[string]struct{}, len(ref.EntryNames))
	for _, name := range ref.EntryNames {
		selected[name] = struct{}{}
	}
	foundSelected := make(map[string]struct{}, len(selected))

	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: webhook %s contains a malformed entry", ErrInvalidConfiguration, k8sutil.DescribeObject(&obj))
		}
		entryName, _, _ := unstructured.NestedString(entry, "name")
		if _, ok := selected[entryName]; ok {
			foundSelected[entryName] = struct{}{}
		}

		service, found, err := unstructured.NestedMap(entry, "clientConfig", "service")
		if err != nil || !found {
			return fmt.Errorf("%w: webhook entry %q does not reference a Service", ErrConflict, entryName)
		}
		serviceName, _, _ := unstructured.NestedString(service, "name")
		serviceNamespace, _, _ := unstructured.NestedString(service, "namespace")
		if serviceName != serviceRef.Name || serviceNamespace != serviceRef.Namespace {
			return fmt.Errorf("%w: webhook entry %q references Service %s/%s, requested %s/%s", ErrConflict, entryName, serviceNamespace, serviceName, serviceRef.Namespace, serviceRef.Name)
		}
	}

	for name := range selected {
		if _, ok := foundSelected[name]; !ok {
			return fmt.Errorf("%w: webhook entry %q was not found in %s", ErrInvalidConfiguration, name, k8sutil.DescribeObject(&obj))
		}
	}

	return nil
}
