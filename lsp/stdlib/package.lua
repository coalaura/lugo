---@meta
package = {}

---A string that describes some compile-time configurations for packages.
---@type string
package.config = "\\\n;\n?\n!\n-\n"

---The path used by require to search for a C loader.
---@type string
package.cpath = ""

---A table used by require to control which modules are already loaded.
---@type table
package.loaded = {}

---The path used by require to search for a Lua loader.
---@type string
package.path = ""

---A table to store loaders for specific modules.
---@type table
package.preload = {}

---A table used by require to control how to load modules.
---@type table
package.searchers = {}

---Dynamically links the host program with the C library libname.
---@param libname string
---@param funcname string
---@return function|nil, string?
function package.loadlib(libname, funcname) end

---Searches for the given name in the given path.
---@param name string
---@param path string
---@param sep? string
---@param rep? string
---@return string|nil, string?
function package.searchpath(name, path, sep, rep) end
