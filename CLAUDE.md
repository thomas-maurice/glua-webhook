# Claude Code Instructions for glua-webhook

This file contains instructions for Claude Code when working on this repository.

## Project Overview

glua-webhook is a Kubernetes admission webhook that executes Lua scripts stored in ConfigMaps to mutate and validate Kubernetes resources. It's based on `github.com/thomas-maurice/glua` and supports both MutatingAdmissionWebhook and ValidatingAdmissionWebhook configurations.

## Code Style

- Use standard Go comment style: `<funcName>: <description>[optionally \n<example usage>]`
- Comment all non-trivial operations
- Write unit tests for any new function added
- **DO NOT use emojis** in code or documentation
- Follow Go conventions: `gofmt` before committing
- Keep functions focused and under 50 lines when possible
- Use descriptive variable names (no single-letter vars except iterators)

## Testing Requirements

### Before ANY Commit

**CRITICAL**: You MUST run these commands before committing:

```bash
make test          # Run all unit tests with race detection
make fmt           # Format all Go code
```

### Test Coverage Requirements

- **Overall coverage**: Must remain >70% for each package
- **Current coverage**:
  - `pkg/luarunner`: 90.3%
  - `pkg/scriptloader`: 98.0%
  - `pkg/webhook`: 82.2%
- **New features**: Must include unit tests achieving >70% coverage
- **Script tests**: All example Lua scripts must have corresponding tests in `test/script_test.go`

### Test Commands

```bash
make test                  # Unit tests (REQUIRED before commit)
make test-scripts          # Lua script tests
make test-integration      # Kind-based integration tests (slower)
make test-all             # All tests (unit + integration)
make coverage             # Generate coverage report
```

### Integration Tests

- Integration tests use Kind clusters
- Run `make test-integration` before pushing major changes
- Requires Kind and kubectl installed
- May take 2-5 minutes to complete

## Documentation Requirements

**CRITICAL**: Documentation MUST be updated when making significant changes.

### When to Update Documentation

Update documentation when you:

1. **Add new features** → Update README.md and relevant docs/ pages
2. **Change behavior** → Update affected documentation sections
3. **Add/modify examples** → Update README.md "Real-World Examples" section
4. **Change annotations** → Update docs/reference/annotations.md
5. **Add glua modules** → Update docs/guides/writing-scripts.md
6. **Change configuration** → Update docs/reference/configuration.md
7. **Fix bugs** → Update troubleshooting sections if applicable

### Documentation Files

| File | Purpose | When to Update |
|------|---------|----------------|
| `README.md` | Main documentation, examples, quickstart | New features, examples, behavior changes |
| `docs/index.md` | Documentation homepage | Major architectural changes |
| `docs/getting-started/installation.md` | Installation guide | Deployment changes, prerequisites |
| `docs/guides/writing-scripts.md` | Lua script guide | New modules, patterns, best practices |
| `docs/guides/type-stubs.md` | IDE support guide | TypeRegistry changes |
| `docs/reference/annotations.md` | Annotation reference | New/changed annotations |
| `docs/examples/` | Example documentation | New examples |

### Example Update Checklist

When adding a new example script:

- [ ] Create script in `examples/scripts/`
- [ ] Add ConfigMap in `examples/manifests/01-configmaps.yaml`
- [ ] Create test in `test/script_test.go`
- [ ] Add to README.md "Real-World Examples" section
- [ ] Describe what it DOES, not just what it is
- [ ] Show before/after comparison if applicable

## Workflow

### Development Workflow

1. **Make changes** to code
2. **Add tests** for new functionality
3. **Run tests**: `make test` (MUST pass)
4. **Update documentation** if behavior changed
5. **Format code**: `make fmt`
6. **Commit** with descriptive message

### Pre-Commit Checklist

Before committing, verify:

- [ ] `make test` passes (ALL tests)
- [ ] Test coverage >70% for changed packages
- [ ] `make fmt` applied
- [ ] Documentation updated if needed
- [ ] Example scripts tested if modified
- [ ] No emojis in code or docs
- [ ] Commit message is descriptive

### Commit Message Format

Use descriptive commit messages:

```
<area>: <brief description>

<detailed description if needed>

- What changed
- Why it changed
- Impact on users (if applicable)
```

