---@meta

---Manifest-profile FiveM metadata bundle.
---Runtime ABI globals are intentionally unavailable while authoring `fxmanifest.lua` or `__resource.lua`.

---FiveM's JSON helper library is available while evaluating manifest directives.
---@class json
json = {}

---@param value any
---@return string
function json.encode(value) end

---@param payload string
---@return any
function json.decode(payload) end

---@type userdata|nil
json.null = nil

---The manifest runtime keeps the custom debug surface available for source tracking and metatable access.
debug = debug

---@param value any
---@return table|nil
function debug.getmetatable(value) end

---@param value any
---@param mt table|nil
---@return any
function debug.setmetatable(value, mt) end

---@param message? string
---@param level? integer
---@return string
function debug.traceback(message, level) end

---Selects the fxv2 manifest version for the resource.
---@param version string
function fx_version(version) end

---Declares the target game for the resource.
---@param gameName string
function game(gameName) end

---Declares additional target games for the resource.
---@param gameName string
function games(gameName) end

---Adds descriptive author metadata to the resource manifest.
---@param name string
function author(name) end

---Adds descriptive summary metadata to the resource manifest.
---@param text string
function description(text) end

---Adds descriptive version metadata to the resource manifest.
---@param value string
function version(value) end

---Enables Lua 5.4 semantics for the resource runtime.
---@param enabled? boolean|string
function lua54(enabled) end

---Enables the experimental fxv2 OAL bridge for native calls.
---@param enabled? boolean|string
function use_experimental_fxv2_oal(enabled) end

---Marks the resource as server-only.
---@param enabled? boolean|string
function server_only(enabled) end

---Declares a browser entrypoint for NUI content.
---@param path string
function ui_page(path) end

---Includes a client-side Lua file or glob.
---@param path string
function client_script(path) end

---Includes multiple client-side Lua files or globs.
---@param path string
function client_scripts(path) end

---Includes a server-side Lua file or glob.
---@param path string
function server_script(path) end

---Includes multiple server-side Lua files or globs.
---@param path string
function server_scripts(path) end

---Includes a shared Lua file or glob.
---@param path string
function shared_script(path) end

---Includes multiple shared Lua files or globs.
---@param path string
function shared_scripts(path) end

---Includes an additional resource file or glob.
---@param path string
function file(path) end

---Includes multiple additional resource files or globs.
---@param path string
function files(path) end

---Registers a client-visible export name.
---@param name string
function export(name) end

---Registers a client-visible export name explicitly.
---@param name string
function client_export(name) end

---Registers multiple client-visible export names explicitly.
---@param name string
function client_exports(name) end

---Registers a server-visible export name.
---@param name string
function server_export(name) end

---Registers multiple server-visible export names.
---@param name string
function server_exports(name) end

---Declares a dependency on another resource.
---@param resourceName string
function dependency(resourceName) end

---Declares multiple dependencies on other resources.
---@param resourceName string
function dependencies(resourceName) end

---Declares a provided alias for this resource.
---@param alias string
function provide(alias) end

---Declares multiple provided aliases for this resource.
---@param alias string
function provides(alias) end

---Declares a map data file entry.
---@param dataType string
---@param path string
function data_file(dataType, path) end

---Marks the resource as a map resource.
---@param enabled? boolean|string
function this_is_a_map(enabled) end

---Declares a loading-screen resource file.
---@param path string
function loadscreen(path) end

---Controls whether the loading screen requires manual shutdown.
---@param enabled? boolean|string
function loadscreen_manual_shutdown(enabled) end

---Replaces the active level meta file.
---@param path string
function replace_level_meta(path) end

---Adds a level meta file before the base level metadata.
---@param path string
function before_level_meta(path) end

---Adds a level meta file after the base level metadata.
---@param path string
function after_level_meta(path) end

---Marks files that should stay unencrypted in escrowed resources.
---@param path string
function escrow_ignore(path) end

---Selects the Node.js runtime version for JS resource scripts.
---@param version string
function node_version(version) end
