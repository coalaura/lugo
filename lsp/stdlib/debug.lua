---@meta
debug = {}

---Enters an interactive mode with the user, running each string that the user enters.
function debug.debug() end

---Returns the current hook settings of the thread, as three values: the current hook function, the current hook mask, and the current hook count.
---@param thread? thread
---@return function|nil, string, integer
function debug.gethook(thread) end

---Returns a table with information about a function.
---@param thread? thread
---@param f function|integer
---@param what? string
---@return table|nil
function debug.getinfo(thread, f, what) end

---Returns the name and the value of the local variable with index local of the function at level f of the stack.
---@param thread? thread
---@param f integer|function
---@param locl integer
---@return string|nil, any
function debug.getlocal(thread, f, locl) end

---Returns the metatable of the given value or nil if it does not have one.
---@param value any
---@return table|nil
function debug.getmetatable(value) end

---Returns the registry table.
---@return table
function debug.getregistry() end

---Returns the name and the value of the upvalue with index up of the function.
---@param f function
---@param up integer
---@return string|nil, any
function debug.getupvalue(f, up) end

---Returns the n-th user value associated to the userdata u plus a boolean, false if the userdata does not have that value.
---@param u userdata
---@param n integer
---@return any, boolean
function debug.getuservalue(u, n) end

---Sets a new limit for the C stack.
---@param limit integer
---@return integer|boolean
function debug.setcstacklimit(limit) end

---Sets the given function as a hook.
---@param thread? thread
---@param hook function
---@param mask string
---@param count? integer
function debug.sethook(thread, hook, mask, count) end

---Assigns the value value to the local variable with index local of the function at level level of the stack.
---@param thread? thread
---@param level integer
---@param locl integer
---@param value any
---@return string|nil
function debug.setlocal(thread, level, locl, value) end

---Sets the metatable for the given value to the given table (which can be nil). Returns value.
---@param value any
---@param table table|nil
---@return any
function debug.setmetatable(value, table) end

---Assigns the value value to the upvalue with index up of the function.
---@param f function
---@param up integer
---@param value any
---@return string|nil
function debug.setupvalue(f, up, value) end

---Sets the given value as the n-th user value associated to the given userdata.
---@param udata userdata
---@param value any
---@param n integer
---@return userdata
function debug.setuservalue(udata, value, n) end

---Returns a string with a traceback of the call stack.
---@param thread? thread
---@param message? string
---@param level? integer
---@return string
function debug.traceback(thread, message, level) end

---Returns a unique identifier (as a light userdata) for the upvalue numbered n from the given function.
---@param f function
---@param n integer
---@return userdata
function debug.upvalueid(f, n) end

---Make the n1-th upvalue of the Lua closure f1 refer to the n2-th upvalue of the Lua closure f2.
---@param f1 function
---@param n1 integer
---@param f2 function
---@param n2 integer
function debug.upvaluejoin(f1, n1, f2, n2) end
