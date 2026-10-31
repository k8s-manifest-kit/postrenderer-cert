package cert

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// testUnwrapChildren returns the errors wrapped by err through either the
// single-error or the multi-error unwrap form.
func testUnwrapChildren(err error) []error {
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		return multi.Unwrap()
	}
	if single := errors.Unwrap(err); single != nil {
		return []error{single}
	}

	return nil
}

// testFindCauseBeside returns the non-sentinel error wrapped alongside
// sentinel in the innermost multi-cause wrapper of err's chain, or nil when
// the sentinel's wrapper carries no additional wrapped error.
func testFindCauseBeside(err error, sentinel error) error {
	seen := map[error]bool{}

	var visit func(current error) error
	visit = func(current error) error {
		if current == nil || seen[current] {
			return nil
		}
		seen[current] = true

		children := testUnwrapChildren(current)
		hasSentinel := false
		for _, child := range children {
			if errors.Is(child, sentinel) {
				hasSentinel = true
				break
			}
		}
		if hasSentinel {
			for _, child := range children {
				if child != nil && !errors.Is(child, sentinel) {
					return child
				}
			}
		}

		for _, child := range children {
			if cause := visit(child); cause != nil {
				return cause
			}
		}

		return nil
	}

	return visit(err)
}

// TestRenderPreservesWebhookAccessorCause renders a webhook configuration
// whose webhooks value is not a slice. The sentinel and message structure of
// the invalid-configuration error must be preserved, and the accessor error
// reported by the unstructured nested-field access must remain reachable
// through the standard Go error chain instead of only appearing in the text.
func TestRenderPreservesWebhookAccessorCause(t *testing.T) {
	g := NewWithT(t)

	postRenderer, err := New(
		WithProvider(ProviderCertManager),
		WithTarget(Target{
			Service:     k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
			Webhooks:    []WebhookReference{{Kind: WebhookKindMutating, Name: "webhook-config"}},
			Certificate: CertificateSource{Generated: &CertificateSpec{IssuerRef: IssuerRef{Name: "internal-ca"}}},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testObject("admissionregistration.k8s.io/v1", "MutatingWebhookConfiguration", "", "webhook-config", map[string]any{
			"webhooks": "not-a-slice",
		}),
	}

	_, err = postRenderer(t.Context(), objects)
	g.Expect(err).To(HaveOccurred())

	// The public sentinel still matches through the wrapped chain.
	g.Expect(errors.Is(err, ErrInvalidConfiguration)).To(BeTrue())

	// The message structure of the wrapping error is unchanged.
	g.Expect(err.Error()).To(ContainSubstring("has invalid entries"))

	// The actual accessor cause is wrapped beside the sentinel and reachable
	// from the final public error by identity, not only by message text.
	cause := testFindCauseBeside(err, ErrInvalidConfiguration)
	g.Expect(cause).NotTo(BeNil())
	g.Expect(cause.Error()).To(ContainSubstring("webhooks accessor error"))
	g.Expect(errors.Is(err, cause)).To(BeTrue())
}