Examples:
- `webhook: add support for validating webhooks`
- `luarunner: load all glua modules including fs`
- `docs: add real-world examples section to README`
- `test: increase coverage for scriptloader to 98%`

## Project Structure

```
├── cmd/webhook/          # Main webhook server binary
├── pkg/
│   ├── luarunner/       # Lua script execution with TypeRegistry
│   ├── scriptloader/    # ConfigMap script loader
│   └── webhook/         # HTTP handlers (mutating + validating)
├── test/
│   ├── integration/     # Kind-based integration tests
│   └── script_test.go   # Lua script tests
├── examples/
│   ├── manifests/       # Kubernetes manifests
│   └── scripts/         # Example Lua scripts
├── docs/                # Documentation website
├── Dockerfile           # Multi-stage build
└── Makefile            # Build and test targets
```

## Key Design Decisions

### Script Execution

- **One VM per script**: Each script gets isolated gopher-lua VM
- **Sequential execution**: Scripts run in alphabetical order
- **Error handling**: Failed scripts logged but don't block admission (`failurePolicy: Ignore`)
- **State passing**: Output of script N becomes input of script N+1

### ConfigMap Format

- ConfigMaps MUST have `script.lua` key
- One script per ConfigMap (not multiple scripts)
- Referenced via annotation: `glua.maurice.fr/scripts: "namespace/name"`
- Scripts sorted alphabetically by full reference (`namespace/name`)

### TypeRegistry

- Automatically registers K8s objects for IDE support
- Uses glua TypeRegistry for stub generation
- Best-effort registration (errors logged but not fatal)
- Enables LSP autocompletion for Lua scripts

## Common Tasks

### Adding a New Glua Module

1. Import module in `pkg/luarunner/runner.go`
2. Add to `loadModules()` function
3. Update docs/guides/writing-scripts.md
4. Add example usage to README if significant
5. Test with `make test`

### Adding a New Example Script

1. Create script in `examples/scripts/<name>.lua`
2. Add ConfigMap in `examples/manifests/01-configmaps.yaml`
3. Create test in `test/script_test.go`
4. Add to README.md "Real-World Examples" with before/after
5. Run `make test-scripts` to verify

### Changing Annotation Format

1. Update `pkg/scriptloader/loader.go` constants
2. Update tests in `pkg/scriptloader/loader_test.go`
3. Update docs/reference/annotations.md
4. Update all example manifests
5. Update README.md
6. Run `make test` to verify

### Adding New Webhook Configuration

1. Update `pkg/webhook/handler.go`
2. Add tests in `pkg/webhook/handler_test.go`
3. Create manifest in `examples/manifests/`
4. Update README.md
5. Update docs/getting-started/installation.md
6. Run `make test` to verify

## Troubleshooting

### Tests Failing

```bash
# Run specific package tests
go test -v ./pkg/luarunner

# Run with race detection
go test -race ./pkg/...

# Run single test
go test -v -run TestRunScript_Success ./pkg/luarunner
```

### Coverage Dropped

```bash
# Generate coverage report
make coverage

# View in browser
open coverage.html
```

### Integration Tests Failing

```bash
# Check Kind cluster
kind get clusters

# Check kubectl access
kubectl cluster-info

# Delete and recreate
make kind-delete
make kind-create
```

## Best Practices

### Testing

- Test happy path AND error cases
- Use table-driven tests for multiple scenarios
- Mock external dependencies (K8s API, etc.)
- Keep tests fast (<1s per test)
- Use descriptive test names: `TestFunctionName_Scenario`

### Error Handling

- Return errors, don't panic
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Log errors before returning
- Use appropriate log levels (ERROR, WARNING, INFO, DEBUG)

### Documentation

- **Show, don't just tell**: Include before/after examples
- **Be specific**: "Injects Fluent Bit sidecar" not "Adds container"
- **Test examples**: All code examples must work
- **Keep updated**: Docs out of sync are worse than no docs

## Resources

- glua library: https://github.com/thomas-maurice/glua
- gopher-lua: https://github.com/yuin/gopher-lua
- Kubernetes admission webhooks: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/

## Questions?

When in doubt:
1. Check existing code patterns
2. Read the tests (they show expected behavior)
3. Run `make test` frequently
4. Keep changes focused and atomic
