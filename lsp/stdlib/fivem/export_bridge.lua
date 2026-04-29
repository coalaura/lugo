---@meta

---Event-backed cross-resource export bridge.
---`exports[resource][name]` lazily resolves exported callables through synthetic `__cfx_export_<resource>_<name>` events.
exports = {}
