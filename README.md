# glua-webhook

**Kubernetes admission webhook that mutates/validates resources using Lua scripts stored in ConfigMaps.**

Change policies by editing ConfigMaps - no recompilation, no redeployment needed.

## Quick Start (Kubernetes User)

### 1. Deploy the Webhook

```bash
# Create namespace
kubectl create namespace glua-webhook

# Apply manifests (includes RBAC, deployment, service, TLS setup)
kubectl apply -f https://github.com/thomas-maurice/glua-webhook/releases/latest/download/install.yaml

# Enable for your namespace
kubectl label namespace default glua.maurice.fr/enabled=true
```

### 2. Create a Policy Script

```bash
kubectl create configmap add-labels \
  --namespace default \
  --from-literal=script.lua='
object.metadata = object.metadata or {}
object.metadata.labels = object.metadata.labels or {}
object.metadata.labels["processed-by"] = "glua-webhook"
object.metadata.labels["processed-at"] = os.date("%Y-%m-%dT%H:%M:%SZ")
'
```

### 3. Use It

```bash
# Create a Pod referencing the script
kubectl run nginx --image=nginx \
  --annotations='glua.maurice.fr/scripts=default/add-labels'

# Check the result
kubectl get pod nginx -o jsonpath='{.metadata.labels}' | jq
```

Output:
```json
{
  "processed-at": "2025-10-28T22:00:00Z",
  "processed-by": "glua-webhook",
  "run": "nginx"
}
```

### 4. Update Policy (No Redeployment!)

```bash
# Edit the ConfigMap to change the policy
kubectl edit configmap add-labels

# Delete and recreate the pod - new policy applies immediately
kubectl delete pod nginx
kubectl run nginx --image=nginx \
  --annotations='glua.maurice.fr/scripts=default/add-labels'
```

**That's it!** Your pods are now automatically processed by Lua scripts.

---

## What Problem Does This Solve?

Traditional admission controllers are compiled binaries - changing policies requires:
1. Modify code
2. Recompile
3. Build Docker image
4. Push to registry
5. Update deployment
6. Wait for rollout

**glua-webhook eliminates all of this:**
- Policies live in ConfigMaps
- Edit ConfigMap, policy changes instantly
- No compilation, no Docker, no deployment
- Lua is easy to write and test

---

## Real-World Examples

### Example 1: Inject Logging Sidecar

**Create the script:**
```bash
kubectl create configmap inject-logging --from-literal=script.lua='
if object.kind ~= "Pod" then return end

object.spec = object.spec or {}
object.spec.containers = object.spec.containers or {}

-- Check if already exists
for i = 1, #object.spec.containers do
  if object.spec.containers[i].name == "fluent-bit" then return end
end

-- Add sidecar
table.insert(object.spec.containers, {
  name = "fluent-bit",
  image = "fluent/fluent-bit:latest",
  volumeMounts = {{name = "varlog", mountPath = "/var/log", readOnly = true}}
})

-- Add volume
object.spec.volumes = object.spec.volumes or {}
table.insert(object.spec.volumes, {
  name = "varlog",
  hostPath = {path = "/var/log"}
})
'
```

**Use it:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
  annotations:
    glua.maurice.fr/scripts: "default/inject-logging"
spec:
  containers:
  - name: app
    image: myapp:latest
```

**Result:** Every Pod automatically gets Fluent Bit sidecar for log collection.

---

### Example 2: Enforce Cost-Center Labels

**Create validation script:**
```bash
kubectl create configmap validate-cost-center --from-literal=script.lua='
if object.kind ~= "Pod" then return end

if not object.metadata or not object.metadata.labels or
   not object.metadata.labels["cost-center"] then
  error("All Pods must have a cost-center label")
end

local cc = object.metadata.labels["cost-center"]
if not string.match(cc, "^CC%-%d+$") then
  error("cost-center must match format CC-NNNN")
end
'

