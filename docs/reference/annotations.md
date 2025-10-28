# Annotations Reference

This document describes all annotations used by glua-webhook.

## Resource Annotations

Annotations are specified on individual Kubernetes resources to control webhook behavior.

### `glua.maurice.fr/scripts`

**Description**: Specifies which Lua scripts to run against this resource.

**Format**: Comma-separated list of ConfigMap references in `namespace/name` format.

**Example**:

```yaml
metadata:
  annotations:
    glua.maurice.fr/scripts: "default/script1,kube-system/script2,default/script3"
```

**Behavior**:
- Scripts are executed in **alphabetical order** by ConfigMap name
- Each script gets its own isolated Lua VM instance
- Failed scripts are logged but don't block admission (per `failurePolicy: Ignore`)
- The output of one script becomes the input to the next

**ConfigMap Format**:

Each ConfigMap must contain a key named `script.lua`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-script
  namespace: default
data:
  script.lua: |
    -- Your Lua code here
    object.metadata.labels["processed"] = "true"
```

## Namespace Labels

Labels are specified on namespaces to enable/disable webhooks.

### `glua.maurice.fr/enabled`

**Description**: Enables the MutatingAdmissionWebhook for this namespace.

**Values**: `"true"` or `"false"`

**Example**:

```bash
kubectl label namespace default glua.maurice.fr/enabled=true
```

**Behavior**:
- Only resources in namespaces with this label set to `"true"` are processed by the mutating webhook
- Removing this label or setting it to `"false"` disables mutation for the namespace

### `glua.maurice.fr/validation-enabled`

**Description**: Enables the ValidatingAdmissionWebhook for this namespace.

**Values**: `"true"` or `"false"`

**Example**:

```bash
kubectl label namespace production glua.maurice.fr/validation-enabled=true
```

**Behavior**:
- Only resources in namespaces with this label set to `"true"` are processed by the validating webhook
- Removing this label or setting it to `"false"` disables validation for the namespace

## Common Patterns

### Enable Both Webhooks

```bash
kubectl label namespace default \
  glua.maurice.fr/enabled=true \
  glua.maurice.fr/validation-enabled=true
```

### Environment-Specific Scripts

```yaml
# Production pods - strict validation
apiVersion: v1
kind: Pod
metadata:
  namespace: production
  annotations:
    glua.maurice.fr/scripts: "common/security-check,production/validate-resources"
spec:
  # ...

---
# Development pods - permissive
apiVersion: v1
kind: Pod
metadata:
  namespace: development
  annotations:
    glua.maurice.fr/scripts: "common/add-labels"
spec:
  # ...
```

### Sequential Processing

Use naming to control execution order:

```yaml
metadata:
  annotations:
    # These run in this exact order:
    glua.maurice.fr/scripts: "default/01-validate,default/02-set-defaults,default/99-add-labels"
```

### Conditional Processing

Scripts can check resource properties:

```yaml
# This Pod will only be processed by the sidecar script if it has the label
apiVersion: v1
kind: Pod
metadata:
  labels:
    inject-sidecar: "true"
  annotations:
    glua.maurice.fr/scripts: "default/conditional-sidecar"
spec:
  # ...
```

```lua
-- conditional-sidecar.lua
if object.metadata.labels == nil or object.metadata.labels["inject-sidecar"] ~= "true" then
  return
end

