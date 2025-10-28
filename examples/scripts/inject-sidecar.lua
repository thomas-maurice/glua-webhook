-- inject-sidecar.lua: Injects a logging sidecar into Pods

if object.kind ~= "Pod" then return end

object.spec = object.spec or {}
object.spec.containers = object.spec.containers or {}
object.spec.volumes = object.spec.volumes or {}

-- Check if sidecar already exists
for i = 1, #object.spec.containers do
	if object.spec.containers[i].name == "log-collector" then
		return
	end
end

-- Add sidecar container
table.insert(object.spec.containers, {
	name = "log-collector",
	image = "fluent/fluent-bit:latest",
	volumeMounts = {{name = "varlog", mountPath = "/var/log", readOnly = true}}
})

-- Add volume
table.insert(object.spec.volumes, {
	name = "varlog",
	hostPath = {path = "/var/log"}
})
