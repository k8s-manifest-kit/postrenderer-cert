package cert

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func matchesWorkloadKind(obj unstructured.Unstructured, options WorkloadOptions) bool {
	groupKind := obj.GroupVersionKind().GroupKind()
	for _, allowed := range options.Kinds {
		if groupKind == allowed {
			return true
		}
	}

	return false
}

func labelsFromObject(obj unstructured.Unstructured) map[string]string {
	labels := obj.GetLabels()
	if labels == nil {
		return map[string]string{}
	}

	return labels
}

func patchWorkload(obj *unstructured.Unstructured, secretName string, options WorkloadOptions) error {
	template, found, err := unstructured.NestedMap(obj.Object, "spec", "template")
	if err != nil || !found {
		return fmt.Errorf("%w: workload has no pod template", ErrConflict)
	}
	podSpec, found, err := unstructured.NestedMap(template, "spec")
	if err != nil || !found {
		return fmt.Errorf("%w: workload pod template has no spec", ErrConflict)
	}

	containers, found, err := unstructured.NestedSlice(podSpec, "containers")
	if err != nil || !found || len(containers) == 0 {
		return fmt.Errorf("%w: workload has no containers", ErrConflict)
	}
	initContainers, _, err := unstructured.NestedSlice(podSpec, "initContainers")
	if err != nil {
		return fmt.Errorf("%w: workload initContainers are malformed", ErrConflict)
	}

	selectAll := options.ContainerSelector == nil
	if !selectAll {
		selectAll = !containsSelectedContainer(containers, initContainers, options.ContainerSelector)
	}

	volumes, _, err := unstructured.NestedSlice(podSpec, "volumes")
	if err != nil {
		return fmt.Errorf("%w: workload volumes are malformed", ErrConflict)
	}
	volumes, err = ensureSecretVolume(volumes, options.VolumeName, secretName)
	if err != nil {
		return err
	}

	if err := patchContainerList(containers, options, selectAll, options.VolumeName, options.MountPath); err != nil {
		return err
	}
	if err := patchContainerList(initContainers, options, selectAll, options.VolumeName, options.MountPath); err != nil {
		return err
	}

	if err := unstructured.SetNestedSlice(podSpec, volumes, "volumes"); err != nil {
		return fmt.Errorf("%w: set workload volumes: %v", ErrConflict, err)
	}
	if err := unstructured.SetNestedSlice(podSpec, containers, "containers"); err != nil {
		return fmt.Errorf("%w: set workload containers: %v", ErrConflict, err)
	}
	if len(initContainers) > 0 {
		if err := unstructured.SetNestedSlice(podSpec, initContainers, "initContainers"); err != nil {
			return fmt.Errorf("%w: set workload initContainers: %v", ErrConflict, err)
		}
	}
	if err := unstructured.SetNestedMap(template, podSpec, "spec"); err != nil {
		return fmt.Errorf("%w: set workload pod spec: %v", ErrConflict, err)
	}
	if err := unstructured.SetNestedMap(obj.Object, template, "spec", "template"); err != nil {
		return fmt.Errorf("%w: set workload pod template: %v", ErrConflict, err)
	}

	return nil
}

func containsSelectedContainer(containers []any, initContainers []any, selector func(string) bool) bool {
	for _, list := range [][]any{containers, initContainers} {
		for _, raw := range list {
			container, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name, _, _ := unstructured.NestedString(container, "name")
			if selector(name) {
				return true
			}
		}
	}

	return false
}

func ensureSecretVolume(volumes []any, volumeName string, secretName string) ([]any, error) {
	for _, raw := range volumes {
		volume, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: malformed volume", ErrConflict)
		}
		name, _, _ := unstructured.NestedString(volume, "name")
		if name != volumeName {
			continue
		}
		secret, found, err := unstructured.NestedMap(volume, "secret")
		if err != nil || !found {
			return nil, fmt.Errorf("%w: volume %q is not a Secret volume", ErrConflict, volumeName)
		}
		actual, _, _ := unstructured.NestedString(secret, "secretName")
		if actual != secretName {
			return nil, fmt.Errorf("%w: volume %q uses Secret %q, requested %q", ErrConflict, volumeName, actual, secretName)
		}

		return volumes, nil
	}

	return append(volumes, map[string]any{
		"name": volumeName,
		"secret": map[string]any{
			"secretName": secretName,
		},
	}), nil
}

func patchContainerList(containers []any, options WorkloadOptions, selectAll bool, volumeName string, mountPath string) error {
	for _, raw := range containers {
		container, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: malformed container", ErrConflict)
		}
		name, _, _ := unstructured.NestedString(container, "name")
		if !selectAll && !options.ContainerSelector(name) {
			continue
		}
		if err := ensureVolumeMount(container, volumeName, mountPath); err != nil {
			return fmt.Errorf("container %q: %w", name, err)
		}
	}

	return nil
}

func ensureVolumeMount(container map[string]any, volumeName string, mountPath string) error {
	mounts, _, err := unstructured.NestedSlice(container, "volumeMounts")
	if err != nil {
		return fmt.Errorf("%w: malformed volumeMounts", ErrConflict)
	}
	for _, raw := range mounts {
		mount, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: malformed volume mount", ErrConflict)
		}
		name, _, _ := unstructured.NestedString(mount, "name")
		path, _, _ := unstructured.NestedString(mount, "mountPath")
		if name != volumeName && path != mountPath {
			continue
		}
		if name != volumeName || path != mountPath {
			return fmt.Errorf("%w: volume mount %q/%q conflicts with requested %q/%q", ErrConflict, name, path, volumeName, mountPath)
		}
		readOnly, found, err := unstructured.NestedBool(mount, "readOnly")
		if err != nil || (found && !readOnly) {
			return fmt.Errorf("%w: volume mount %q must be read-only", ErrConflict, volumeName)
		}
		if !found {
			mount["readOnly"] = true
		}
		return nil
	}

	return unstructured.SetNestedSlice(container, append(mounts, map[string]any{
		"name":      volumeName,
		"mountPath": mountPath,
		"readOnly":  true,
	}), "volumeMounts")
}
