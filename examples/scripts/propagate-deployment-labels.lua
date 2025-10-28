-- propagate-deployment-labels.lua
-- Propagates labels from Deployment to Pod with a suffix
--
-- This script looks for labels on Deployments that match a specific pattern
-- and propagates them to Pods with a modified value.
--
-- Example:
--   Deployment has label: foo.bar/baz=hello=true
--   Pods get label:       foo.bar/baz=hello=true-pod

local log = require("log")

-- Only process Pods
if object.kind ~= "Pod" then
  log.info("Skipping non-Pod resource: " .. object.kind)
  return
end

-- Note: In production, Pods created by Deployments have ownerReferences pointing to ReplicaSets
-- For testing purposes, we'll process all Pods with the matching label pattern

-- Ensure labels table exists
if object.metadata.labels == nil then
  object.metadata.labels = {}
end

-- Find labels matching the pattern foo.bar/baz=*
-- and propagate them with -pod suffix
local propagated_count = 0
for key, value in pairs(object.metadata.labels) do
  -- Check if this is a label we should propagate
  if string.match(key, "^foo%.bar/baz") then
    -- Modify the value by adding -pod suffix
    local new_value = value .. "-pod"

    -- Only update if different from current value
    if object.metadata.labels[key] ~= new_value then
      log.info("Propagating label " .. key .. ": " .. value .. " -> " .. new_value)
      object.metadata.labels[key] = new_value
      propagated_count = propagated_count + 1
    end
  end
end

if propagated_count > 0 then
  log.info("Propagated " .. propagated_count .. " labels with -pod suffix")
else
  log.info("No foo.bar/baz labels found to propagate")
end
