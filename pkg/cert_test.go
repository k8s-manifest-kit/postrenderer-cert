package cert

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func TestOpenShiftConfiguresWebhookServiceAndWorkload(t *testing.T) {
	g := NewWithT(t)
	selector, err := labels.Parse("app=webhook")
	g.Expect(err).NotTo(HaveOccurred())

	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(Target{
			Service: k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks: []WebhookReference{
				{Kind: WebhookKindMutating, Name: "webhook-config"},
			},
			Workload: WorkloadOptions{
				Selector: selector,
				ContainerSelector: func(name string) bool {
					return name == "missing"
				},
			},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testWebhookConfiguration("MutatingWebhookConfiguration", "webhook-config", "webhook-system", "webhook"),
		testDeployment(
			"webhook-system",
			"webhook",
			map[string]any{"app": "webhook"},
			map[string]any{"name": "server"},
			map[string]any{"name": "sidecar"},
		),
	}

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects[0].GetAnnotations()).To(BeEmpty())

	service := rendered[0]
	g.Expect(service.GetAnnotations()).To(HaveKeyWithValue("service.beta.openshift.io/serving-cert-secret-name", "webhook"))

	webhook := rendered[1]
	g.Expect(webhook.GetAnnotations()).To(HaveKeyWithValue("service.beta.openshift.io/inject-cabundle", "true"))

	deployment := rendered[2]
	volumes, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "volumes")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(volumes).To(ContainElement(map[string]any{
		"name":   "webhook-tls",
		"secret": map[string]any{"secretName": "webhook"},
	}))
	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	for _, raw := range containers {
		container := raw.(map[string]any)
		mounts, _, nestedErr := unstructured.NestedSlice(container, "volumeMounts")
		g.Expect(nestedErr).NotTo(HaveOccurred())
		g.Expect(mounts).To(ContainElement(map[string]any{
			"name":      "webhook-tls",
			"mountPath": "/tls",
			"readOnly":  true,
		}))
	}

	secondRender, err := postRenderer(t.Context(), rendered)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secondRender).To(Equal(rendered))
}

func TestCertManagerGeneratesCertificateAndConfiguresAllWebhookEntries(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderCertManager),
		WithTarget(Target{
			Service: k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks: []WebhookReference{
				{Kind: WebhookKindValidating, Name: "webhook-config", EntryNames: []string{"first"}},
			},
			Certificate: CertificateSource{
				Generated: &CertificateSpec{
					IssuerRef: IssuerRef{Name: "internal-ca", Kind: "ClusterIssuer"},
					DNSNames:  []string{"webhook.custom.example"},
				},
			},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testWebhookConfigurationWithEntries("ValidatingWebhookConfiguration", "webhook-config", "webhook-system", "webhook", "first", "second"),
	}

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rendered).To(HaveLen(3))

	webhook := rendered[1]
	g.Expect(webhook.GetAnnotations()).To(HaveKeyWithValue("cert-manager.io/inject-ca-from", "webhook-system/webhook-certificate"))

	certificate := rendered[2]
	g.Expect(certificate.GetAPIVersion()).To(Equal("cert-manager.io/v1"))
	g.Expect(certificate.GetKind()).To(Equal("Certificate"))
	g.Expect(certificate.GetNamespace()).To(Equal("webhook-system"))
	g.Expect(certificate.GetName()).To(Equal("webhook-certificate"))
	secretName, found, err := unstructured.NestedString(certificate.Object, "spec", "secretName")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(secretName).To(Equal("webhook"))
	dnsNames, found, err := unstructured.NestedStringSlice(certificate.Object, "spec", "dnsNames")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(dnsNames).To(ContainElements(
		"webhook",
		"webhook.webhook-system",
		"webhook.webhook-system.svc",
		"webhook.webhook-system.svc.cluster.local",
		"webhook.custom.example",
	))
}

func TestCertManagerRejectsMixedWebhookServices(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderCertManager),
		WithTarget(Target{
			Service:  k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks: []WebhookReference{{Kind: WebhookKindValidating, Name: "webhook-config"}},
			Certificate: CertificateSource{
				ExistingRef: &k8stypes.NamespacedName{Namespace: "webhook-system", Name: "ca"},
			},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = postRenderer(t.Context(), []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "", "webhook-config", map[string]any{
			"webhooks": []any{
				testWebhookEntry("first", "webhook-system", "webhook"),
				testWebhookEntry("other-service", "webhook-system", "other-webhook"),
			},
		}),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, ErrConflict)).To(BeTrue())
}

func TestMissingServiceIsSkipped(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(Target{
			Service:  k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks: []WebhookReference{{Kind: WebhookKindMutating, Name: "webhook-config"}},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testWebhookConfiguration("MutatingWebhookConfiguration", "webhook-config", "webhook-system", "webhook"),
	}
	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rendered).To(Equal(objects))
}
