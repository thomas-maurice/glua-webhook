# Writing Lua Scripts

This guide covers everything you need to know to write effective Lua scripts for glua-webhook.

## Basic Concepts

### The `object` Global

Every script receives a global variable named `object` that contains the Kubernetes resource as a Lua table:

```lua
-- Access fields
local name = object.metadata.name
local namespace = object.metadata.namespace
local kind = object.kind

-- Modify fields
object.metadata.labels["my-label"] = "my-value"
```

The object structure matches the Kubernetes resource structure converted to Lua tables.

### No Return Statement Needed

You don't need to return the modified object - the webhook automatically uses the modified `object` global:

```lua
-- This is all you need
object.metadata.labels["modified"] = "true"

-- No need for: return object
```

### Nil Handling

Always check for nil before accessing nested fields:

```lua
-- Bad - will error if metadata is nil
object.metadata.labels["key"] = "value"

-- Good - creates tables as needed
if object.metadata == nil then
  object.metadata = {}
end
if object.metadata.labels == nil then
  object.metadata.labels = {}
end
object.metadata.labels["key"] = "value"
```

## Available Modules

### JSON Module

```lua
local json = require("json")

-- Parse JSON string
local data, err = json.parse('{"name":"John","age":30}')
if err then
  error("Parse failed: " .. err)
end

-- Convert to JSON string
local str, err = json.stringify({name="Jane", age=25})
if err then
  error("Stringify failed: " .. err)
end

-- Use in annotations
object.metadata.annotations["data"] = str
```

### YAML Module

```lua
local yaml = require("yaml")

-- Parse YAML
local data, err = yaml.parse([[
name: John
age: 30
]])

-- Convert to YAML
local str, err = yaml.stringify({name="Jane", age=25})
```

### Time Module

```lua
local time = require("time")

-- Get current time
local now = time.now()

-- Format time
local formatted = time.format(now, "%Y-%m-%d")

-- Add to labels
object.metadata.labels["created-at"] = formatted
```

### Hash Module

```lua
local hash = require("hash")

-- SHA256 hash
local h = hash.sha256("mydata")

-- MD5 hash
local h = hash.md5("mydata")

-- Use in labels
object.metadata.labels["content-hash"] = h
```

### Base64 Module

```lua
local base64 = require("base64")

-- Encode
local encoded = base64.encode("hello")

-- Decode
local decoded = base64.decode(encoded)
```

### HTTP Module

```lua
local http = require("http")

-- GET request
local response, err = http.get("https://api.example.com/data")
if err then
  error("HTTP request failed: " .. err)
end

-- Parse response body
local json = require("json")
local data, err = json.parse(response.body)
```

### Log Module

```lua
local log = require("log")

-- Log messages (appears in webhook logs)
log.info("Processing resource: " .. object.metadata.name)
log.warn("Warning message")
log.error("Error message")
```

### Template Module

```lua
local template = require("template")

-- Render template
local tmpl = "Hello, {{.name}}!"
local result, err = template.render(tmpl, {name="World"})
-- result = "Hello, World!"
```

## Common Patterns

### Conditional Mutations

Only mutate specific resource types:

```lua
-- Only process Pods
if object.kind ~= "Pod" then
  return
end

-- Only process resources in specific namespace
if object.metadata.namespace ~= "production" then
  return
end

-- Only process resources with specific label
if object.metadata.labels == nil or object.metadata.labels["app"] ~= "myapp" then
  return
end
```

### Adding Labels

```lua
-- Ensure metadata and labels exist
if object.metadata == nil then
  object.metadata = {}
end
if object.metadata.labels == nil then
  object.metadata.labels = {}
end

-- Add labels
object.metadata.labels["environment"] = "production"
object.metadata.labels["managed-by"] = "glua-webhook"
object.metadata.labels["timestamp"] = os.date("%Y-%m-%dT%H:%M:%SZ")
```

### Adding Annotations

```lua
-- Ensure annotations exist
if object.metadata == nil then
  object.metadata = {}
end
if object.metadata.annotations == nil then
  object.metadata.annotations = {}
end

-- Add annotation
object.metadata.annotations["description"] = "Processed by Lua script"
```

### Injecting Sidecar Containers

```lua
-- Only for Pods
if object.kind ~= "Pod" then
  return
end

-- Ensure containers array exists
if object.spec == nil then
  object.spec = {}
end
if object.spec.containers == nil then
  object.spec.containers = {}
end

-- Check if sidecar already exists
local has_sidecar = false
for i = 1, #object.spec.containers do
  if object.spec.containers[i].name == "my-sidecar" then
    has_sidecar = true
    break
  end
end

-- Add sidecar if missing
if not has_sidecar then
  table.insert(object.spec.containers, {
    name = "my-sidecar",
    image = "my-sidecar:latest",
    ports = {
      {
        containerPort = 8080,
        name = "http"
      }
    }
  })
end
```

