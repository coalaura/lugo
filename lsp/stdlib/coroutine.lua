---@meta
coroutine = {}

---Closes and dead-stops the coroutine co.
---@param co thread
---@return boolean, any ...
function coroutine.close(co) end

---Creates a new coroutine, with body f.
---@param f function
---@return thread
function coroutine.create(f) end

---Returns true when the running coroutine can yield.
---@param co? thread
---@return boolean
function coroutine.isyieldable(co) end

---Starts or continues the execution of coroutine co.
---@param co thread
---@param ... any
---@return boolean, any ...
function coroutine.resume(co, ...) end

---Returns the running coroutine plus a boolean, true when the running coroutine is the main one.
---@return thread, boolean
function coroutine.running() end

---Returns the status of coroutine co, as a string.
---@param co thread
---@return "running"|"suspended"|"normal"|"dead"
function coroutine.status(co) end

---Creates a new coroutine, with body f. Returns a function that resumes the coroutine each time it is called.
---@param f function
---@return function
function coroutine.wrap(f) end

---Suspends the execution of the calling coroutine.
---@param ... any
---@return any ...
function coroutine.yield(...) end
