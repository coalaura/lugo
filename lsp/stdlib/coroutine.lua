---@meta coroutine

---@class coroutinelib
coroutine = {}

---@param f async fun(...):...
---@return thread
---@nodiscard
function coroutine.create(f) end

---@param co? thread
---@return boolean
---@nodiscard
function coroutine.isyieldable(co) end
---@version >5.2
---@return boolean
---@nodiscard
function coroutine.isyieldable() end

---@version >5.4
---@param co thread
---@return boolean noerror
---@return any errorobject
function coroutine.close(co) end

---@param co    thread
---@param val1? any
---@return boolean success
---@return any ...
function coroutine.resume(co, val1, ...) end

---@return thread running
---@return boolean ismain
---@nodiscard
function coroutine.running() end

---@param co thread
---@return
---| '"running"'   # ---| '"suspended"' # ---| '"normal"'    # ---| '"dead"'      # ---@nodiscard
function coroutine.status(co) end

---@param f async fun(...):...
---@return fun(...):...
---@nodiscard
function coroutine.wrap(f) end

---@async
---@return any ...
function coroutine.yield(...) end

return coroutine
