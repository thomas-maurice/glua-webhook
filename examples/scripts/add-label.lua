-- add-label.lua: Adds processing labels to any resource

object.metadata = object.metadata or {}
object.metadata.labels = object.metadata.labels or {}

object.metadata.labels["glua.maurice.fr/processed"] = "true"
object.metadata.labels["glua.maurice.fr/timestamp"] = os.date("%Y-%m-%dT%H:%M:%SZ")
