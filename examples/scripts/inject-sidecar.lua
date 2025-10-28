-- inject-sidecar.lua: Injects a logging sidecar container into Pods
-- This script demonstrates container manipulation for Pods

-- Only process Pod resources
if object.kind ~= "Pod" then
	return
end

-- Ensure spec and containers exist
if object.spec == nil then
	object.spec = {}
end

if object.spec.containers == nil then
	object.spec.containers = {}
end

-- Check if sidecar already exists
local has_sidecar = false
for i = 1, #object.spec.containers do
	if object.spec.containers[i].name == "log-collector" then
		has_sidecar = true
		break
	end
end

-- Add sidecar if it doesn't exist
if not has_sidecar then
	table.insert(object.spec.containers, {
		name = "log-collector",
		image = "fluent/fluent-bit:latest",
		volumeMounts = {
			{
				name = "varlog",
				mountPath = "/var/log",
				readOnly = true
			}
		}
	})

	-- Add volume if it doesn't exist
	if object.spec.volumes == nil then
		object.spec.volumes = {}
	end

	table.insert(object.spec.volumes, {
		name = "varlog",
		hostPath = {
			path = "/var/log"
		}
	})
end
