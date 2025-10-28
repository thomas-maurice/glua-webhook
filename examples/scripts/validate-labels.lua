-- validate-labels.lua: Validates required labels are present

local required_labels = {"app", "env"}

if not object.metadata or not object.metadata.labels then
	error("Resource must have labels")
end

for _, label in ipairs(required_labels) do
	if not object.metadata.labels[label] or object.metadata.labels[label] == "" then
		error("Required label '" .. label .. "' is missing")
	end
end

print("All required labels present")
