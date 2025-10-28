-- add-annotations.lua: Adds mutation metadata using glua modules

local json = require("json")
local time = require("time")

object.metadata = object.metadata or {}
object.metadata.annotations = object.metadata.annotations or {}

local mutation_info = {
	mutated_by = "glua-webhook",
	mutation_time = time.now(),
	script = "add-annotations.lua"
}

local encoded, err = json.stringify(mutation_info)
if not err then
	object.metadata.annotations["glua.maurice.fr/mutation-info"] = encoded
end
