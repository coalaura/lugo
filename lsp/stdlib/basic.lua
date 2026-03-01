---@meta

---@type string
_VERSION = "Lua 5.4"

---A global variable (not a function) that holds the global environment.
---Lua itself does not use this variable; changing its value does not affect any environment, nor vice versa.
---@class _G
_G = {}

---@type table
arg = {}

---Issues an error when the value of its argument v is false (i.e., nil or false).
---@param v any
---@param message? any
---@return any
function assert(v, message) end

---Performs a full garbage-collection cycle or other GC operations.
---@param opt? "collect"|"stop"|"restart"|"count"|"step"|"setpause"|"setstepmul"|"isrunning"
---@param arg? integer
---@return any
function collectgarbage(opt, arg) end

---Opens the named file and executes its contents as a Lua chunk.
---@param filename? string
---@return any
function dofile(filename) end

---Terminates the last protected function called and returns message as the error object.
---@param message any
---@param level? integer
function error(message, level) end

---Returns the metatable of the given object.
---@param object any
---@return table|nil
function getmetatable(object) end

---Returns three values (an iterator function, the table t, and 0) so that the construction will iterate over the key–value pairs (1,t[1]), (2,t[2]), etc.
---@param t table
---@return function, table, integer
function ipairs(t) end

---Loads a chunk.
---@param chunk string|function
---@param chunkname? string
---@param mode? "b"|"t"|"bt"
---@param env? table
---@return function|nil, string?
function load(chunk, chunkname, mode, env) end

---Similar to load, but gets the chunk from file filename.
---@param filename? string
---@param mode? "b"|"t"|"bt"
---@param env? table
---@return function|nil, string?
function loadfile(filename, mode, env) end

---Custom global logger mapped in your dump.
---@param ... any
function log(...) end

---Allows a program to traverse all fields of a table.
---@param table table
---@param index? any
---@return any, any
function next(table, index) end

---If t has a metamethod __pairs, calls it with t as argument and returns the first three results from the call.
---@param t table
---@return function, table, any
function pairs(t) end

---Calls function f with the given arguments in protected mode.
---@param f function
---@param ... any
---@return boolean, any ...
function pcall(f, ...) end

---Receives any number of arguments and prints their values to stdout.
---@param ... any
function print(...) end

---Checks whether v1 is equal to v2, without invoking the __eq metamethod.
---@param v1 any
---@param v2 any
---@return boolean
function rawequal(v1, v2) end

---Gets the real value of table[index], without invoking the __index metamethod.
---@param table table
---@param index any
---@return any
function rawget(table, index) end

---Returns the length of the object v, without invoking the __len metamethod.
---@param v table|string
---@return integer
function rawlen(v) end

---Sets the real value of table[index] to value, without invoking the __newindex metamethod.
---@param table table
---@param index any
---@param value any
---@return table
function rawset(table, index, value) end

---Loads the given module.
---@param modname string
---@return any
function require(modname) end

---If index is a number, returns all arguments after argument number index; a negative number indexes from the end. If index is "#", returns the total number of extra arguments.
---@param index integer|"#"
---@param ... any
---@return any
function select(index, ...) end

---Sets the metatable for the given table.
---@param table table
---@param metatable table|nil
---@return table
function setmetatable(table, metatable) end

---When called with no base, tonumber tries to convert its argument to a number.
---@param e any
---@param base? integer
---@return number|nil
function tonumber(e, base) end

---Receives a value of any type and converts it to a string in a human-readable format.
---@param v any
---@return string
function tostring(v) end

---Returns the type of its only argument, coded as a string.
---@param v any
---@return string
function type(v) end

---Emits a warning with the given message components.
---@param ... string
function warn(...) end

---Similar to pcall, but sets a new message handler msgh.
---@param f function
---@param msgh function
---@param ... any
---@return boolean, any ...
function xpcall(f, msgh, ...) end
