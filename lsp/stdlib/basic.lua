---@meta _

---@type string[]
arg = {}

---@generic T
---@param v? T
---@param message? any
---@param ... any
---@return T
---@return any ...
function assert(v, message, ...) end

---@overload fun(opt?: "collect")
---@overload fun(opt: "stop")
---@overload fun(opt: "restart")
---@overload fun(opt: "count"): number
---@overload fun(opt: "step", arg: integer): true
---@overload fun(opt: "setpause", arg: integer): integer
---@overload fun(opt: "setstepmul", arg: integer): integer
function collectgarbage(...) end

---@overload fun(opt?: "collect")
---@overload fun(opt: "stop")
---@overload fun(opt: "restart")
---@overload fun(opt: "count"): number
---@overload fun(opt: "step", arg: integer): true
---@overload fun(opt: "setpause", arg: integer): integer
---@overload fun(opt: "setstepmul", arg: integer): integer
---@overload fun(opt: "isrunning"): boolean
---@overload fun(opt: "generational")
---@overload fun(opt: "incremental")
function collectgarbage(...) end

---@overload fun(opt?: "collect")
---@overload fun(opt: "stop")
---@overload fun(opt: "restart")
---@overload fun(opt: "count"): number
---@overload fun(opt: "step", arg: integer): true
---@overload fun(opt: "setpause", arg: integer): integer
---@overload fun(opt: "setstepmul", arg: integer): integer
---@overload fun(opt: "isrunning"): boolean
function collectgarbage(...) end

---@overload fun(opt?: "collect")
---@overload fun(opt: "stop")
---@overload fun(opt: "restart")
---@overload fun(opt: "count"): number
---@overload fun(opt: "step", arg: integer): true
---@overload fun(opt: "isrunning"): boolean
---@overload fun(opt: "incremental", pause?: integer, multiplier?: integer, stepsize?: integer)
---@overload fun(opt: "generational", minor?: integer, major?: integer)
function collectgarbage(...) end

---@overload fun(opt?: "collect")
---@overload fun(opt: "stop")
---@overload fun(opt: "restart")
---@overload fun(opt: "count"): number
---@overload fun(opt: "step", arg: integer): true
---@overload fun(opt: "isrunning"): boolean
---@overload fun(opt: "incremental"): "generational" | "incremental"
---@overload fun(opt: "generational"): "generational" | "incremental"
---@overload fun(opt: "param", arg: "minormul", value?: integer): integer
---@overload fun(opt: "param", arg: "majorminor", value?: integer): integer
---@overload fun(opt: "param", arg: "minormajor", value?: integer): integer
---@overload fun(opt: "param", arg: "pause", value?: integer): integer
---@overload fun(opt: "param", arg: "stepmul", value?: integer): integer
---@overload fun(opt: "param", arg: "stepsize", value?: integer): integer
function collectgarbage(...) end

---@param filename? string
---@return any ...
function dofile(filename) end

---@param message any
---@param level?  integer
function error(message, level) end

---@class _G
_G = {}

---@version 5.1
---@param f? integer|async fun(...):...
---@return table
---@nodiscard
function getfenv(f) end

---@param object any
---@return table metatable
---@nodiscard
function getmetatable(object) end

---@generic T: table, V
---@param t T
---@return fun(table: V[], i?: integer):integer, V
---@return T
---@return integer i
function ipairs(t) end

---@alias loadmode
---| "b"  # ---| "t"  # ---|>"bt" #
---@param func       function
---@param chunkname? string
---@return function?
---@return string?   error_message
---@nodiscard
function load(func, chunkname) end
---@param chunk      string|function
---@param chunkname? string
---@param mode?      loadmode
---@param env?       table
---@return function?
---@return string?   error_message
---@nodiscard
function load(chunk, chunkname, mode, env) end

---@param filename? string
---@return function?
---@return string?  error_message
---@nodiscard
function loadfile(filename) end
---@param filename? string
---@param mode?     loadmode
---@param env?      table
---@return function?
---@return string?  error_message
---@nodiscard
function loadfile(filename, mode, env) end

---@version 5.1
---@param text       string
---@param chunkname? string
---@return function?
---@return string?   error_message
---@nodiscard
function loadstring(text, chunkname) end

---@version 5.1
---@param proxy boolean|table|userdata
---@return userdata
---@nodiscard
function newproxy(proxy) end

---@version 5.1
---@param name string
---@param ...  any
function module(name, ...) end

---@generic K, V
---@param table table<K, V>
---@param index? K
---@return K?
---@return V?
---@nodiscard
function next(table, index) end

