-- propagate-deployment-labels.lua: Propagates labels with -pod suffix
-- Example: foo.bar/baz="hello=true" -> foo.bar/baz="hello=true-pod"

local log = require("log")

if object.kind ~= "Pod" then return end

object.metadata.labels = object.metadata.labels or {}

local count = 0
for key, value in pairs(object.metadata.labels) do
  if string.match(key, "^foo%.bar/baz") then
    local new_value = value .. "-pod"
    if object.metadata.labels[key] ~= new_value then
      log.info("Propagating label " .. key .. ": " .. value .. " -> " .. new_value)
      object.metadata.labels[key] = new_value
      count = count + 1
    end
  end
end

if count > 0 then
  log.info("Propagated " .. count .. " labels with -pod suffix")
end
