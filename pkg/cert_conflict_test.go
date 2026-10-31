package cert

import (
	"errors"
	"slices"
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCertManagerRejectsConflictingExistingCertificates(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderCertManager),
		WithTarget(testGeneratedCertificateTarget()),
	)
	g.Expect(err).NotTo(HaveOccurred())

	for _, tc := range []struct {
		name   string
		mutate func(spec map[string]any)
	}{
		{
			name:   "secret name",
			mutate: func(spec map[string]any) { spec["secretName"] = "other-secret" },
		},
		{
			name:   "issuer name",
			mutate: func(spec map[string]any) { testSetIssuerRefField(spec, "name", "other-ca") },
		},
		{
			name:   "issuer kind",
			mutate: func(spec map[string]any) { testSetIssuerRefField(spec, "kind", "ClusterIssuer") },
		},
		{
			name:   "issuer group",
			mutate: func(spec map[string]any) { testSetIssuerRefField(spec, "group", "other.example") },
		},
		{
			name:   "dns names",
			mutate: func(spec map[string]any) { spec["dnsNames"] = []any{"webhook.webhook-system.svc"} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			spec := testGeneratedCertificateSpec()
			tc.mutate(spec)

			objects := []unstructured.Unstructured{
				testObject("v1", "Service", "webhook-system", "webhook", nil),
				testCertificate("webhook-system", "webhook-certificate", spec),
			}
			snapshot := testDeepCopyObjects(objects)

			_, err := postRenderer(t.Context(), objects)
			g.Expect(err).To(HaveOccurred())
			g.Expect(errors.Is(err, ErrConflict)).To(BeTrue())
			g.Expect(objects).To(Equal(snapshot))
		})
	}
}

func TestCertManagerReusesCompatibleExistingCertificate(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderCertManager),
		WithTarget(testGeneratedCertificateTarget()),
	)
	g.Expect(err).NotTo(HaveOccurred())

	// The DNS names are deliberately in a different order than requested to
	// prove compatibility is set-based.
	spec := testGeneratedCertificateSpec()
	slices.Reverse(spec["dnsNames"].([]any))

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testCertificate("webhook-system", "webhook-certificate", spec),
	}
	snapshot := testDeepCopyObjects(objects)

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rendered).To(HaveLen(2))
	g.Expect(rendered[1]).To(Equal(objects[1]))
	g.Expect(objects).To(Equal(snapshot))

	secondRender, err := postRenderer(t.Context(), rendered)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secondRender).To(Equal(rendered))
}
