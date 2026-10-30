package cert

import (
	"github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

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

	return testDeploymentWithPodSpec(namespace, name, labels, map[string]any{
		"containers": containerValues,
	})
}

func testDeploymentWithPodSpec(namespace string, name string, labels map[string]any, podSpec map[string]any) unstructured.Unstructured {
	return testObject("apps/v1", "Deployment", namespace, name, map[string]any{
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels":    labels,
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": podSpec,
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

func testCertificate(namespace string, name string, spec map[string]any) unstructured.Unstructured {
	return testObject("cert-manager.io/v1", "Certificate", namespace, name, map[string]any{
		"spec": spec,
	})
}

// testGeneratedCertificateSpec returns the Certificate spec the cert-manager
// provider derives for Service webhook-system/webhook with issuer internal-ca
// and the extra DNS name webhook.custom.example.
func testGeneratedCertificateSpec() map[string]any {
	return map[string]any{
		"secretName": "webhook",
		"issuerRef": map[string]any{
			"name":  "internal-ca",
			"kind":  "Issuer",
			"group": "cert-manager.io",
		},
		"dnsNames": []any{
			"webhook",
			"webhook.webhook-system",
			"webhook.webhook-system.svc",
			"webhook.webhook-system.svc.cluster.local",
			"webhook.custom.example",
		},
	}
}

func testSetIssuerRefField(spec map[string]any, field string, value string) {
	issuerRef := spec["issuerRef"].(map[string]any)
	issuerRef[field] = value
}

func testContainer(name string, mounts ...map[string]any) map[string]any {
	container := map[string]any{"name": name}
	if len(mounts) > 0 {
		mountValues := make([]any, len(mounts))
		for i, mount := range mounts {
			mountValues[i] = mount
		}
		container["volumeMounts"] = mountValues
	}

	return container
}

func testVolumeMount(volumeName string, mountPath string) map[string]any {
	return map[string]any{
		"name":      volumeName,
		"mountPath": mountPath,
	}
}

func testReadOnlyVolumeMount(volumeName string, mountPath string) map[string]any {
	mount := testVolumeMount(volumeName, mountPath)
	mount["readOnly"] = true
	return mount
}

func testWritableVolumeMount(volumeName string, mountPath string) map[string]any {
	mount := testVolumeMount(volumeName, mountPath)
	mount["readOnly"] = false
	return mount
}

func testSecretVolume(volumeName string, secretName string) map[string]any {
	return map[string]any{
		"name":   volumeName,
		"secret": map[string]any{"secretName": secretName},
	}
}

func testConfigMapVolume(volumeName string, configName string) map[string]any {
	return map[string]any{
		"name":      volumeName,
		"configMap": map[string]any{"name": configName},
	}
}

// testContainersByList returns the regular or init containers of a patched
// workload indexed by container name.
func testContainersByList(obj unstructured.Unstructured, listName string) map[string]map[string]any {
	containers := map[string]map[string]any{}
	rawContainers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", listName)
	if err != nil || !found {
		return containers
	}
	for _, raw := range rawContainers {
		container, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(container, "name")
		containers[name] = container
	}

	return containers
}

// testContainerMounts returns the volumeMounts of one container, reporting
// whether the container or its mounts were found.
func testContainerMounts(obj unstructured.Unstructured, listName string, containerName string) ([]any, bool) {
	container, ok := testContainersByList(obj, listName)[containerName]
	if !ok {
		return nil, false
	}
	mounts, found, err := unstructured.NestedSlice(container, "volumeMounts")
	if err != nil || !found {
		return nil, false
	}

	return mounts, true
}

func testFindObject(objects []unstructured.Unstructured, namespace string, name string) *unstructured.Unstructured {
	for i := range objects {
		if objects[i].GetNamespace() == namespace && objects[i].GetName() == name {
			return &objects[i]
		}
	}

	return nil
}

func testDeepCopyObjects(objects []unstructured.Unstructured) []unstructured.Unstructured {
	copies := make([]unstructured.Unstructured, len(objects))
	for i := range objects {
		copies[i] = *objects[i].DeepCopy()
	}

	return copies
}

// testAppSelector returns a selector matching the given app label.
func testAppSelector(app string) labels.Selector {
	return labels.SelectorFromSet(labels.Set{"app": app})
}

// testGeneratedCertificateTarget returns a cert-manager target for Service
// webhook-system/webhook with a generated Certificate.
func testGeneratedCertificateTarget() Target {
	return Target{
		Service: k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
		Certificate: CertificateSource{
			Generated: &CertificateSpec{
				IssuerRef: IssuerRef{Name: "internal-ca"},
				DNSNames:  []string{"webhook.custom.example"},
			},
		},
	}
}

// testWorkloadTarget returns an OpenShift target that patches workloads
// labeled app=webhook in webhook-system, optionally selecting containers by
// name. A nil container selector selects every container.
func testWorkloadTarget(containerSelector func(name string) bool) Target {
	return Target{
		Service: k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
		Workload: WorkloadOptions{
			Selector:          testAppSelector("webhook"),
			ContainerSelector: containerSelector,
		},
	}
}

// assertCertificateForService asserts that a generated Certificate targets the
// named Service with the default secret and service DNS names.
func assertCertificateForService(g *gomega.WithT, certificate *unstructured.Unstructured, namespace string, serviceName string) {
	g.Expect(certificate).NotTo(gomega.BeNil())
	g.Expect(certificate.GetAPIVersion()).To(gomega.Equal("cert-manager.io/v1"))
	g.Expect(certificate.GetKind()).To(gomega.Equal("Certificate"))
	g.Expect(certificate.GetNamespace()).To(gomega.Equal(namespace))
	secretName, found, err := unstructured.NestedString(certificate.Object, "spec", "secretName")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(secretName).To(gomega.Equal(serviceName))
	dnsNames, found, err := unstructured.NestedStringSlice(certificate.Object, "spec", "dnsNames")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(dnsNames).To(gomega.Equal([]string{
		serviceName,
		serviceName + "." + namespace,
		serviceName + "." + namespace + ".svc",
		serviceName + "." + namespace + ".svc.cluster.local",
	}))
}

// assertWorkloadMountsSecret asserts that a patched single-container workload
// carries exactly the default TLS Secret volume mounted read-only.
func assertWorkloadMountsSecret(g *gomega.WithT, workload unstructured.Unstructured, secretName string, containerName string) {
	volumes, found, err := unstructured.NestedSlice(workload.Object, "spec", "template", "spec", "volumes")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(volumes).To(gomega.Equal([]any{testSecretVolume("webhook-tls", secretName)}))

	mounts, found := testContainerMounts(workload, "containers", containerName)
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(mounts).To(gomega.Equal([]any{testReadOnlyVolumeMount("webhook-tls", "/tls")}))
}