# Enable validation webhook
kubectl label namespace default glua.maurice.fr/validation-enabled=true
```

**Test it:**
```bash
# This fails
$ kubectl run test --image=nginx --annotations='glua.maurice.fr/scripts=default/validate-cost-center'
Error: All Pods must have a cost-center label

# This succeeds
$ kubectl run test --image=nginx \
    --labels='cost-center=CC-1234' \
    --annotations='glua.maurice.fr/scripts=default/validate-cost-center'
pod/test created
```

---

### Example 3: Set Default Resource Requests

**Create script:**
```bash
kubectl create configmap set-defaults --from-literal=script.lua='
if object.kind ~= "Pod" then return end

for i = 1, #object.spec.containers do
  local c = object.spec.containers[i]
  c.resources = c.resources or {}
  c.resources.requests = c.resources.requests or {}

  if not c.resources.requests.cpu then
    c.resources.requests.cpu = "100m"
  end
  if not c.resources.requests.memory then
    c.resources.requests.memory = "128Mi"
  end
end
'
```

**Apply to namespace (all Pods):**
```bash
# Label namespace to apply to all Pods
kubectl label namespace default \
  glua.maurice.fr/enabled=true \
  glua.maurice.fr/default-scripts=default/set-defaults
```

**Result:** All Pods in namespace get sensible defaults if not specified.

---

## Key Features

### ConfigMap-Based Scripts
Scripts live in ConfigMaps with key `script.lua`. Edit ConfigMap to update policy - no redeployment.

### Sequential Execution
Chain multiple scripts:
```yaml
annotations:
  glua.maurice.fr/scripts: "security/psp,monitoring/inject-metrics,common/labels"
```
Executes alphabetically: `common/labels` → `monitoring/inject-metrics` → `security/psp`

### Mutation & Validation
- **MutatingAdmissionWebhook**: Transform resources (add sidecars, set defaults)
- **ValidatingAdmissionWebhook**: Reject resources (enforce policies)

Enable per-namespace:
```bash
kubectl label namespace prod \
  glua.maurice.fr/enabled=true \
  glua.maurice.fr/validation-enabled=true
```

### Full Lua Power
Scripts access [glua](https://github.com/thomas-maurice/glua) modules:
- `json` - Parse/stringify JSON
- `yaml` - Parse/stringify YAML
- `http` - Make HTTP requests
- `time` - Time functions
- `hash` - SHA256, MD5
- `log` - Structured logging
- `template` - Go templates

**Example:**
```lua
local http = require("http")
local json = require("json")

-- Fetch approved image tag from registry API
local resp, err = http.get("https://registry.internal/api/approved-tags")
if not err then
  local data = json.parse(resp.body)
  object.spec.containers[1].image = "myapp:" .. data.tag
end
```

### Test Scripts Locally
Test before deploying to K8s:
```bash
# Download CLI
wget https://github.com/thomas-maurice/glua-webhook/releases/latest/download/glua-webhook-linux-amd64

# Test on existing Pod
kubectl get pod nginx -o json | ./glua-webhook exec --script myscript.lua

# Test on file
./glua-webhook exec --script myscript.lua --input pod.json --output result.json

# Chain scripts (simulates webhook)
kubectl get pod nginx -o json | \
  ./glua-webhook exec --script add-labels.lua | \
  ./glua-webhook exec --script inject-sidecar.lua
```

---

## Installation

### Quick Install (Recommended)

```bash
kubectl apply -f https://github.com/thomas-maurice/glua-webhook/releases/latest/download/install.yaml
kubectl label namespace default glua.maurice.fr/enabled=true
```

### Manual Install

```bash
git clone https://github.com/thomas-maurice/glua-webhook
cd glua-webhook

# Deploy to existing cluster
kubectl apply -f examples/manifests/00-namespace.yaml
kubectl apply -f examples/manifests/01-configmaps.yaml
kubectl apply -f examples/manifests/02-deployment.yaml
kubectl apply -f examples/manifests/03-service.yaml
kubectl apply -f examples/manifests/04-rbac.yaml
kubectl apply -f examples/manifests/05-webhooks.yaml

