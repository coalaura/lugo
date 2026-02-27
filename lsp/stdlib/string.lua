---@meta string

---@class stringlib
string = {}

---@param s  string|number
---@param i? integer
---@param j? integer
---@return integer ...
---@nodiscard
function string.byte(s, i, j) end

---@param byte integer
---@param ... integer
---@return string
---@nodiscard
function string.char(byte, ...) end

---@param f      async fun(...):...
---@param strip? boolean
---@return string
---@nodiscard
function string.dump(f, strip) end
---@param f      async fun(...):...
---@return string
---@nodiscard
function string.dump(f) end

---@param s       string|number
---@param pattern string|number
---@param init?   integer
---@param plain?  boolean
---@return integer|nil start
---@return integer|nil end
---@return any|nil ... captured
---@nodiscard
function string.find(s, pattern, init, plain) end

---@param s string|number
---@param ... any
---@return string
---@nodiscard
function string.format(s, ...) end

---@param s       string|number
---@param pattern string|number
---@return fun():string, ...
---@nodiscard
function string.gmatch(s, pattern) end
---@param s       string|number
---@param pattern string|number
---@param init?   integer
---@return fun():string, ...
function string.gmatch(s, pattern, init) end

---@param s       string|number
---@param pattern string|number
---@param repl    string|number|table|function
---@param n?      integer
---@return string
---@return integer count
function string.gsub(s, pattern, repl, n) end

---@param s string|number
---@return integer
---@nodiscard
function string.len(s) end

---@param s string|number
---@return string
---@nodiscard
function string.lower(s) end

---@param s       string|number
---@param pattern string|number
---@param init?   integer
---@return any ...
---@nodiscard
function string.match(s, pattern, init) end

---@version >5.3
---@param fmt string
---@param v1  string|number
---@param v2? string|number
---@param ... string|number
---@return string binary
---@nodiscard
function string.pack(fmt, v1, v2, ...) end

---@version >5.3
---@param fmt string
---@return integer
---@nodiscard
function string.packsize(fmt) end

---@param s    string|number
---@param n    integer
---@return string
---@nodiscard
function string.rep(s, n) end
---@param s    string|number
---@param n    integer
---@param sep? string|number
---@return string
---@nodiscard
function string.rep(s, n, sep) end

---@param s string|number
---@return string
---@nodiscard
function string.reverse(s) end

---@param s  string|number
---@param i  integer
---@param j? integer
---@return string
---@nodiscard
function string.sub(s, i, j) end

---@version >5.3
---@param fmt  string
---@param s    string
---@param pos? integer
---@return any ...
---@nodiscard
function string.unpack(fmt, s, pos) end

---@param s string|number
---@return string
---@nodiscard
function string.upper(s) end

return string
