package cert

import (
	"sync"
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func TestConcurrentRendersReturnIndependentOutput(t *testing.T) {
	baseTarget := Target{
		Service:  k8stypes.NamespacedName{Namespace: "webhook-system", Name: "webhook"},
		Webhooks: []WebhookReference{{Kind: WebhookKindMutating, Name: "webhook-config"}},
		Workload: WorkloadOptions{Selector: testAppSelector("webhook")},
	}
	certManagerTarget := baseTarget
	certManagerTarget.Certificate = CertificateSource{
		Generated: &CertificateSpec{IssuerRef: IssuerRef{Name: "internal-ca"}},
	}

	for _, tc := range []struct {
		provider Provider
		target   Target
	}{
		{ProviderOpenShift, baseTarget},
		{ProviderCertManager, certManagerTarget},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			g := NewWithT(t)
			postRenderer, err := New(WithProvider(tc.provider), WithTarget(tc.target))
			g.Expect(err).NotTo(HaveOccurred())

			objects := []unstructured.Unstructured{
				testObject("v1", "Service", "webhook-system", "webhook", nil),
				testWebhookConfiguration("MutatingWebhookConfiguration", "webhook-config", "webhook-system", "webhook"),
				testDeployment("webhook-system", "webhook", map[string]any{"app": "webhook"}, map[string]any{"name": "server"}),
			}
			snapshot := testDeepCopyObjects(objects)
			ctx := t.Context()

			const workerCount = 8
			type renderResult struct {
				rendered []unstructured.Unstructured
				err      error
			}

			start := make(chan struct{})
			results := make(chan renderResult, workerCount)

			var workers sync.WaitGroup
			for range workerCount {
				workers.Add(1)
				go func() {
					defer workers.Done()
					<-start
					rendered, renderErr := postRenderer(ctx, objects)
					results <- renderResult{rendered: rendered, err: renderErr}
				}()
			}
			close(start)
			workers.Wait()
			close(results)

			outputs := make([][]unstructured.Unstructured, 0, workerCount)
			for result := range results {
				g.Expect(result.err).NotTo(HaveOccurred())
				outputs = append(outputs, result.rendered)
			}
			g.Expect(outputs).To(HaveLen(workerCount))

			first := outputs[0]
			for _, output := range outputs[1:] {
				g.Expect(output).To(Equal(first))
			}

			repeated, err := postRenderer(ctx, objects)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(repeated).To(Equal(first))

			// Mutating one output through its nested containers must not
			// affect any other output.
			containerList, found, err := unstructured.NestedFieldNoCopy(first[2].Object, "spec", "template", "spec", "containers")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			firstContainer, ok := containerList.([]any)[0].(map[string]any)
			g.Expect(ok).To(BeTrue())
			firstContainer["name"] = "mutated"

			for _, output := range outputs[1:] {
				g.Expect(testContainersByList(output[2], "containers")).To(HaveKey("server"))
			}
			g.Expect(objects).To(Equal(snapshot))
		})
	}
}
