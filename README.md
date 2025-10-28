# glua-webhook

**A Kubernetes admission webhook that transforms resources using Lua scripts stored in ConfigMaps.**

glua-webhook intercepts Kubernetes resource creation/updates and runs your custom Lua scripts against them - providing a programmable, dynamic mutation/validation layer that doesn't require webhook redeployment when policies change.

## What Problem Does This Solve?

Traditional Kubernetes admission controllers are compiled binaries. Changing policies means recompiling, rebuilding images, and redeploying. glua-webhook solves this by:

- **Storing policies as ConfigMaps** - Update a ConfigMap, policy changes immediately
- **Writing logic in Lua** - No compilation, no container rebuilds
- **Chaining transformations** - Multiple scripts run sequentially on the same resource
- **Dynamic validation** - Reject resources that don't meet criteria without redeploy

**Use glua-webhook when you need:**
- Sidecar injection (logging, monitoring, security agents)
- Policy enforcement (required labels, naming conventions, resource limits)
- Default value injection (resource requests, annotations, labels)
- Dynamic configuration (fetch data from APIs, apply conditional logic)
- Compliance labeling (audit trails, regulatory tags)

## How It Works

```
┌──────────────┐
│ kubectl apply│
└──────┬───────┘
       │ Pod YAML
       ▼
┌─────────────────────────────┐
│   Kubernetes API Server     │
│  (Admission Controller)     │
└──────┬──────────────────────┘
       │ AdmissionReview{Request}
       ▼
┌─────────────────────────────┐
│      glua-webhook           │
│                             │
│  1. Extract annotations     │
│  2. Fetch ConfigMaps        │
│  3. Execute Lua scripts     │
│  4. Generate JSONPatch      │
└──────┬──────────────────────┘
       │ AdmissionReview{Response}
       ▼
┌─────────────────────────────┐
│    Modified Pod created     │
└─────────────────────────────┘
```

**Workflow:**

1. User creates/updates a Kubernetes resource with annotation `glua.maurice.fr/scripts: "namespace/configmap-name"`
2. Kubernetes API server calls glua-webhook (mutating or validating)
3. Webhook parses annotation and fetches ConfigMap(s) containing Lua scripts
4. Scripts execute in alphabetical order (each in isolated VM)
5. Modified resource is created, or request is rejected if validation fails

## Real Example

**What you submit:**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  annotations:
    glua.maurice.fr/scripts: "default/inject-logging"
spec:
  containers:
  - name: nginx
    image: nginx:latest
```

**ConfigMap with Lua script:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: inject-logging
  namespace: default
data:
  script.lua: |
    -- Only process Pods
    if object.kind ~= "Pod" then return end

    -- Add Fluent Bit sidecar for log collection
    table.insert(object.spec.containers, {
      name = "fluent-bit",
      image = "fluent/fluent-bit:latest",
      volumeMounts = {{
        name = "varlog",
        mountPath = "/var/log",
        readOnly = true
      }}
    })

    -- Add host path volume
    if object.spec.volumes == nil then
      object.spec.volumes = {}
    end
    table.insert(object.spec.volumes, {
      name = "varlog",
      hostPath = { path = "/var/log" }
    })

    -- Add label for tracking
    if object.metadata.labels == nil then
      object.metadata.labels = {}
    end
    object.metadata.labels["logging-injected"] = "true"
```

**What gets created:**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    logging-injected: "true"       # ← ADDED BY SCRIPT
  annotations:
    glua.maurice.fr/scripts: "default/inject-logging"
spec:
  containers:
  - name: nginx
    image: nginx:latest
  - name: fluent-bit               # ← ADDED BY SCRIPT
    image: fluent/fluent-bit:latest
    volumeMounts:
    - name: varlog
      mountPath: /var/log
      readOnly: true
  volumes:                         # ← ADDED BY SCRIPT
  - name: varlog
    hostPath:
      path: /var/log
```

**Result:** Every Pod automatically gets log collection without modifying deployments or charts.

**To change the policy:** Just edit the ConfigMap - no webhook redeployment needed.

## Features

### Dual Webhook Support

- **MutatingAdmissionWebhook**: Transform resources (add sidecars, labels, defaults)
- **ValidatingAdmissionWebhook**: Reject resources that don't meet criteria

Controlled via namespace labels:
- `glua.maurice.fr/enabled=true` - Enable mutation
- `glua.maurice.fr/validation-enabled=true` - Enable validation

### ConfigMap-Based Scripts

Scripts stored in ConfigMaps with `script.lua` key. Update the ConfigMap, policy changes immediately.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-policy
  namespace: default
data:
  script.lua: |
    -- Lua code here
    object.metadata.labels["processed"] = "true"
```