# Or use local Kind cluster
make kind-create
make kind-deploy
kubectl label namespace default glua.maurice.fr/enabled=true
```

### TLS Certificate Setup

Kubernetes requires TLS for admission webhooks. Two options:

**Option 1: cert-manager (Recommended)**
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
kubectl apply -f examples/manifests/cert-manager-issuer.yaml
```

**Option 2: Manual certificates**
```bash
./scripts/generate-certs.sh
kubectl create secret tls glua-webhook-certs \
  --cert=certs/tls.crt \
  --key=certs/tls.key \
  -n glua-webhook
```

---

## Writing Lua Scripts

### How Lua Scripts Work with Kubernetes Resources

When a Kubernetes resource is admitted, glua-webhook:
1. Converts the resource to JSON
2. Parses JSON into a Lua table
3. Sets global variable `object` to this table
4. Executes your script
5. Converts modified `object` back to JSON
6. Generates JSONPatch with changes

**The `object` variable is a Lua table matching the Kubernetes resource structure exactly.**

### Basic Structure

Scripts receive global variable `object` (the Kubernetes resource):

```lua
-- Read fields
local name = object.metadata.name
local kind = object.kind
local namespace = object.metadata.namespace

-- Modify fields (mutation)
object.metadata.labels = object.metadata.labels or {}
object.metadata.labels["env"] = "production"

-- Reject resource (validation)
if not object.metadata.labels["owner"] then
  error("Resources must have an owner label")
end

-- No return needed - modifications to 'object' are automatic
```

### Understanding the `object` Table Structure

The `object` table mirrors the Kubernetes YAML structure:

```yaml
# Kubernetes Pod YAML
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: default
  labels:
    app: nginx
spec:
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80
```

Becomes this Lua table:

```lua
object = {
  apiVersion = "v1",
  kind = "Pod",
  metadata = {
    name = "nginx",
    namespace = "default",
    labels = {
      app = "nginx"
    }
  },
  spec = {
    containers = {
      {
        name = "nginx",
        image = "nginx:latest",
        ports = {
          {containerPort = 80}
        }
      }
    }
  }
}
```

### Safe Nil Checking (CRITICAL)

**Always check for nil before accessing nested fields:**

```lua
-- WRONG - crashes if metadata is nil
object.metadata.labels["key"] = "value"

-- CORRECT - safe initialization
object.metadata = object.metadata or {}
object.metadata.labels = object.metadata.labels or {}
object.metadata.labels["key"] = "value"
```

**Why this matters:** Resources may not have all fields populated. Accessing `object.metadata.labels` fails if `metadata` is nil.

### Manipulating Kubernetes Resources

#### 1. Adding/Modifying Labels

```lua
-- Initialize labels table
object.metadata = object.metadata or {}
object.metadata.labels = object.metadata.labels or {}

-- Add single label
object.metadata.labels["env"] = "production"
object.metadata.labels["team"] = "platform"

-- Copy label from one key to another
if object.metadata.labels["app"] then
  object.metadata.labels["app.kubernetes.io/name"] = object.metadata.labels["app"]
end

-- Add timestamp label
object.metadata.labels["last-modified"] = os.date("%Y-%m-%dT%H:%M:%SZ")
```

#### 2. Adding/Modifying Annotations

```lua
-- Initialize annotations
object.metadata.annotations = object.metadata.annotations or {}

-- Add annotation
object.metadata.annotations["description"] = "Managed by glua-webhook"

-- Store JSON in annotation
local json = require("json")
local metadata = {processed = true, timestamp = os.time()}
local encoded, err = json.stringify(metadata)
if not err then
  object.metadata.annotations["processing-info"] = encoded
end
```

#### 3. Modifying Pod Containers