### Validation

```lua
-- Validate required labels
local required_labels = {"app", "env", "owner"}

if object.metadata == nil or object.metadata.labels == nil then
  error("Resource must have labels")
end

for _, label in ipairs(required_labels) do
  if object.metadata.labels[label] == nil or object.metadata.labels[label] == "" then
    error("Required label '" .. label .. "' is missing")
  end
end

-- Validate naming convention
if not string.match(object.metadata.name, "^[a-z][a-z0-9-]*$") then
  error("Name must match pattern ^[a-z][a-z0-9-]*$")
end
```

### Setting Defaults

```lua
-- Set default resource requests
if object.kind == "Pod" then
  if object.spec and object.spec.containers then
    for i = 1, #object.spec.containers do
      local container = object.spec.containers[i]

      -- Ensure resources section exists
      if container.resources == nil then
        container.resources = {}
      end
      if container.resources.requests == nil then
        container.resources.requests = {}
      end

      -- Set defaults if not specified
      if container.resources.requests.cpu == nil then
        container.resources.requests.cpu = "100m"
      end
      if container.resources.requests.memory == nil then
        container.resources.requests.memory = "128Mi"
      end
    end
  end
end
```

## Best Practices

### 1. Keep Scripts Focused

Each script should do one thing well:

```lua
-- Good: add-labels.lua
object.metadata.labels["env"] = "prod"

-- Good: add-annotations.lua
object.metadata.annotations["owner"] = "platform-team"

-- Bad: do-everything.lua (100 lines doing multiple things)
```

### 2. Use Descriptive Names

Name your ConfigMaps clearly:

- `add-monitoring-labels`
- `inject-logging-sidecar`
- `validate-security-labels`
- `set-resource-defaults`

### 3. Handle Errors Gracefully

```lua
-- Bad
local data = json.parse(some_string)

-- Good
local data, err = json.parse(some_string)
if err then
  -- Log error but don't crash
  print("Warning: failed to parse JSON: " .. err)
  return
end
```

### 4. Use Alphabetical Ordering

Scripts run in alphabetical order, so use prefixes if order matters:

- `01-validate-required-labels.lua`
- `02-set-defaults.lua`
- `03-add-monitoring-labels.lua`
- `99-final-annotations.lua`

### 5. Test Locally First

Always test your scripts with the test framework:

```bash
go test -v ./test/script_test.go
```

### 6. Add Comments

Document your scripts:

```lua
-- inject-monitoring-sidecar.lua
--
-- Injects a Prometheus metrics sidecar into all Pods
-- in the 'production' namespace.
--
-- The sidecar exposes metrics on port 9090.

if object.kind ~= "Pod" then
  return
end

-- Rest of script...
```

### 7. Avoid Long-Running Operations

Scripts should be fast (<100ms). Avoid:

- HTTP requests (unless necessary and fast)
- Complex calculations
- Large data processing

## Debugging Scripts

### Using Log Module

```lua
local log = require("log")

log.info("Script started for: " .. object.metadata.name)
log.info("Resource kind: " .. object.kind)

-- Log the entire object for debugging
local spew = require("spew")
log.info(spew.dump(object))
```

### Using Spew Module

```lua
local spew = require("spew")

-- Pretty-print the entire object
print(spew.dump(object))

-- Print specific fields
print(spew.dump(object.metadata))
```

### Testing with Dry Run

Test without actually creating resources:

```bash
kubectl apply -f my-resource.yaml --dry-run=server -o yaml
```

This will show you the mutated resource without creating it.

## Common Mistakes

### 1. Not Checking for Nil

```lua
-- Wrong - will error if labels doesn't exist
object.metadata.labels["key"] = "value"

-- Correct
if object.metadata.labels == nil then
  object.metadata.labels = {}
end
object.metadata.labels["key"] = "value"
```

### 2. Trying to Return Object

```lua
-- Wrong - not necessary
return object

-- Correct - just modify it
object.metadata.labels["key"] = "value"
```

### 3. Using Wrong JSON Functions

```lua
-- Wrong - these don't exist
local str = json.encode({})
local data = json.decode(str)

-- Correct - use stringify/parse
local str, err = json.stringify({})
local data, err = json.parse(str)
```

### 4. Forgetting Error Handling

```lua
-- Wrong - might panic if parse fails
local data = json.parse(some_string)

-- Correct - check errors
local data, err = json.parse(some_string)
if err then
  error("Failed to parse: " .. err)
end
```

## Next Steps

- [Examples](../examples/index.md)
- [API Reference](../reference/lua-api.md)
- [Testing Scripts](testing-scripts.md)
