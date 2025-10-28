-- add-label.lua: Adds a label to the resource
-- This script demonstrates basic metadata modification

if object.metadata == nil then
	object.metadata = {}
end

if object.metadata.labels == nil then
	object.metadata.labels = {}
end

-- Add a label indicating this resource was processed
object.metadata.labels["glua.maurice.fr/processed"] = "true"
object.metadata.labels["glua.maurice.fr/timestamp"] = os.date("%Y-%m-%dT%H:%M:%SZ")