**Adding a container:**
```lua
if object.kind ~= "Pod" then return end

object.spec = object.spec or {}
object.spec.containers = object.spec.containers or {}

-- Add sidecar container
table.insert(object.spec.containers, {
  name = "sidecar",
  image = "my-sidecar:latest",
  env = {
    {name = "LOG_LEVEL", value = "info"},
    {name = "APP_NAME", value = object.metadata.name}
  },
  ports = {
    {containerPort = 9090, name = "metrics"}
  },
  volumeMounts = {
    {name = "config", mountPath = "/etc/config", readOnly = true}
  }
})
```

**Modifying existing containers:**
```lua
-- Iterate through all containers
for i = 1, #object.spec.containers do
  local container = object.spec.containers[i]

  -- Add environment variable to all containers
  container.env = container.env or {}
  table.insert(container.env, {
    name = "CLUSTER_NAME",
    value = "production-us-west-2"
  })

  -- Set resource limits if not specified
  container.resources = container.resources or {}
  container.resources.limits = container.resources.limits or {}
  if not container.resources.limits.memory then
    container.resources.limits.memory = "512Mi"
  end

  -- Add volume mount to all containers
  container.volumeMounts = container.volumeMounts or {}
  table.insert(container.volumeMounts, {
    name = "shared-data",
    mountPath = "/data"
  })
end
```

**Finding and modifying specific container:**
```lua
for i = 1, #object.spec.containers do
  if object.spec.containers[i].name == "app" then
    -- Update image tag
    object.spec.containers[i].image = "myapp:v2.0.0"

    -- Add command
    object.spec.containers[i].command = {"/bin/sh", "-c", "exec /app/start.sh"}

    -- Add args
    object.spec.containers[i].args = {"--port=8080", "--verbose"}
  end
end
```

#### 4. Adding Volumes

```lua
if object.kind ~= "Pod" then return end

object.spec.volumes = object.spec.volumes or {}

-- Add ConfigMap volume
table.insert(object.spec.volumes, {
  name = "config",
  configMap = {
    name = "app-config",
    items = {
      {key = "app.conf", path = "app.conf"}
    }
  }
})

-- Add Secret volume
table.insert(object.spec.volumes, {
  name = "secrets",
  secret = {
    secretName = "app-secrets",
    defaultMode = 0400
  }
})

-- Add EmptyDir volume
table.insert(object.spec.volumes, {
  name = "cache",
  emptyDir = {
    sizeLimit = "1Gi"
  }
})

-- Add HostPath volume
table.insert(object.spec.volumes, {
  name = "host-data",
  hostPath = {
    path = "/var/data",
    type = "DirectoryOrCreate"
  }
})
```

#### 5. Setting Resource Requests/Limits

```lua
for i = 1, #object.spec.containers do
  local c = object.spec.containers[i]

  -- Initialize resources structure
  c.resources = c.resources or {}
  c.resources.requests = c.resources.requests or {}
  c.resources.limits = c.resources.limits or {}

  -- Set requests
  if not c.resources.requests.cpu then
    c.resources.requests.cpu = "100m"
  end
  if not c.resources.requests.memory then
    c.resources.requests.memory = "128Mi"
  end

  -- Set limits
  if not c.resources.limits.cpu then
    c.resources.limits.cpu = "1000m"
  end
  if not c.resources.limits.memory then
    c.resources.limits.memory = "512Mi"
  end
end
```

#### 6. Adding Init Containers

```lua
object.spec.initContainers = object.spec.initContainers or {}

table.insert(object.spec.initContainers, {
  name = "init-db",
  image = "busybox:latest",
  command = {"sh", "-c", "until nc -z db 5432; do echo waiting for db; sleep 2; done"}
})

table.insert(object.spec.initContainers, {
  name = "copy-configs",
  image = "busybox:latest",
  command = {"cp", "/config-source/app.conf", "/config/app.conf"},
  volumeMounts = {
    {name = "config-source", mountPath = "/config-source", readOnly = true},
    {name = "config", mountPath = "/config"}
  }
})
```

#### 7. Modifying Service Specs

