package cert

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadRejectsConflictingVolumesAndMounts(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(testWorkloadTarget(nil)),
	)
	g.Expect(err).NotTo(HaveOccurred())

	for _, tc := range []struct {
		name    string
		podSpec map[string]any
	}{
		{
			name: "non-Secret volume with the TLS volume name",
			podSpec: map[string]any{
				"containers": []any{testContainer("server")},
				"volumes":    []any{testConfigMapVolume("webhook-tls", "settings")},
			},
		},
		{
			name: "Secret volume with the wrong Secret",
			podSpec: map[string]any{
				"containers": []any{testContainer("server")},
				"volumes":    []any{testSecretVolume("webhook-tls", "other-secret")},
			},
		},
		{
			name: "volume mount with the TLS name and a different path",
			podSpec: map[string]any{
				"containers": []any{testContainer("server", testVolumeMount("webhook-tls", "/other"))},
			},
		},
		{
			name: "volume mount with the TLS path and a different name",
			podSpec: map[string]any{
				"containers": []any{testContainer("server", testVolumeMount("other-volume", "/tls"))},
			},
		},
		{
			name: "writable TLS volume mount",
			podSpec: map[string]any{
				"containers": []any{testContainer("server", testWritableVolumeMount("webhook-tls", "/tls"))},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			objects := []unstructured.Unstructured{
				testObject("v1", "Service", "webhook-system", "webhook", nil),
				testDeploymentWithPodSpec("webhook-system", "webhook", map[string]any{"app": "webhook"}, tc.podSpec),
			}
			snapshot := testDeepCopyObjects(objects)

			_, err := postRenderer(t.Context(), objects)
			g.Expect(err).To(HaveOccurred())
			g.Expect(errors.Is(err, ErrConflict)).To(BeTrue())
			g.Expect(objects).To(Equal(snapshot))
		})
	}
}

func TestWorkloadReusesCompatibleVolumeAndMount(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(testWorkloadTarget(nil)),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testDeploymentWithPodSpec("webhook-system", "webhook", map[string]any{"app": "webhook"}, map[string]any{
			"containers": []any{testContainer("server", testReadOnlyVolumeMount("webhook-tls", "/tls"))},
			"volumes":    []any{testSecretVolume("webhook-tls", "webhook")},
		}),
	}
	snapshot := testDeepCopyObjects(objects)

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(Equal(snapshot))

	deployment := rendered[1]
	volumes, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "volumes")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(volumes).To(Equal([]any{testSecretVolume("webhook-tls", "webhook")}))
	mounts, found := testContainerMounts(deployment, "containers", "server")
	g.Expect(found).To(BeTrue())
	g.Expect(mounts).To(Equal([]any{testReadOnlyVolumeMount("webhook-tls", "/tls")}))

	secondRender, err := postRenderer(t.Context(), rendered)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secondRender).To(Equal(rendered))
}