### Sequential Execution

Chain multiple scripts by listing them in annotation:

```yaml
metadata:
  annotations:
    glua.maurice.fr/scripts: "security/psp,monitoring/inject-metrics,common/labels"
```

Scripts execute in **alphabetical order** by `namespace/configmap-name`:
1. `common/labels`
2. `monitoring/inject-metrics`
3. `security/psp`

Output of one script becomes input to the next.

### Isolated Lua VMs

Each script runs in its own gopher-lua VM instance - no state pollution, no global variable conflicts.

### Full glua Module Access

Scripts have access to all [glua](https://github.com/thomas-maurice/glua) modules:

```lua
local json = require("json")     -- JSON parse/stringify
local yaml = require("yaml")     -- YAML parse/stringify
local http = require("http")     -- HTTP client
local time = require("time")     -- Time functions
local hash = require("hash")     -- SHA256, MD5
local base64 = require("base64") -- Encoding
local template = require("template") -- Go templates
local log = require("log")       -- Structured logging
local spew = require("spew")     -- Debug printing
local fs = require("fs")         -- File system (read-only)
```

### TypeRegistry Integration

glua-webhook uses [glua TypeRegistry](https://github.com/thomas-maurice/glua#type-registry) to provide Lua Language Server annotations. This enables:

- **IDE autocompletion** for Kubernetes types
- **Type checking** in your editor
- **Generated stubs** for all K8s API objects

See [docs/guides/type-stubs.md](docs/guides/type-stubs.md) for IDE setup.

### Comprehensive Logging

Every step is logged with context for debugging:

```
INFO  Received admission request uid=abc123 kind=Pod name=nginx namespace=default
INFO  Extracted annotation glua.maurice.fr/scripts: "default/inject-logging"
INFO  Loading script from ConfigMap default/inject-logging
INFO  Executing script default/inject-logging (1/1)
INFO  Script default/inject-logging completed successfully in 3.2ms
INFO  Generated JSONPatch with 5 operations
INFO  Admission allowed with modifications
```

### High Test Coverage

- Unit tests: >70% coverage (luarunner: 90.5%, scriptloader: 98%, webhook: 82.2%)
- Integration tests: Kind cluster-based end-to-end tests
- Script tests: Framework for testing Lua scripts in isolation

## Quick Start (5 minutes)

### Prerequisites

- Kubernetes 1.20+
- kubectl
- Docker
- Kind (for local testing)

**Using Nix?** We provide a flake:

```bash
direnv allow  # Loads go, kubectl, kind, curl, wget, make, golangci-lint
```

### Installation

```bash
# Clone
git clone https://github.com/thomas-maurice/glua-webhook
cd glua-webhook

# Create Kind cluster
make kind-create

# Build and deploy webhook
make kind-deploy

# Enable webhook for default namespace
kubectl label namespace default glua.maurice.fr/enabled=true
```

### Create Your First Script

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: add-processed-label
  namespace: default
data:
  script.lua: |
    -- Ensure metadata.labels exists
    if object.metadata == nil then
      object.metadata = {}
    end
    if object.metadata.labels == nil then
      object.metadata.labels = {}
    end

    -- Add labels
    object.metadata.labels["processed-by"] = "glua-webhook"
    object.metadata.labels["processed-at"] = os.date("%Y-%m-%dT%H:%M:%SZ")
EOF
```

### Test It

```bash
# Create a pod with the annotation
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  annotations:
    glua.maurice.fr/scripts: "default/add-processed-label"
spec:
  containers:
  - name: nginx
    image: nginx:latest
EOF

# Check the result
kubectl get pod test-pod -o jsonpath='{.metadata.labels}' | jq
```

Output:

```json
{
  "processed-at": "2025-01-15T14:32:10Z",
  "processed-by": "glua-webhook"
}
```

### Try Example Scripts

The repository includes working examples:

```bash
# Apply all example ConfigMaps
kubectl apply -f examples/manifests/01-configmaps.yaml

# Create a pod with multiple scripts
kubectl apply -f examples/manifests/07-example-pod.yaml

# Inspect the transformed pod
kubectl get pod example-pod -o yaml
```

The pod will have:
- Labels with processing timestamps (add-label.lua)
- Fluent Bit sidecar with volume mounts (inject-sidecar.lua)
- Mutation metadata in annotations (add-annotations.lua)

## Use Cases

### 1. Sidecar Injection

Automatically inject sidecars for logging, monitoring, security, or service mesh:

```lua
-- Inject Datadog APM agent
if object.kind == "Pod" and object.metadata.namespace == "production" then
  -- Check if already injected
  local has_datadog = false
  for i = 1, #object.spec.containers do
    if object.spec.containers[i].name == "datadog-agent" then
      has_datadog = true
      break
    end
  end

  if not has_datadog then
    table.insert(object.spec.containers, {
      name = "datadog-agent",
      image = "datadog/agent:latest",
      env = {
        {name = "DD_API_KEY", valueFrom = {secretKeyRef = {name = "datadog", key = "api-key"}}},
        {name = "DD_APM_ENABLED", value = "true"}
      }
    })
  end
end
```

### 2. Policy Enforcement

Validate resources meet organizational standards:

```lua
-- Enforce cost-center label on all Pods
if object.kind == "Pod" then
  if object.metadata == nil or object.metadata.labels == nil or
     object.metadata.labels["cost-center"] == nil then
    error("All Pods must have a 'cost-center' label for billing")
  end

  -- Validate format (e.g., "CC-1234")
  local cc = object.metadata.labels["cost-center"]
  if not string.match(cc, "^CC%-%d+$") then
    error("cost-center label must match format 'CC-NNNN'")
  end
end
```

### 3. Default Values

Set sensible defaults for resources:

```lua
-- Set default CPU/memory requests if not specified
if object.kind == "Pod" and object.spec and object.spec.containers then
  for i = 1, #object.spec.containers do
    local c = object.spec.containers[i]

    -- Ensure resources structure exists
    if c.resources == nil then c.resources = {} end
    if c.resources.requests == nil then c.resources.requests = {} end
    if c.resources.limits == nil then c.resources.limits = {} end

    -- Set defaults
    if c.resources.requests.cpu == nil then
      c.resources.requests.cpu = "100m"
    end
    if c.resources.requests.memory == nil then
      c.resources.requests.memory = "128Mi"
    end
    if c.resources.limits.memory == nil then
      c.resources.limits.memory = "512Mi"
    end
  end
end
```

### 4. Dynamic Configuration

Fetch data from external APIs:

```lua
local http = require("http")
local json = require("json")
local log = require("log")

-- Get latest approved image tag from internal registry
local response, err = http.get("https://registry.internal/api/v1/images/myapp/latest-approved")
if err then
  log.warn("Failed to fetch approved image tag: " .. err)
  return
end

local data, parse_err = json.parse(response.body)
if parse_err then
  log.warn("Failed to parse registry response: " .. parse_err)
  return
end

-- Update first container image
if data.tag and object.spec and object.spec.containers and #object.spec.containers > 0 then
  object.spec.containers[1].image = "registry.internal/myapp:" .. data.tag
  log.info("Updated image to approved tag: " .. data.tag)
end
```

### 5. Compliance and Audit

Add audit trails and compliance labels:

```lua
local json = require("json")
local hash = require("hash")

-- Create audit metadata
local audit = {
  mutated_at = os.date("%Y-%m-%dT%H:%M:%SZ"),
  mutated_by = "glua-webhook",
  original_hash = hash.sha256(json.stringify(object) or "")
}

-- Add to annotations
if object.metadata.annotations == nil then
  object.metadata.annotations = {}
end
object.metadata.annotations["audit/mutation-log"] = json.stringify(audit)

-- Add compliance labels
if object.metadata.labels == nil then
  object.metadata.labels = {}
end
object.metadata.labels["compliance/pci-dss"] = "validated"
object.metadata.labels["compliance/hipaa"] = "compliant"
object.metadata.labels["security/scanned"] = "true"
```

## Writing Lua Scripts

### Basic Structure

Every script receives a global variable `object` containing the Kubernetes resource as a Lua table:

```lua
-- Read fields
local name = object.metadata.name
local namespace = object.metadata.namespace
local kind = object.kind

-- Modify fields (mutation)
if object.metadata.labels == nil then
  object.metadata.labels = {}
end
object.metadata.labels["env"] = "production"

-- Reject resource (validation)
if object.metadata.labels["owner"] == nil then
  error("Resources must have an 'owner' label")
end

-- No return statement needed - modifications to 'object' are automatic
```

### Nil Checking

**Always check for nil before accessing nested fields:**

```lua
-- WRONG - will error if metadata or labels is nil
object.metadata.labels["key"] = "value"

-- CORRECT - safe approach
if object.metadata == nil then
  object.metadata = {}
end
if object.metadata.labels == nil then
  object.metadata.labels = {}
end
object.metadata.labels["key"] = "value"
```

### Using Modules

```lua
local json = require("json")
local log = require("log")
local hash = require("hash")
local time = require("time")

-- JSON parse/stringify
local data, err = json.parse('{"foo":"bar"}')
if not err then
  local str, err2 = json.stringify(data)
end

-- Logging (appears in webhook logs)
log.info("Processing: " .. object.metadata.name)
log.warn("Warning message")
log.error("Error message")

-- Hashing
local h = hash.sha256("data")

-- Time
local now = time.now()
local formatted = time.format(now, "%Y-%m-%d")
```

### Complete Example

```lua
-- inject-prometheus-exporter.lua
-- Injects Prometheus metrics exporter into Pods

local log = require("log")

-- Only process Pods
if object.kind ~= "Pod" then
  log.info("Skipping non-Pod resource: " .. object.kind)
  return
end

-- Skip if exporter already present
if object.spec and object.spec.containers then
  for i = 1, #object.spec.containers do
    if object.spec.containers[i].name == "prometheus-exporter" then
      log.info("Prometheus exporter already present, skipping")
      return
    end
  end
end

-- Add Prometheus exporter sidecar
table.insert(object.spec.containers, {
  name = "prometheus-exporter",
  image = "prom/node-exporter:latest",
  ports = {{containerPort = 9100, name = "metrics", protocol = "TCP"}}
})

-- Add Prometheus scrape annotations
if object.metadata.annotations == nil then
  object.metadata.annotations = {}
end
object.metadata.annotations["prometheus.io/scrape"] = "true"
object.metadata.annotations["prometheus.io/port"] = "9100"
object.metadata.annotations["prometheus.io/path"] = "/metrics"

log.info("Injected Prometheus exporter into Pod: " .. object.metadata.name)
```

**For comprehensive scripting guide, see [docs/guides/writing-scripts.md](docs/guides/writing-scripts.md).**

## Configuration

### Annotations (on resources)

| Annotation | Description | Example |
|------------|-------------|---------|
| `glua.maurice.fr/scripts` | Comma-separated list of ConfigMap references in `namespace/name` format | `"default/script1,kube-system/script2"` |

**Format:** `namespace/configmap-name`
**Execution order:** Alphabetical by full reference (use numeric prefixes for explicit ordering)

### Namespace Labels

| Label | Description | Values |
|-------|-------------|--------|
| `glua.maurice.fr/enabled` | Enable MutatingAdmissionWebhook for this namespace | `"true"` / `"false"` |
| `glua.maurice.fr/validation-enabled` | Enable ValidatingAdmissionWebhook for this namespace | `"true"` / `"false"` |

**Example:**

```bash
# Enable both webhooks for production namespace
kubectl label namespace production \
  glua.maurice.fr/enabled=true \
  glua.maurice.fr/validation-enabled=true
```

### Webhook Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8443` | HTTPS server port |
| `-cert` | `/etc/webhook/certs/tls.crt` | TLS certificate file |
| `-key` | `/etc/webhook/certs/tls.key` | TLS private key file |
| `-kubeconfig` | `""` | Path to kubeconfig (empty for in-cluster config) |
| `-mutating-path` | `/mutate` | Mutating webhook endpoint path |
| `-validating-path` | `/validate` | Validating webhook endpoint path |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TLS_CERT_FILE` | Override TLS certificate path |
| `TLS_KEY_FILE` | Override TLS key path |

## Development

### Project Structure

```
.
├── cmd/
│   └── webhook/            # Main webhook server
├── pkg/
│   ├── luarunner/          # Lua script execution engine with TypeRegistry
│   ├── scriptloader/       # ConfigMap loader with annotation parsing
│   └── webhook/            # HTTP handlers for mutating/validating webhooks
├── test/
│   ├── integration/        # Kind-based integration tests
│   └── script_test.go      # Lua script unit tests
├── examples/
│   ├── manifests/          # Kubernetes deployment YAMLs
│   └── scripts/            # Example Lua scripts (add-label, inject-sidecar, etc.)
├── docs/                   # Comprehensive documentation
│   ├── getting-started/    # Installation guides
│   ├── guides/             # Scripting guides, type stubs
│   └── reference/          # API reference, annotations
├── Dockerfile              # Multi-stage Docker build
├── Makefile                # Build and test targets
├── flake.nix               # Nix development environment
├── .envrc                  # direnv configuration
└── CLAUDE.md               # Development workflow guidelines
```

### Build and Test

```bash
# Build webhook binary
make build

# Build Docker image
make docker-build

# Run ALL tests (unit + integration) and build
make

# Run only unit tests
make test-unit

# Run integration tests with Kind
make test-integration

# Test example Lua scripts
make test-scripts

# Format code
make fmt

# Lint
make lint
```

### Local Development with Kind

```bash
# Create Kind cluster
make kind-create

# Build and load Docker image into Kind
make kind-load-image

# Deploy webhook to Kind cluster
make kind-deploy

# Watch webhook logs
kubectl logs -n glua-webhook deployment/glua-webhook -f

# Delete Kind cluster
make kind-delete
```

### Using Nix Development Environment

The repository includes a Nix flake providing:

- go (latest)
- kubectl
- kind
- curl, wget
- gnumake
- golangci-lint

**Enable automatically with direnv:**

```bash
direnv allow
```

**Or manually:**

```bash
nix develop
```

### Testing Lua Scripts

Framework for testing scripts in isolation:

```go
// test/script_test.go
func TestAddLabel(t *testing.T) {
    input := `{
        "kind": "Pod",
        "metadata": {"name": "test", "namespace": "default"}
    }`

    script := `
        if object.metadata.labels == nil then
            object.metadata.labels = {}
        end
        object.metadata.labels["test-label"] = "test-value"
    `

    runner := setupScriptRunner(t)
    output, err := runner.RunScriptsSequentially(
        map[string]string{"test": script},
        []byte(input),
    )

    require.NoError(t, err)
    assert.Contains(t, string(output), `"test-label":"test-value"`)
}
```

Run with:

```bash
make test-scripts
```

## Performance

**Benchmarks (single script):**

- Script execution: 1-5ms (depends on complexity)
- VM creation: ~0.1ms
- ConfigMap load: 10-50ms (cached by Kubernetes client)
- **Total latency:** 10-50ms for 1-3 scripts

**Optimization tips:**

- Keep scripts simple (< 100 lines)
- Avoid HTTP requests unless necessary
- Use alphabetical ordering to run cheap scripts first
- Combine related mutations into single scripts
- Use namespace selectors to limit webhook scope
- Monitor webhook logs for slow scripts

## Security Considerations

- **TLS Required:** Kubernetes requires TLS for admission webhooks
- **RBAC:** Webhook needs GET access to ConfigMaps in referenced namespaces
- **Non-root Container:** Runs as UID 1000 (non-root user)
- **Lua Sandboxing:** Scripts run in isolated gopher-lua VMs (no global state sharing)
- **No OS Access:** Lua VMs have no direct OS access (filesystem module is read-only)
- **Network Policies:** Consider restricting webhook egress if scripts make HTTP calls
- **ConfigMap Permissions:** Limit who can create/modify ConfigMaps with scripts
- **Audit Logging:** All script executions are logged with context

**Best practices:**

- Use namespace selectors to limit webhook scope
- Review scripts before deployment (especially if they make HTTP requests)
- Use RBAC to restrict ConfigMap modifications
- Monitor webhook logs for suspicious activity
- Use `failurePolicy: Fail` for critical validation webhooks
- Test scripts thoroughly before production use

## Troubleshooting

### Webhook Not Receiving Requests

```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration glua-mutating-webhook -o yaml
kubectl get validatingwebhookconfiguration glua-validating-webhook -o yaml

# Verify namespace has required label
kubectl get namespace default -o jsonpath='{.metadata.labels}'

# Check webhook pods are running
kubectl get pods -n glua-webhook

# Check API server can reach webhook
kubectl get endpoints -n glua-webhook
```

### Script Execution Errors

```bash
# View webhook logs
kubectl logs -n glua-webhook deployment/glua-webhook -f

# Test script locally
make test-scripts

# Validate ConfigMap exists and has script.lua key
kubectl get cm my-script -n default -o yaml
```

### TLS/Certificate Issues

```bash
# Check certificate secret exists
kubectl get secret glua-webhook-certs -n glua-webhook

# Verify certificate validity
kubectl get secret glua-webhook-certs -n glua-webhook \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout

# Check certificate SANs match service name
kubectl get secret glua-webhook-certs -n glua-webhook \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout | grep DNS
```

### ConfigMap Not Found

```bash
# Check ConfigMap exists
kubectl get cm my-script -n default

# Verify annotation format (must be namespace/name)
# WRONG: glua.maurice.fr/scripts: "my-script"
# CORRECT: glua.maurice.fr/scripts: "default/my-script"

# Check RBAC permissions
kubectl auth can-i get configmaps --as=system:serviceaccount:glua-webhook:glua-webhook -n default
```

### Performance Issues

```bash
# Check webhook logs for timing
kubectl logs -n glua-webhook deployment/glua-webhook | grep "completed in"

# Identify slow scripts
kubectl logs -n glua-webhook deployment/glua-webhook | grep "ms$" | sort -t':' -k5 -n

# Monitor webhook latency
kubectl get --raw /metrics | grep webhook_admission_duration
```

## Documentation

- **[Installation Guide](docs/getting-started/installation.md)** - Detailed installation steps
- **[Writing Lua Scripts](docs/guides/writing-scripts.md)** - Comprehensive scripting guide
- **[Type Stubs & IDE Setup](docs/guides/type-stubs.md)** - Enable autocompletion for K8s types
- **[Annotations Reference](docs/reference/annotations.md)** - Complete annotation documentation
- **[Examples](examples/)** - Real, tested example scripts

## Contributing

Contributions are welcome!

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Write tests: Coverage must be >70%
4. Run tests: `make test` (runs unit + integration tests)
5. Format code: `make fmt`
6. Lint: `make lint`
7. Commit: `git commit -m 'Add amazing feature'`
8. Push: `git push origin feature/amazing-feature`
9. Open a Pull Request

### Development Guidelines

See [CLAUDE.md](CLAUDE.md) for complete development workflow.

**Key requirements:**

- **Always run `make` before committing** - runs all tests + builds
- Test coverage must be >70% for new code
- Update documentation when adding features
- Follow Go standard comment style: `// FunctionName: description`
- Use `make act-test-unit` to test GitHub Actions locally

## FAQ

**Q: Why Lua instead of another language?**
A: Lua is lightweight (gopher-lua is pure Go, no cgo), fast, and designed for embedded scripting. The glua library provides native Kubernetes type conversion and module ecosystem.

**Q: Can scripts access the Kubernetes API directly?**
A: No. Scripts receive the resource being admitted as input. They can make HTTP requests to the API server if needed, but there's no direct client-go access.

**Q: What happens if a script fails?**
A: By default (`failurePolicy: Ignore`), the admission is allowed and the error is logged. For validation webhooks, set `failurePolicy: Fail` to reject the resource.

**Q: Can I use external Lua libraries?**
A: Yes, but only pure Lua libraries (no C bindings). Include them directly in your ConfigMap script.

**Q: How do I debug scripts?**
A: Use `require("log")` for logging to webhook logs, `require("spew")` for debug printing, or `kubectl apply --dry-run=server -o yaml` to see mutations without creating resources.

**Q: Is this production-ready?**
A: Test coverage is >70%, integration tests pass, and the webhook is based on the stable glua library. However, review scripts carefully and test thoroughly before production use.

**Q: How do I update a script?**
A: Just edit the ConfigMap. Changes take effect immediately (Kubernetes client caches ConfigMaps for ~1 minute by default).

**Q: Can scripts modify resources outside the admission request?**
A: No. Scripts can only modify the `object` global (the resource being admitted). They can make HTTP requests to fetch data, but can't directly mutate other resources.

## License

[Specify your license]

## Acknowledgments

- [glua](https://github.com/thomas-maurice/glua) - Kubernetes-aware Lua library with type system
- [gopher-lua](https://github.com/yuin/gopher-lua) - Pure Go Lua 5.1 VM

## Support

- **Issues:** [GitHub Issues](https://github.com/thomas-maurice/glua-webhook/issues)
- **Discussions:** [GitHub Discussions](https://github.com/thomas-maurice/glua-webhook/discussions)
- **Documentation:** [docs/](docs/)