```lua
if object.kind ~= "Service" then return end

object.spec = object.spec or {}

-- Change service type
object.spec.type = "LoadBalancer"

-- Add ports
object.spec.ports = object.spec.ports or {}
table.insert(object.spec.ports, {
  name = "https",
  port = 443,
  targetPort = 8443,
  protocol = "TCP"
})

-- Set selector
object.spec.selector = object.spec.selector or {}
object.spec.selector["app"] = "myapp"

-- Add annotations for cloud load balancer
object.metadata.annotations = object.metadata.annotations or {}
object.metadata.annotations["service.beta.kubernetes.io/aws-load-balancer-type"] = "nlb"
```

#### 8. Working with Arrays (Containers, Env Vars, etc.)

```lua
-- Check if array is empty
if #object.spec.containers == 0 then
  error("Pod must have at least one container")
end

-- Add to array
table.insert(object.spec.containers, {name = "new-container", image = "nginx"})

-- Remove from array by index
table.remove(object.spec.containers, 2)

-- Find and remove by name
for i = #object.spec.containers, 1, -1 do
  if object.spec.containers[i].name == "old-container" then
    table.remove(object.spec.containers, i)
  end
end

-- Check if value exists in array
local has_port_80 = false
for i = 1, #object.spec.containers[1].ports do
  if object.spec.containers[1].ports[i].containerPort == 80 then
    has_port_80 = true
    break
  end
end
```

### Filter by Resource Type

```lua
-- Only process Pods
if object.kind ~= "Pod" then return end

-- Process multiple types
if object.kind ~= "Pod" and object.kind ~= "Deployment" then
  return
end

-- Only process specific namespace
if object.metadata.namespace ~= "production" then return end

-- Skip system namespaces
local system_namespaces = {["kube-system"] = true, ["kube-public"] = true}
if system_namespaces[object.metadata.namespace] then
  return
end

-- Only process resources with specific label
if not object.metadata.labels or not object.metadata.labels["process-me"] then
  return
end
```

### Using Modules

```lua
local json = require("json")
local log = require("log")
local hash = require("hash")
local time = require("time")

-- Logging (appears in webhook logs)
log.info("Processing " .. object.metadata.name)
log.warn("Missing optional field")
log.error("Validation failed")

-- JSON operations
local data = {foo = "bar", nested = {key = "value"}}
local str, err = json.stringify(data)
if not err then
  object.metadata.annotations["data"] = str
end

-- Parse JSON from annotation
if object.metadata.annotations["config"] then
  local config, err = json.parse(object.metadata.annotations["config"])
  if not err and config.enabled then
    -- Use parsed config
    object.metadata.labels["feature-enabled"] = "true"
  end
end

-- Hashing
local pod_hash = hash.sha256(object.metadata.name .. object.metadata.namespace)
object.metadata.labels["pod-hash"] = pod_hash:sub(1, 8)

-- Time operations
local now = time.now()
object.metadata.annotations["processed-at"] = tostring(now)
```

### Complete Working Example

Here's a complete script that adds monitoring sidecar with full configuration:

```lua
local log = require("log")

-- Only process Pods in production namespace
if object.kind ~= "Pod" then return end
if object.metadata.namespace ~= "production" then return end

-- Skip if already has monitoring sidecar
for i = 1, #object.spec.containers do
  if object.spec.containers[i].name == "prometheus-exporter" then
    log.info("Monitoring sidecar already present")
    return
  end
end

log.info("Adding monitoring sidecar to " .. object.metadata.name)

-- Add prometheus exporter sidecar
table.insert(object.spec.containers, {
  name = "prometheus-exporter",
  image = "prom/node-exporter:latest",
  ports = {{containerPort = 9100, name = "metrics", protocol = "TCP"}},
  resources = {
    requests = {cpu = "50m", memory = "64Mi"},
    limits = {cpu = "100m", memory = "128Mi"}
  },
  volumeMounts = {
    {name = "proc", mountPath = "/host/proc", readOnly = true},
    {name = "sys", mountPath = "/host/sys", readOnly = true}
  }
})

-- Add required volumes
object.spec.volumes = object.spec.volumes or {}
table.insert(object.spec.volumes, {
  name = "proc",
  hostPath = {path = "/proc"}
})
table.insert(object.spec.volumes, {
  name = "sys",
  hostPath = {path = "/sys"}
})

-- Add Prometheus annotations
object.metadata.annotations = object.metadata.annotations or {}
object.metadata.annotations["prometheus.io/scrape"] = "true"
object.metadata.annotations["prometheus.io/port"] = "9100"
object.metadata.annotations["prometheus.io/path"] = "/metrics"

-- Add monitoring label
object.metadata.labels = object.metadata.labels or {}
object.metadata.labels["monitoring"] = "enabled"

log.info("Successfully added monitoring sidecar")
```

