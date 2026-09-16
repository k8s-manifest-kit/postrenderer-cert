package cert

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func testObject(apiVersion string, kind string, namespace string, name string, fields map[string]any) unstructured.Unstructured {
	object := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}
	for key, value := range fields {
		object[key] = value
	}

	return unstructured.Unstructured{Object: object}
}

func testWebhookEntry(name string, namespace string, service string) map[string]any {
	return map[string]any{
		"name": name,
		"clientConfig": map[string]any{
			"service": map[string]any{
				"name":      service,
				"namespace": namespace,
			},
		},
	}
}

func testDeployment(namespace string, name string, labels map[string]any, containers ...map[string]any) unstructured.Unstructured {
	containerValues := make([]any, len(containers))
	for i, container := range containers {
		containerValues[i] = container
	}

	return testObject("apps/v1", "Deployment", namespace, name, map[string]any{
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels":    labels,
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": containerValues,
				},
			},
		},
	})
}

func testWebhookConfiguration(kind string, name string, namespace string, service string) unstructured.Unstructured {
	return testWebhookConfigurationWithEntries(kind, name, namespace, service, "webhook")
}

func testWebhookConfigurationWithEntries(kind string, name string, namespace string, service string, entryNames ...string) unstructured.Unstructured {
	entries := make([]any, len(entryNames))
	for i, entryName := range entryNames {
		entries[i] = testWebhookEntry(entryName, namespace, service)
	}

	return testObject("admissionregistration.k8s.io/v1", kind, "", name, map[string]any{
		"webhooks": entries,
	})
}
