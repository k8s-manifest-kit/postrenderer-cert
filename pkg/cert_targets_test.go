package cert

import (
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func TestMultipleOpenShiftTargetsConfigureOnlyTheirOwnResources(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(Target{
			Service:  k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks: []WebhookReference{{Kind: WebhookKindMutating, Name: "webhook-config"}},
			Workload: WorkloadOptions{Selector: testAppSelector("webhook")},
		}),
		WithTarget(Target{
			Service:  k8stypes.NamespacedName{Namespace: "other-system", Name: "other"},
			Webhooks: []WebhookReference{{Kind: WebhookKindValidating, Name: "other-config"}},
			Workload: WorkloadOptions{Selector: testAppSelector("other")},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testObject("v1", "Service", "other-system", "other", nil),
		testWebhookConfiguration("MutatingWebhookConfiguration", "webhook-config", "webhook-system", "webhook"),
		testWebhookConfiguration("ValidatingWebhookConfiguration", "other-config", "other-system", "other"),
		testDeployment("webhook-system", "webhook", map[string]any{"app": "webhook"}, map[string]any{"name": "server"}),
		testDeployment("other-system", "other", map[string]any{"app": "other"}, map[string]any{"name": "other-server"}),
	}

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rendered).To(HaveLen(6))

	g.Expect(rendered[0].GetAnnotations()).To(Equal(map[string]string{
		"service.beta.openshift.io/serving-cert-secret-name": "webhook",
	}))
	g.Expect(rendered[1].GetAnnotations()).To(Equal(map[string]string{
		"service.beta.openshift.io/serving-cert-secret-name": "other",
	}))
	g.Expect(rendered[2].GetAnnotations()).To(Equal(map[string]string{
		"service.beta.openshift.io/inject-cabundle": "true",
	}))
	g.Expect(rendered[3].GetAnnotations()).To(Equal(map[string]string{
		"service.beta.openshift.io/inject-cabundle": "true",
	}))

	assertWorkloadMountsSecret(g, rendered[4], "webhook", "server")
	assertWorkloadMountsSecret(g, rendered[5], "other", "other-server")
}

func TestMultipleCertManagerTargetsGenerateIndependentCertificates(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderCertManager),
		WithTarget(Target{
			Service:  k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks: []WebhookReference{{Kind: WebhookKindMutating, Name: "webhook-config"}},
			Certificate: CertificateSource{
				Generated: &CertificateSpec{IssuerRef: IssuerRef{Name: "internal-ca"}},
			},
		}),
		WithTarget(Target{
			Service:  k8stypes.NamespacedName{Namespace: "other-system", Name: "other"},
			Webhooks: []WebhookReference{{Kind: WebhookKindValidating, Name: "other-config"}},
			Certificate: CertificateSource{
				Generated: &CertificateSpec{IssuerRef: IssuerRef{Name: "internal-ca"}},
			},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testObject("v1", "Service", "other-system", "other", nil),
		testWebhookConfiguration("MutatingWebhookConfiguration", "webhook-config", "webhook-system", "webhook"),
		testWebhookConfiguration("ValidatingWebhookConfiguration", "other-config", "other-system", "other"),
	}

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rendered).To(HaveLen(6))

	g.Expect(rendered[2].GetAnnotations()).To(Equal(map[string]string{
		"cert-manager.io/inject-ca-from": "webhook-system/webhook-certificate",
	}))
	g.Expect(rendered[3].GetAnnotations()).To(Equal(map[string]string{
		"cert-manager.io/inject-ca-from": "other-system/other-certificate",
	}))

	assertCertificateForService(g, testFindObject(rendered, "webhook-system", "webhook-certificate"), "webhook-system", "webhook")
	assertCertificateForService(g, testFindObject(rendered, "other-system", "other-certificate"), "other-system", "other")
}