### Testing Your Scripts Locally

Before deploying to Kubernetes, test scripts locally:

```bash
# Test on existing resource
kubectl get pod nginx -o json | ./glua-webhook exec --script myscript.lua

# Test with verbose logging
kubectl get pod nginx -o json | ./glua-webhook exec --script myscript.lua --verbose

# Test on file
./glua-webhook exec --script myscript.lua --input pod.json --output result.json

# View the diff
diff <(cat pod.json | jq -S .) <(cat result.json | jq -S .)

# Chain multiple scripts
kubectl get pod nginx -o json | \
  ./glua-webhook exec --script add-labels.lua | \
  ./glua-webhook exec --script inject-sidecar.lua | \
  jq .
```

### More Examples

See [examples/scripts/README.md](examples/scripts/README.md) for:
- `add-label.lua` - Add timestamps and processed flags
- `inject-sidecar.lua` - Inject logging sidecar
- `validate-labels.lua` - Enforce required labels
- `add-annotations.lua` - Add JSON metadata
- `propagate-deployment-labels.lua` - Transform label values

Full scripting guide: [docs/guides/writing-scripts.md](docs/guides/writing-scripts.md)

---

## Configuration Reference

### Annotations (on resources)

| Annotation | Value | Description |
|------------|-------|-------------|
| `glua.maurice.fr/scripts` | `"ns/cm1,ns/cm2"` | Comma-separated ConfigMap references |

**Format:** `namespace/configmap-name`

**Example:**
```yaml
metadata:
  annotations:
    glua.maurice.fr/scripts: "default/script1,kube-system/script2"
```

### Namespace Labels

| Label | Value | Description |
|-------|-------|-------------|
| `glua.maurice.fr/enabled` | `"true"` | Enable mutation webhook |
| `glua.maurice.fr/validation-enabled` | `"true"` | Enable validation webhook |

**Example:**
```bash
kubectl label namespace prod \
  glua.maurice.fr/enabled=true \
  glua.maurice.fr/validation-enabled=true
```

### Webhook Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8443` | HTTPS server port |
| `--cert` | `/etc/webhook/certs/tls.crt` | TLS certificate |
| `--key` | `/etc/webhook/certs/tls.key` | TLS private key |
| `--kubeconfig` | `""` | Kubeconfig path (empty = in-cluster) |

---

## Troubleshooting

### Webhook Not Working

```bash
# Check webhook is running
kubectl get pods -n glua-webhook

# View logs
kubectl logs -n glua-webhook deployment/glua-webhook -f

# Verify namespace is labeled
kubectl get namespace default -o jsonpath='{.metadata.labels}'

# Test webhook connectivity
kubectl get endpoints -n glua-webhook
```

### Script Errors

```bash
# View webhook logs for errors
kubectl logs -n glua-webhook deployment/glua-webhook | grep ERROR

# Test script locally first
kubectl get pod mypod -o json | ./glua-webhook exec --script myscript.lua

# Verify ConfigMap exists
kubectl get cm myscript -n default
```

### TLS Issues