-- Inject sidecar
-- ...
```

## Script Resolution

### Resolution Order

1. Parse annotation value
2. Split by comma
3. For each reference:
   - Extract namespace and ConfigMap name
   - Fetch ConfigMap from Kubernetes API
   - Extract `script.lua` key
   - Load into script collection
4. Sort scripts alphabetically by full reference (`namespace/name`)
5. Execute in order

### Example

Given annotation:

```yaml
glua.maurice.fr/scripts: "kube-system/z-script,default/a-script,default/m-script"
```

Execution order:
1. `default/a-script`
2. `default/m-script`
3. `kube-system/z-script`

(Alphabetical by `namespace/name`)

## Error Handling

### ConfigMap Not Found

If a referenced ConfigMap doesn't exist:
- Error is logged
- Admission request is **rejected**
- Resource creation fails

```
ERROR: Failed to load scripts: failed to fetch ConfigMap default/missing-script: configmaps "missing-script" not found
```

### Missing `script.lua` Key

If a ConfigMap exists but doesn't have `script.lua` key:
- Warning is logged
- Script is skipped
- Other scripts continue executing
- Admission request is **allowed**

```
WARNING: ConfigMap default/bad-script does not contain 'script.lua' key
```

### Script Execution Error

If a script fails during execution:
- Error is logged
- Script is skipped
- Other scripts continue executing
- Admission request is **allowed** (per `failurePolicy: Ignore`)

```
WARNING: Script default/buggy-script failed (ignoring): script execution failed: <string>:10: attempt to index a nil value
```

## Limits and Constraints

### Annotation Size

Kubernetes annotations have a maximum size of **256KB** for all annotations combined. Keep your script references concise.

**Bad** (too many scripts):
```yaml
glua.maurice.fr/scripts: "ns1/script1,ns1/script2,ns1/script3,...,ns1/script100"
```

**Good** (combine related logic):
```yaml
glua.maurice.fr/scripts: "ns1/consolidated-script,ns1/validation"
```

### ConfigMap Size

ConfigMaps have a maximum size of **1MB**. Keep scripts focused and under 10KB each.

### Number of Scripts

While there's no hard limit, consider:
- Each script adds ~1-5ms latency
- More than 10 scripts per resource is excessive
- Combine related mutations into single scripts

### Timeout

The webhook has a 10-second timeout (`timeoutSeconds: 10`). If all scripts combined take longer:
- Admission request is **allowed** (per `failurePolicy: Ignore`)
- Warning is logged

## Best Practices

### 1. Use Clear Names

```bash
# Good
glua.maurice.fr/scripts: "security/pod-security-policy,monitoring/inject-metrics"

# Bad
glua.maurice.fr/scripts: "default/s1,default/s2"
```

### 2. Group by Function

Store scripts in namespaces that reflect their purpose:

- `security/*` - Security-related mutations
- `monitoring/*` - Monitoring sidecars and labels
- `compliance/*` - Compliance checks
- `common/*` - Shared utilities

### 3. Document Your Scripts

Add description in ConfigMap metadata:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: inject-monitoring
  namespace: monitoring
  annotations:
    description: "Injects Prometheus metrics sidecar into all Pods"
    version: "1.0.0"
    author: "platform-team"
data:
  script.lua: |
    -- Script implementation
```

### 4. Version Your Scripts

Use semantic versioning in ConfigMap names or labels:

```yaml
glua.maurice.fr/scripts: "security/pod-security-v2,monitoring/metrics-v1"
```

## Troubleshooting

### Script Not Running

1. Check namespace labels:
   ```bash
   kubectl get namespace default -o jsonpath='{.metadata.labels}'
   ```

2. Verify annotation format (must include namespace):
   ```yaml
   # Wrong
   glua.maurice.fr/scripts: "my-script"

   # Correct
   glua.maurice.fr/scripts: "default/my-script"
   ```

3. Check ConfigMap exists:
   ```bash
   kubectl get cm my-script -n default
   ```

### Script Order Issues

1. Check alphabetical ordering:
   ```bash
   # These run in this order:
   # default/a-script
   # default/b-script
   # kube-system/c-script
   glua.maurice.fr/scripts: "default/b-script,kube-system/c-script,default/a-script"
   ```

2. Use numeric prefixes for explicit ordering:
   ```yaml
   glua.maurice.fr/scripts: "default/01-first,default/02-second,default/03-third"
   ```

### Performance Issues

1. Check webhook logs for timing:
   ```bash
   kubectl logs -n glua-webhook deployment/glua-webhook | grep "Script.*completed"
   ```

2. Reduce number of scripts:
   ```yaml
   # Before: 3 scripts, ~15ms
   glua.maurice.fr/scripts: "ns/script1,ns/script2,ns/script3"

   # After: 1 consolidated script, ~5ms
   glua.maurice.fr/scripts: "ns/consolidated"
   ```

## See Also

- [Writing Lua Scripts](../guides/writing-scripts.md)
- [Configuration Reference](configuration.md)
- [Examples](../examples/index.md)