---@generic T: table, K, V
---@param t T
---@return fun(table: table<K, V>, index?: K):K, V
---@return T
function pairs(t) end

---@param f     function
---@param f     async fun(...):...
---@param arg1? any
---@param ...   any
---@return boolean success
---@return any result
---@return any ...
function pcall(f, arg1, ...) end

---@param ... any
function print(...) end

---@param v1 any
---@param v2 any
---@return boolean
---@nodiscard
function rawequal(v1, v2) end

---@param table table
---@param index any
---@return any
---@nodiscard
function rawget(table, index) end

---@param v table|string
---@return integer len
---@nodiscard
function rawlen(v) end

---@param table table
---@param index any
---@param value any
---@return table
function rawset(table, index, value) end

---@param index integer|"#"
---@param ...   any
---@return any
---@nodiscard
function select(index, ...) end

---@version 5.1
---@param f     (async fun(...):...)|integer
---@param table table
---@return function
function setfenv(f, table) end


---@class metatable
---@field __mode 'v'|'k'|'kv'|nil
---@field __metatable any|nil
---@field __tostring (fun(t):string)|nil
---@field __gc fun(t)|nil
---@field __add (fun(t1,t2):any)|nil
---@field __sub (fun(t1,t2):any)|nil
---@field __mul (fun(t1,t2):any)|nil
---@field __div (fun(t1,t2):any)|nil
---@field __mod (fun(t1,t2):any)|nil
---@field __pow (fun(t1,t2):any)|nil
---@field __unm (fun(t):any)|nil
---@field __idiv (fun(t1,t2):any)|nil
---@field __band (fun(t1,t2):any)|nil
---@field __bor (fun(t1,t2):any)|nil
---@field __bxor (fun(t1,t2):any)|nil
---@field __bnot (fun(t):any)|nil
---@field __shl (fun(t1,t2):any)|nil
---@field __shr (fun(t1,t2):any)|nil
---@field __concat (fun(t1,t2):any)|nil
---@field __len (fun(t):integer)|nil
---@field __eq (fun(t1,t2):boolean)|nil
---@field __lt (fun(t1,t2):boolean)|nil
---@field __le (fun(t1,t2):boolean)|nil
---@field __index table|(fun(t,k):any)|nil
---@field __newindex table|fun(t,k,v)|nil
---@field __call (fun(t,...):...)|nil
---@field __pairs (fun(t):((fun(t,k,v):any,any),any,any))|nil
---@field __ipairs (fun(t):(fun(t,k,v):(integer|nil),any))|nil
---@field __close (fun(t,errobj):any)|nil

---@param table      table
---@param metatable? metatable|table
---@return table
function setmetatable(table, metatable) end

---@overload fun(e: string, base: integer):integer
---@param e any
---@return number?
---@nodiscard
function tonumber(e) end

---@param v any
---@return string
---@nodiscard
function tostring(v) end

---@alias type
---| "nil"
---| "number"
---| "string"
---| "boolean"
---| "table"
---| "function"
---| "thread"
---| "userdata"
---| "cdata"

---@param v any
---@return type type
---@nodiscard
function type(v) end

_VERSION = "Lua 5.1"
_VERSION = "Lua 5.2"
_VERSION = "Lua 5.3"
_VERSION = "Lua 5.4"
_VERSION = "Lua 5.5"

---@version >5.4
---@param message string
---@param ...     any
function warn(message, ...) end

---@param f     function
---@param err   function
---@return boolean success
---@return any result
---@return any ...
function xpcall(f, err) end
---@param f     async fun(...):...
---@param msgh  function
---@param arg1? any
---@param ...   any
---@return boolean success
---@return any result
---@return any ...
function xpcall(f, msgh, arg1, ...) end

---@version 5.1
---@generic T1, T2, T3, T4, T5, T6, T7, T8, T9, T10
---@param list {
--- [1]?: T1,
--- [2]?: T2,
--- [3]?: T3,
--- [4]?: T4,
--- [5]?: T5,
--- [6]?: T6,
--- [7]?: T7,
--- [8]?: T8,
--- [9]?: T9,
--- [10]?: T10,
---}
---@param i?   integer
---@param j?   integer
---@return T1, T2, T3, T4, T5, T6, T7, T8, T9, T10
---@nodiscard
function unpack(list, i, j) end

---@version 5.1
---@generic T1, T2, T3, T4, T5, T6, T7, T8, T9
---@param list {[1]: T1, [2]: T2, [3]: T3, [4]: T4, [5]: T5, [6]: T6, [7]: T7, [8]: T8, [9]: T9 }
---@return T1, T2, T3, T4, T5, T6, T7, T8, T9
---@nodiscard
function unpack(list) end