```bash
# Check certificate secret
kubectl get secret glua-webhook-certs -n glua-webhook

# Verify certificate
kubectl get secret glua-webhook-certs -n glua-webhook \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

---

## Development Setup

### For Developers

If you want to develop glua-webhook or test changes:

#### Using Nix (Recommended)

We provide a Nix flake with all tools:

```bash
# Clone repo
git clone https://github.com/thomas-maurice/glua-webhook
cd glua-webhook

# Enable Nix flake (provides go, kubectl, kind, make, golangci-lint)
direnv allow

# Or manually
nix develop
```

The flake provides:
- Go (latest)
- kubectl
- kind
- curl, wget
- make
- golangci-lint

#### Manual Setup

Install dependencies:
- Go 1.25+
- kubectl
- kind
- make
- golangci-lint (optional)

### Build and Test

```bash
# Build binary
make build

# Run all tests (unit + integration)
make

# Run only unit tests
make test-unit

# Test Lua scripts
make test-scripts

# Format code
make fmt

# Lint
make lint
```

### Local Testing with Kind

```bash
# Create Kind cluster
make kind-create

# Build and deploy to Kind
make kind-deploy

# Watch logs
kubectl logs -n glua-webhook deployment/glua-webhook -f

# Test changes
kubectl apply -f examples/manifests/07-example-pod.yaml

# Clean up
make kind-delete
```

### Project Structure

```
.
├── cmd/glua-webhook/      # CLI (exec, webhook commands)
│   ├── main.go            # Entrypoint
│   ├── root.go            # Root command
│   ├── exec.go            # Test scripts locally
│   └── webhook.go         # Run webhook server
├── pkg/
│   ├── luarunner/         # Lua execution engine
│   ├── scriptloader/      # ConfigMap loader
│   └── webhook/           # HTTP handlers
├── examples/
│   ├── manifests/         # Kubernetes YAMLs
│   └── scripts/           # Example Lua scripts
├── docs/                  # Documentation
├── test/                  # Integration tests
├── Makefile               # Build targets
├── flake.nix              # Nix dev environment
└── .envrc                 # direnv config
```

### Adding Features

1. Write code
2. Add tests (>70% coverage required)
3. Run `make` (tests + build)
4. Run `make fmt` and `make lint`
5. Update docs
6. Submit PR

See [CLAUDE.md](CLAUDE.md) for full workflow.

---

## Documentation

- **[Installation Guide](docs/getting-started/installation.md)** - Detailed install steps
- **[Writing Scripts](docs/guides/writing-scripts.md)** - Complete Lua guide
- **[Example Scripts](examples/scripts/README.md)** - Tested examples with explanations
- **[Type Stubs](docs/guides/type-stubs.md)** - IDE autocompletion for K8s types
- **[Label Propagation Guide](docs/guides/deployment-label-propagation.md)** - Step-by-step tutorial
- **[Annotations Reference](docs/reference/annotations.md)** - Complete API docs

---

## FAQ

**Q: Why Lua?**
A: Lightweight, fast, designed for embedding. gopher-lua is pure Go with no cgo dependencies.

**Q: How do I debug scripts?**
A: Test locally with `glua-webhook exec` command, use `log.info()` for logging, or `kubectl apply --dry-run=server`.

**Q: Can scripts access Kubernetes API?**
A: Not directly. Scripts can make HTTP requests to the API server if needed.

**Q: What if a script fails?**
A: By default the admission is allowed and error logged. Set `failurePolicy: Fail` for critical validations.

**Q: How do I update a script?**
A: Just edit the ConfigMap - changes apply immediately (K8s caches for ~1 minute).

**Q: Is this production-ready?**
A: Test coverage >70%, integration tests pass. Review scripts carefully before production.

---

## License

[Specify your license]

## Support

- **Issues:** [GitHub Issues](https://github.com/thomas-maurice/glua-webhook/issues)
- **Docs:** [Documentation](docs/)

## Acknowledgments

- [glua](https://github.com/thomas-maurice/glua) - Kubernetes-aware Lua library
- [gopher-lua](https://github.com/yuin/gopher-lua) - Pure Go Lua VM
