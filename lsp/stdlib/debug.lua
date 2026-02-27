---@meta debug

---@class debuglib
debug = {}

---@class debuginfo
---@field name            string
---@field namewhat        string
---@field source          string
---@field short_src       string
---@field linedefined     integer
---@field lastlinedefined integer
---@field what            string
---@field currentline     integer
---@field istailcall      boolean
---@field nups            integer
---@field nparams         integer
---@field isvararg        boolean
---@field func            function
---@field ftransfer       integer
---@field ntransfer       integer
---@field activelines     table

function debug.debug() end

---@version 5.1
---@param o any
---@return table
---@nodiscard
function debug.getfenv(o) end

---@param co? thread
---@return function hook
---@return string mask
---@return integer count
---@nodiscard
function debug.gethook(co) end

---@alias infowhat string
---|+"n"     # ---|+"S"     # ---|+"l"     # ---|+"t"     # ---|+"u" # ---|+"u" # ---|+"f"     # ---|+"r"     # ---|+"L"     #
---@overload fun(f: integer|function, what?: infowhat):debuginfo
---@param thread thread
---@param f      integer|async fun(...):...
---@param what?  infowhat
---@return debuginfo
---@nodiscard
function debug.getinfo(thread, f, what) end

---@overload fun(level: integer, index: integer):string, any
---@param thread  thread
---@param level   integer
---@param index   integer
---@return string name
---@return any    value
---@nodiscard
function debug.getlocal(thread, level, index) end
---@overload fun(f: integer|async fun(...):..., index: integer):string, any
---@param thread  thread
---@param f       integer|async fun(...):...
---@param index   integer
---@return string name
---@return any    value
---@nodiscard
function debug.getlocal(thread, f, index) end

---@param object any
---@return table metatable
---@nodiscard
function debug.getmetatable(object) end

---@return table
---@nodiscard
function debug.getregistry() end

---@param f  async fun(...):...
---@param up integer
---@return string name
---@return any    value
---@nodiscard
function debug.getupvalue(f, up) end

---@param u  userdata
---@param n? integer
---@return any
---@return boolean
---@nodiscard
function debug.getuservalue(u, n) end
---@param u userdata
---@return any
---@nodiscard
function debug.getuservalue(u) end

---@deprecated
---@param limit integer
---@return integer|boolean
function debug.setcstacklimit(limit) end

---@version 5.1
---@generic T
---@param object T
---@param env    table
---@return T object
function debug.setfenv(object, env) end

---@alias hookmask string
---|+"c" # ---|+"r" # ---|+"l" #
---@overload fun(hook: (async fun(...):...), mask: hookmask, count?: integer)
---@overload fun(thread: thread):...
---@overload fun(...):...
---@param thread thread
---@param hook   async fun(...):...
---@param mask   hookmask
---@param count? integer
function debug.sethook(thread, hook, mask, count) end

---@overload fun(level: integer, index: integer, value: any):string
---@param thread thread
---@param level  integer
---@param index  integer
---@param value  any
---@return string name
function debug.setlocal(thread, level, index, value) end

---@generic T
---@param value T
---@param meta? table
---@return T value
function debug.setmetatable(value, meta) end

---@param f     async fun(...):...
---@param up    integer
---@param value any
---@return string name
function debug.setupvalue(f, up, value) end

---@param udata userdata
---@param value any
---@param n?    integer
---@return userdata udata
function debug.setuservalue(udata, value, n) end
---@param udata userdata
---@param value any
---@return userdata udata
function debug.setuservalue(udata, value) end

---@overload fun(message?: any, level?: integer): string
---@param thread   thread
---@param message? any
---@param level?   integer
---@return string  message
---@nodiscard
function debug.traceback(thread, message, level) end

---@version >5.2, JIT
---@param f async fun(...):...
---@param n integer
---@return lightuserdata id
---@nodiscard
function debug.upvalueid(f, n) end

---@version >5.2, JIT
---@param f1 async fun(...):...
---@param n1 integer
---@param f2 async fun(...):...
---@param n2 integer
function debug.upvaluejoin(f1, n1, f2, n2) end

return debug
