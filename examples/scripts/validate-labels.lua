-- validate-labels.lua: Validates that required labels are present
-- This script demonstrates validation logic

local required_labels = {"app", "env"}

if object.metadata == nil or object.metadata.labels == nil then
	error("Resource must have labels")
end

for _, label in ipairs(required_labels) do
	if object.metadata.labels[label] == nil or object.metadata.labels[label] == "" then
		error("Required label '" .. label .. "' is missing")
	end
end

print("All required labels present")
