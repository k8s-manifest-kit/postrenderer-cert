package provider

import (
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	// ErrInvalidConfiguration indicates invalid certificate configuration.
	ErrInvalidConfiguration = errors.New("invalid certificate configuration")
	// ErrConflict indicates a present manifest conflicts with requested state.
	ErrConflict = errors.New("certificate configuration conflict")
)

// Provider applies provider-specific certificate configuration to a manifest
// batch. Implementations are intentionally small; the parent post-renderer
// owns target selection and pipeline orchestration.
type Provider interface {
	ConfigureService(*unstructured.Unstructured, Target) error
	ConfigureWebhooks([]unstructured.Unstructured, Target) error
	EnsureCertificate([]unstructured.Unstructured, Target) ([]unstructured.Unstructured, error)
}
