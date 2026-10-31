package cert

import (
	"github.com/k8s-manifest-kit/pkg/util/option"
	providerconfig "github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Provider identifies the controller that owns certificate configuration.
type Provider = providerconfig.Type

const (
	// ProviderOpenShift uses the OpenShift Service CA annotations.
	ProviderOpenShift = providerconfig.OpenShift
	// ProviderCertManager uses cert-manager Certificate and cainjector resources.
	ProviderCertManager = providerconfig.CertManager
)

type WebhookKind = providerconfig.WebhookKind

const (
	WebhookKindMutating   = providerconfig.WebhookKindMutating
	WebhookKindValidating = providerconfig.WebhookKindValidating
)

type WebhookReference = providerconfig.WebhookReference
type IssuerRef = providerconfig.IssuerRef
type CertificateSpec = providerconfig.CertificateSpec
type CertificateSource = providerconfig.CertificateSource
type WorkloadOptions = providerconfig.WorkloadOptions
type Target = providerconfig.Target

// Options configures the certificate post-renderer.
type Options struct {
	Provider Provider
	Targets  []Target
}

// ApplyTo implements option.Option for Options.
func (opts Options) ApplyTo(target *Options) {
	if opts.Provider != "" {
		target.Provider = opts.Provider
	}
	target.Targets = append(target.Targets, opts.Targets...)
}

// Option configures the certificate post-renderer.
type Option = option.Option[Options]

// WithProvider selects the certificate-management provider.
func WithProvider(provider Provider) Option {
	return option.FunctionalOption[Options](func(opts *Options) {
		opts.Provider = provider
	})
}

// WithTarget adds a target to the post-renderer.
func WithTarget(target Target) Option {
	return option.FunctionalOption[Options](func(opts *Options) {
		opts.Targets = append(opts.Targets, target)
	})
}

// DefaultWorkloadKinds returns the default supported pod-template workload kinds.
func DefaultWorkloadKinds() []schema.GroupKind {
	return providerconfig.DefaultWorkloadKinds()
}
