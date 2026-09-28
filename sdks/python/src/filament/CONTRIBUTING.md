# Contributing to the Filament Python SDK

The Python SDK is maintained in the
[Filament repository](https://github.com/galaxy-io/filament). Report SDK issues
and open pull requests there.

## Making changes

The client and models are generated. Change request and response definitions
in `protos/`, and configure SDK names, authentication, and generator options
in `fern/`. Regenerate the SDK instead of editing generated Python files.

Package metadata, the [README](../../README.md), examples, and interoperability
tests are maintained separately under `sdks/python/`. This contributing guide
is protected from regeneration by the adjacent `.fernignore`.

## Setup and generation

Use Python 3.10 or newer and uv. Regeneration also requires the repository's
Buf/protobuf toolchain, just, and the Fern CLI. Sign in to Fern with an account
in the `getgalaxy` organization.

Run these commands from the Filament repository root:

```sh
uv sync --project sdks/python
just sdks
```

`just sdks` regenerates the OpenAPI specification from protobuf, validates the
Fern configuration, and generates the SDK. Include the relevant source changes
and regenerated files in your pull request.

## Validation

Build the wheel and source distribution:

```sh
uv build --no-sources --project sdks/python
```

For changes to SDK behavior, run the interoperability tests. These require the
Go version specified in `go.mod` and start an ephemeral Filament API server:

```sh
FILAMENT_SDK_PYTHON="$PWD/sdks/python/.venv/bin/python" GOWORK=off \
  go test ./sdks/python/tests -run TestPythonSDK -count=1
```

Explain the change and the checks you ran in your pull request. See the
[Filament contribution guide](../../../../.github/CONTRIBUTING.md) for repository
setup and pull request guidelines.
