# Development

Build and test with the Go toolchain version required by `go.mod` and the
module Makefile:

```bash
make test
make test/race
make fmt
make lint
make check
```

`make test` runs the unit tests and `make test/race` additionally enables the
race detector. `make check` runs lint and vulncheck; it is not a substitute
for `make test`. Formatting is a separate target (`make fmt`).

Tests stay offline: the post-renderer validates and patches rendered manifests
and does not contact a cluster. There is no live integration target. Tests
use unstructured Kubernetes objects built by the helpers in `_support_test.go`
files, vanilla Gomega assertions, and `t.Context()` where a context is needed.

## Go style

- Declare each parameter type explicitly. Do not group parameters that share
  a type: write `apiVersion string, kind string`, not `apiVersion, kind
  string`.
- Keep the root `pkg` package focused on post-renderer orchestration;
  provider-specific behavior belongs under `pkg/provider/<name>`. Shared
  implementation helpers live in `_support.go` files and test-only helpers in
  `_support_test.go` files.
- Wrap errors with `%w` when a cause exists and check sentinels with
  `errors.Is`, for example `ErrConflict`. Conflict and validation errors must
  name the object, field, or value needed to diagnose the failure.

### Multiline strings

- Declare every multiline string — fixture, manifest, template, or expected
  output — as one descriptive, standalone, package-level `const name =` with a
  raw string literal.
- Never embed multiline string literals in function or method bodies, and
  never declare them as function-local constants.
- Never group multiline constants in a `const (...)` block. Related
  single-line scalar constants can still be grouped, and single-line inline
  strings are unaffected.
- Keep large literal content separate from execution logic for readability.
  Preserve meaningful content indentation; the declaration layout must not
  introduce unintended bytes.
- A function may format the named template into a local variable. The rule
  concerns where the literal is declared, not where its string value is used.

```go
const fixtureYAML = `apiVersion: v1
kind: ConfigMap
`
```

## Mutating unstructured objects

`unstructured.NestedMap` and `unstructured.NestedSlice` return copies of the
nested value. Mutating the returned map or slice is invisible to the rendered
object unless the value is written back with `SetNestedMap` or
`SetNestedSlice`. Persist every intended mutation, and prove in tests that
the rendered target actually changed rather than only asserting that no error
occurred. The regression anchor is
`TestWorkloadReusePersistsReadOnlyVolumeMount` in `pkg/cert_conflict_test.go`:
a reused TLS volume mount that omits `readOnly` is rendered with
`readOnly: true` only because `ensureVolumeMount` writes the mutated mount
list back to the container.

## Testing expectations

- Input safety: rendering must not mutate caller-owned manifests. Snapshot
  the input objects with `testDeepCopyObjects` and compare them after
  rendering.
- Conflicts and idempotency: each conflicting annotation, volume, mount, or
  generated Certificate case needs a test asserting `errors.Is(err,
  ErrConflict)` and unchanged inputs. Compatible existing objects are reused
  and re-rendering the previous output must return an equal result
  (`pkg/cert_conflict_test.go`). Selectors, multi-target isolation, and
  missing-target skipping are covered by `pkg/cert_targets_test.go` and
  `pkg/cert_test.go`.
- Concurrency: a renderer built from the built-in providers may be called by
  overlapping goroutines. Concurrent tests start workers behind a barrier,
  join all workers, and assert on the test goroutine; workers share the
  configured renderer and inputs read-only and own their results
  (`pkg/cert_concurrent_test.go`). This covers the built-in pipeline only:
  the library does not synchronize user-supplied functions such as
  `WorkloadOptions.ContainerSelector`, which must be concurrency-safe when
  the same renderer is used by overlapping calls.
