package cert

import (
	"fmt"
	"slices"
	"strings"

	providerconfig "github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider"
	"github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider/certmanager"
	"github.com/k8s-manifest-kit/postrenderer-cert/pkg/provider/openshift"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

type certificateProvider = providerconfig.Provider

func newCertificateProvider(providerType Provider) (certificateProvider, error) {
	switch providerType {
	case ProviderOpenShift:
		return openshift.New(), nil
	case ProviderCertManager:
		return certmanager.New(), nil
	default:
		return nil, fmt.Errorf("%w: unsupported provider %q", ErrInvalidConfiguration, providerType)
	}
}

func validateOptions(options Options) error {
	switch options.Provider {
	case ProviderOpenShift, ProviderCertManager:
	default:
		return fmt.Errorf("%w: unsupported provider %q", ErrInvalidConfiguration, options.Provider)
	}
	if len(options.Targets) == 0 {
		return fmt.Errorf("%w: at least one target is required", ErrInvalidConfiguration)
	}

	for i, target := range options.Targets {
		if strings.TrimSpace(target.Service.Name) == "" || strings.TrimSpace(target.Service.Namespace) == "" {
			return fmt.Errorf("%w: target %d requires a Service namespace and name", ErrInvalidConfiguration, i)
		}
		for webhookIndex, webhook := range target.Webhooks {
			if webhook.Kind != WebhookKindMutating && webhook.Kind != WebhookKindValidating {
				return fmt.Errorf("%w: target %d webhook %d has unsupported kind %q", ErrInvalidConfiguration, i, webhookIndex, webhook.Kind)
			}
			if strings.TrimSpace(webhook.Name) == "" {
				return fmt.Errorf("%w: target %d webhook %d requires a name", ErrInvalidConfiguration, i, webhookIndex)
			}
		}

		certificateConfigured := target.Certificate.ExistingRef != nil || target.Certificate.Generated != nil
		switch options.Provider {
		case ProviderOpenShift:
			if certificateConfigured {
				return fmt.Errorf("%w: target %d cannot configure a cert-manager Certificate for OpenShift", ErrInvalidConfiguration, i)
			}
		case ProviderCertManager:
			if !certificateConfigured {
				return fmt.Errorf("%w: target %d requires an existing or generated Certificate", ErrInvalidConfiguration, i)
			}
			if target.Certificate.ExistingRef != nil && target.Certificate.Generated != nil {
				return fmt.Errorf("%w: target %d cannot configure both an existing and generated Certificate", ErrInvalidConfiguration, i)
			}
			if target.Certificate.ExistingRef != nil && strings.TrimSpace(target.Certificate.ExistingRef.Name) == "" {
				return fmt.Errorf("%w: target %d existing Certificate requires a name", ErrInvalidConfiguration, i)
			}
			if target.Certificate.Generated != nil && strings.TrimSpace(target.Certificate.Generated.IssuerRef.Name) == "" {
				return fmt.Errorf("%w: target %d generated Certificate requires an issuer name", ErrInvalidConfiguration, i)
			}
		}
	}

	return nil
}

func normalizeTarget(target Target) Target {
	target.Webhooks = slices.Clone(target.Webhooks)
	for i := range target.Webhooks {
		target.Webhooks[i].EntryNames = slices.Clone(target.Webhooks[i].EntryNames)
	}
	target.SecretName = strings.TrimSpace(target.SecretName)
	if target.SecretName == "" {
		target.SecretName = target.Service.Name
	}

	target.Workload.Kinds = slices.Clone(target.Workload.Kinds)
	if len(target.Workload.Kinds) == 0 {
		target.Workload.Kinds = DefaultWorkloadKinds()
	}
	if target.Workload.MountPath == "" {
		target.Workload.MountPath = defaultMountPath
	}
	if target.Workload.VolumeName == "" {
		target.Workload.VolumeName = defaultVolumeName
	}

	if target.Certificate.ExistingRef != nil {
		ref := *target.Certificate.ExistingRef
		if ref.Namespace == "" {
			ref.Namespace = target.Service.Namespace
		}
		target.Certificate.ExistingRef = &ref
	}
	if target.Certificate.Generated != nil {
		generated := *target.Certificate.Generated
		generated.DNSNames = slices.Clone(generated.DNSNames)
		if generated.Name == "" {
			generated.Name = target.Service.Name + "-certificate"
		}
		if generated.IssuerRef.Group == "" {
			generated.IssuerRef.Group = defaultIssuerGroup
		}
		if generated.IssuerRef.Kind == "" {
			generated.IssuerRef.Kind = defaultIssuerKind
		}
		target.Certificate.Generated = &generated
	}

	return target
}

func findObject(objects []unstructured.Unstructured, apiVersion string, kind string, ref k8stypes.NamespacedName) int {
	for i := range objects {
		if objects[i].GetAPIVersion() == apiVersion &&
			objects[i].GetKind() == kind &&
			objects[i].GetNamespace() == ref.Namespace &&
			objects[i].GetName() == ref.Name {
			return i
		}
	}

	return -1
}
