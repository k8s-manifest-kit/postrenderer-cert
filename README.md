# Certificate Post-Renderer

`postrenderer-cert` configures webhook-serving certificates in a rendered
manifest batch. It supports OpenShift Service CA annotations and cert-manager
Certificate/cainjector configuration, including serving Secret mounts on
Deployments, StatefulSets, and DaemonSets.

```go
import (
	cert "github.com/k8s-manifest-kit/postrenderer-cert/pkg"
	engine "github.com/k8s-manifest-kit/engine/pkg"

	"k8s.io/apimachinery/pkg/types"
)

renderer, err := cert.New(
	cert.WithProvider(cert.ProviderCertManager),
	cert.WithTarget(cert.Target{
		Service: types.NamespacedName{
			Namespace: "webhook-system",
			Name:      "webhook",
		},
		Webhooks: []cert.WebhookReference{
			{Kind: cert.WebhookKindMutating, Name: "webhook"},
		},
		Certificate: cert.CertificateSource{
			Generated: &cert.CertificateSpec{
				IssuerRef: cert.IssuerRef{
					Name: "internal-ca",
					Kind: "ClusterIssuer",
				},
			},
		},
	}),
)
if err != nil {
	return err
}

// sourceRenderer is another configured engine renderer.
engine, err := engine.New(
	engine.WithRenderer(sourceRenderer),
	engine.WithPostRenderer(renderer),
)
```

The post-renderer is presence-driven: resources removed by earlier filters are
skipped. Present resources with conflicting certificate configuration fail.

See [`docs/design.md`](docs/design.md) for provider behavior and
[`AGENTS.md`](AGENTS.md) for development conventions.
