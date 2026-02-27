---@meta package

---@param modname string
---@return unknown
---@return unknown loaderdata
function require(modname) end
---@param modname string
---@return unknown
function require(modname) end

---@class packagelib
---@field cpath     string
---@field loaded    table
---@field path      string
---@field preload   table
package = {}

package.config = [[
/
;
?
!
-]]

---@version <5.1
package.loaders = {}

---@param libname string
---@param funcname string
---@return any
function package.loadlib(libname, funcname) end

---@version >5.2
package.searchers = {}

---@version >5.2,JIT
---@param name string
---@param path string
---@param sep? string
---@param rep? string
---@return string? filename
---@return string? errmsg
---@nodiscard
function package.searchpath(name, path, sep, rep) end

---@version <5.1
---@param module table
function package.seeall(module) end

return package
