// Package cert provides a certificate-management post-renderer for Kubernetes
// webhook-serving resources.
package cert

import (
	"context"
	"fmt"

	enginetypes "github.com/k8s-manifest-kit/engine/pkg/types"
	providerconfig "github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	defaultMountPath   = "/tls"
	defaultVolumeName  = "webhook-tls"
	defaultIssuerGroup = "cert-manager.io"
	defaultIssuerKind  = "Issuer"
	serviceAPIVersion  = "v1"
	serviceKind        = "Service"
)

var (
	// ErrInvalidConfiguration indicates invalid constructor options.
	ErrInvalidConfiguration = providerconfig.ErrInvalidConfiguration
	// ErrConflict indicates a present manifest conflicts with requested state.
	ErrConflict = providerconfig.ErrConflict
)

type renderer struct {
	provider certificateProvider
	targets  []Target
}

// New creates a certificate-management post-renderer.
func New(opts ...Option) (enginetypes.PostRenderer, error) {
	options := Options{}
	for _, opt := range opts {
		if opt != nil {
			opt.ApplyTo(&options)
		}
	}

	if err := validateOptions(options); err != nil {
		return nil, err
	}

	targets := make([]Target, len(options.Targets))
	for i := range options.Targets {
		targets[i] = normalizeTarget(options.Targets[i])
	}
	provider, err := newCertificateProvider(options.Provider)
	if err != nil {
		return nil, err
	}

	r := &renderer{
		provider: provider,
		targets:  targets,
	}

	return r.Render, nil
}

// Render implements types.PostRenderer.
func (r *renderer) Render(ctx context.Context, objects []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, len(objects))
	for i := range objects {
		result[i] = *objects[i].DeepCopy()
	}

	for targetIndex, target := range r.targets {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("certificate target %d cancelled: %w", targetIndex, err)
		}

		serviceIndex := findObject(result, serviceAPIVersion, serviceKind, target.Service)
		if serviceIndex < 0 {
			continue
		}

		if err := r.provider.ConfigureService(&result[serviceIndex], target); err != nil {
			return nil, fmt.Errorf("target %s/%s: %w", target.Service.Namespace, target.Service.Name, err)
		}

		if err := r.configureWorkloads(result, target); err != nil {
			return nil, fmt.Errorf("target %s/%s workload: %w", target.Service.Namespace, target.Service.Name, err)
		}

		if err := r.provider.ConfigureWebhooks(result, target); err != nil {
			return nil, fmt.Errorf("target %s/%s webhooks: %w", target.Service.Namespace, target.Service.Name, err)
		}

		var err error
		result, err = r.provider.EnsureCertificate(result, target)
		if err != nil {
			return nil, fmt.Errorf("target %s/%s certificate: %w", target.Service.Namespace, target.Service.Name, err)
		}
	}

	return result, nil
}