// TestWorkloadReusePersistsReadOnlyVolumeMount covers a pre-existing
// compatible TLS volume mount that omits readOnly: the reused mount must be
// rendered with readOnly: true, for every patched container, through the
// public post-render path. A container selector limits the patch to the
// selected container, whose mount is still persisted read-only.
func TestWorkloadReusePersistsReadOnlyVolumeMount(t *testing.T) {
	for _, tc := range []struct {
		name              string
		containerSelector func(name string) bool
	}{
		{
			name:              "all containers",
			containerSelector: nil,
		},
		{
			name:              "selected containers",
			containerSelector: func(name string) bool { return name == "server" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			postRenderer, err := New(
				WithProvider(ProviderOpenShift),
				WithTarget(testWorkloadTarget(tc.containerSelector)),
			)
			g.Expect(err).NotTo(HaveOccurred())

			objects := []unstructured.Unstructured{
				testObject("v1", "Service", "webhook-system", "webhook", nil),
				testDeploymentWithPodSpec("webhook-system", "webhook", map[string]any{"app": "webhook"}, map[string]any{
					"containers":     []any{testContainer("server", testVolumeMount("webhook-tls", "/tls")), testContainer("sidecar", testVolumeMount("webhook-tls", "/tls"))},
					"initContainers": []any{testContainer("setup", testVolumeMount("webhook-tls", "/tls"))},
					"volumes":        []any{testSecretVolume("webhook-tls", "webhook")},
				}),
			}
			snapshot := testDeepCopyObjects(objects)

			rendered, err := postRenderer(t.Context(), objects)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(objects).To(Equal(snapshot))

			deployment := rendered[1]
			mounts, found := testContainerMounts(deployment, "containers", "server")
			g.Expect(found).To(BeTrue())
			g.Expect(mounts).To(Equal([]any{testReadOnlyVolumeMount("webhook-tls", "/tls")}))

			expectedUnpatched := []any{testVolumeMount("webhook-tls", "/tls")}
			if tc.containerSelector == nil {
				expectedUnpatched = []any{testReadOnlyVolumeMount("webhook-tls", "/tls")}
			}
			mounts, found = testContainerMounts(deployment, "containers", "sidecar")
			g.Expect(found).To(BeTrue())
			g.Expect(mounts).To(Equal(expectedUnpatched))
			mounts, found = testContainerMounts(deployment, "initContainers", "setup")
			g.Expect(found).To(BeTrue())
			g.Expect(mounts).To(Equal(expectedUnpatched))

			volumes, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "volumes")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(volumes).To(Equal([]any{testSecretVolume("webhook-tls", "webhook")}))

			secondRender, err := postRenderer(t.Context(), rendered)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(secondRender).To(Equal(rendered))
		})
	}
}

func TestContainerSelectorSubsetPatchesOnlySelectedContainers(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(testWorkloadTarget(func(name string) bool { return name == "server" })),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testDeploymentWithPodSpec("webhook-system", "webhook", map[string]any{"app": "webhook"}, map[string]any{
			"containers":     []any{testContainer("server"), testContainer("sidecar")},
			"initContainers": []any{testContainer("setup")},
		}),
	}

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())

	deployment := rendered[1]
	mounts, found := testContainerMounts(deployment, "containers", "server")
	g.Expect(found).To(BeTrue())
	g.Expect(mounts).To(Equal([]any{testReadOnlyVolumeMount("webhook-tls", "/tls")}))
	_, found = testContainerMounts(deployment, "containers", "sidecar")
	g.Expect(found).To(BeFalse())
	_, found = testContainerMounts(deployment, "initContainers", "setup")
	g.Expect(found).To(BeFalse())

	volumes, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "volumes")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(volumes).To(Equal([]any{testSecretVolume("webhook-tls", "webhook")}))
}

func TestContainerSelectorWithoutMatchesFallsBackToAllContainers(t *testing.T) {
	g := NewWithT(t)
	postRenderer, err := New(
		WithProvider(ProviderOpenShift),
		WithTarget(testWorkloadTarget(func(name string) bool { return name == "missing" })),
	)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []unstructured.Unstructured{
		testObject("v1", "Service", "webhook-system", "webhook", nil),
		testDeploymentWithPodSpec("webhook-system", "webhook", map[string]any{"app": "webhook"}, map[string]any{
			"containers":     []any{testContainer("server"), testContainer("sidecar")},
			"initContainers": []any{testContainer("setup")},
		}),
	}

	rendered, err := postRenderer(t.Context(), objects)
	g.Expect(err).NotTo(HaveOccurred())

	deployment := rendered[1]
	for _, containerName := range []string{"server", "sidecar"} {
		mounts, found := testContainerMounts(deployment, "containers", containerName)
		g.Expect(found).To(BeTrue())
		g.Expect(mounts).To(Equal([]any{testReadOnlyVolumeMount("webhook-tls", "/tls")}))
	}
	mounts, found := testContainerMounts(deployment, "initContainers", "setup")
	g.Expect(found).To(BeTrue())
	g.Expect(mounts).To(Equal([]any{testReadOnlyVolumeMount("webhook-tls", "/tls")}))
}
