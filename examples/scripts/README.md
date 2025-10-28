# Example Scripts

This directory contains example Lua scripts demonstrating glua-webhook capabilities. Each script is small, focused, and ready to use.

## Quick Reference

| Script | Purpose | Type | Key Features |
|--------|---------|------|--------------|
| [add-label.lua](#add-labellua) | Add processing labels | Mutation | Basic metadata modification |
| [inject-sidecar.lua](#inject-sidecarlua) | Inject logging sidecar | Mutation | Container manipulation, idempotent |
| [add-annotations.lua](#add-annotationslua) | Add JSON metadata | Mutation | Uses `json` and `time` modules |
| [validate-labels.lua](#validate-labelslua) | Enforce required labels | Validation | Error handling, validation logic |
| [propagate-deployment-labels.lua](#propagate-deployment-labelslua) | Modify label values | Mutation | Pattern matching, logging |

## Testing Scripts Locally

Test any script before deploying to Kubernetes:

```bash
# Test on stdin
kubectl get pod mypod -o json | glua-webhook exec --script add-label.lua

# Test with files
glua-webhook exec --script inject-sidecar.lua --input pod.json --output modified.json

# Chain multiple scripts (simulates webhook behavior)
kubectl get pod mypod -o json | \
  glua-webhook exec --script add-label.lua | \
  glua-webhook exec --script inject-sidecar.lua
```

## Script Details

### add-label.lua

**Purpose**: Adds processing timestamp and processed flag to any resource.

**How it works**:
1. Ensures `metadata.labels` exists using Lua's `or` pattern
2. Adds `glua.maurice.fr/processed="true"` label
3. Adds timestamp in ISO 8601 format using `os.date()`

**Use cases**:
- Track which resources have been processed by webhooks
- Add audit timestamps to resources
- Basic mutation demonstration

**Example**:
```bash
$ kubectl get pod nginx -o json | glua-webhook exec --script add-label.lua
# Output: Pod with added labels:
#   glua.maurice.fr/processed: "true"
#   glua.maurice.fr/timestamp: "2025-10-28T22:00:00Z"
```

---

### inject-sidecar.lua

**Purpose**: Injects a Fluent Bit logging sidecar container into Pods.

**How it works**:
1. Only processes `Pod` resources (returns early for others)
2. Checks if sidecar already exists to be idempotent
3. Adds `log-collector` container with volume mount
4. Adds `varlog` hostPath volume for accessing logs

**Use cases**:
- Automatic log collection for all Pods
- Adding monitoring sidecars
- Demonstrating container array manipulation

**Example**:
```bash
$ kubectl get pod nginx -o json | glua-webhook exec --script inject-sidecar.lua
# Output: Pod with added sidecar:
#   containers[1].name: log-collector
#   containers[1].image: fluent/fluent-bit:latest
```

**Note**: Idempotent - running multiple times won't add duplicate sidecars.

---

### add-annotations.lua

**Purpose**: Adds structured mutation metadata as JSON annotation.

**How it works**:
1. Requires `json` module for encoding
2. Requires `time` module for Unix timestamp
3. Creates metadata object with webhook info
4. Encodes as JSON and stores in annotation

**Use cases**:
- Audit trail of mutations
- Storing structured metadata
- Demonstrating glua module usage

**Example**:
```bash
$ kubectl get pod nginx -o json | glua-webhook exec --script add-annotations.lua
# Output: Pod with annotation:
#   glua.maurice.fr/mutation-info: '{"mutated_by":"glua-webhook","mutation_time":1761688528,"script":"add-annotations.lua"}'
```

**Glua modules used**:
- `json.stringify()` - Encodes Lua table to JSON
- `time.now()` - Returns Unix timestamp

---

### validate-labels.lua

**Purpose**: Validates that required labels exist and are non-empty.

**How it works**:
1. Defines list of required label keys
2. Checks metadata and labels exist
3. Iterates through required labels
4. Calls `error()` if any label missing or empty
5. Prints success message if validation passes

**Use cases**:
- Enforce labeling policies
- Prevent misconfigured resources
- Demonstrate validation webhooks

**Example success**:
```bash
$ echo '{"kind":"Pod","metadata":{"labels":{"app":"nginx","env":"prod"}}}' | \
  glua-webhook exec --script validate-labels.lua
All required labels present
{"kind":"Pod","metadata":{"labels":{"app":"nginx","env":"prod"}}}
```

**Example failure**:
```bash
$ echo '{"kind":"Pod","metadata":{"labels":{"app":"nginx"}}}' | \
  glua-webhook exec --script validate-labels.lua
Error executing script: Required label 'env' is missing
```

**Note**: Use in ValidatingAdmissionWebhook, not MutatingAdmissionWebhook.

---

### propagate-deployment-labels.lua

**Purpose**: Finds labels matching pattern and appends `-pod` suffix to values.

**How it works**:
1. Only processes `Pod` resources
2. Iterates through all labels
3. Matches labels with key pattern `foo.bar/baz*`
4. Appends `-pod` to the value
5. Logs each modification using `log` module

**Use cases**:
- Propagate Deployment labels to Pods with modifications
- Transform label values during admission
- Demonstrate pattern matching and logging

**Example**:
```bash
$ echo '{"kind":"Pod","metadata":{"labels":{"foo.bar/baz":"hello=true","app":"nginx"}}}' | \
  glua-webhook exec --script propagate-deployment-labels.lua
INFO: Propagating label foo.bar/baz: hello=true -> hello=true-pod
INFO: Propagated 1 labels with -pod suffix
# Output: foo.bar/baz="hello=true-pod"
```

**Glua modules used**:
- `log.info()` - Logs informational messages

**Pattern matching**:
- `^foo%.bar/baz` matches labels starting with `foo.bar/baz`
- `.` is escaped as `%.` in Lua patterns

---

## Deploying Scripts

1. Create ConfigMap with script:
```bash
kubectl create configmap my-script \
  --from-file=script.lua=examples/scripts/add-label.lua \
  -n glua-webhook
```

2. Reference in resource annotation:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mypod
  annotations:
    glua.maurice.fr/scripts: "glua-webhook/my-script"
spec:
  containers:
  - name: nginx
    image: nginx
```

3. Webhook executes script on admission.

See [main documentation](../../docs/) for complete deployment guides.

## Writing Your Own Scripts

Key patterns used in these examples:

```lua
-- Safe table initialization
object.metadata = object.metadata or {}
object.metadata.labels = object.metadata.labels or {}

-- Early return for resource filtering
if object.kind ~= "Pod" then return end

-- Error handling for validation
if not condition then
  error("Validation failed: reason")
end

-- Using glua modules
local json = require("json")
local log = require("log")
```

For complete scripting guide, see [docs/guides/writing-scripts.md](../../docs/guides/writing-scripts.md).
