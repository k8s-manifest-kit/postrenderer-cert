# Agent Guide: postrenderer-cert

`postrenderer-cert` configures webhook-serving certificates in rendered
Kubernetes manifests for OpenShift Service CA or cert-manager.

The public package is imported from
`github.com/k8s-manifest-kit/postrenderer-cert/pkg` as `cert` and returns the
existing `engine/pkg/types.PostRenderer` function type.

The post-renderer operates on the final batch after filters and transformers.
Missing targets are allowed because upstream filters may intentionally remove
resources. Conflicting configuration on present resources must return an error.

Provider implementations live under `pkg/provider/openshift` and
`pkg/provider/certmanager`; keep the root package focused on orchestration and
keep shared helper functions in `_support.go` files.

Run commands from this directory:

```bash
make test
make test/race
make fmt
make lint
make check
```

`make check` runs lint and vulncheck; it is not a substitute for `make test`.
Contributor conventions are in [docs/development.md](docs/development.md) and
architecture rationale is in [docs/design.md](docs/design.md).

Use unstructured Kubernetes objects in tests, Gomega assertions, and
`t.Context()` where a test needs a context. Keep tests focused and simple:
use the smallest manifest fixture that proves the behavior, and add helpers
or extra test files only when a concrete behavior or regression requires
them.

Declare parameter types explicitly rather than grouping same-typed parameters;
write `apiVersion string, kind string`, not `apiVersion, kind string`.
