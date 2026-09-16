package cert

import (
	"fmt"

	k8sutil "github.com/k8s-manifest-kit/pkg/util/k8s"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (r *renderer) configureWorkloads(objects []unstructured.Unstructured, target Target) error {
	selector := target.Workload.Selector
	if selector == nil {
		return nil
	}

	for i := range objects {
		obj := &objects[i]
		if obj.GetNamespace() != target.Service.Namespace || !matchesWorkloadKind(*obj, target.Workload) {
			continue
		}
		if !selector.Matches(labels.Set(labelsFromObject(*obj))) {
			continue
		}
		if err := patchWorkload(obj, target.SecretName, target.Workload); err != nil {
			return fmt.Errorf("%s: %w", k8sutil.DescribeObject(obj), err)
		}
	}

	return nil
}
