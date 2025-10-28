-- add-annotations.lua: Adds annotations with JSON encoding
-- This script demonstrates using glua modules (json, time)

local json = require("json")
local time = require("time")

if object.metadata == nil then
	object.metadata = {}
end

if object.metadata.annotations == nil then
	object.metadata.annotations = {}
end

-- Add metadata about the mutation
local mutation_info = {
	mutated_by = "glua-webhook",
	mutation_time = time.now(),
	script = "add-annotations.lua"
}

local encoded, err = json.stringify(mutation_info)
if not err then
	object.metadata.annotations["glua.maurice.fr/mutation-info"] = encoded
end
