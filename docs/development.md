# Development

Use Go 1.26.8 and the module Makefile:

```bash
make test
make fmt
make lint
make check
```

Tests use `testing/fstest` only when filesystem fixtures are needed, vanilla
Gomega assertions, and unstructured Kubernetes objects. Keep tests offline;
the post-renderer validates manifests and does not contact a cluster.

## Go style

Declare each parameter type explicitly. Do not group parameters that share a
type: write `apiVersion string, kind string`, not `apiVersion, kind string`.
