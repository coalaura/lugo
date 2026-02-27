---@meta utf8

---@version >5.3
---@class utf8lib
---@field charpattern string
utf8 = {}

---@param code integer
---@param ... integer
---@return string
---@nodiscard
function utf8.char(code, ...) end

---@param s    string
---@return fun(s: string, p: integer):integer, integer
function utf8.codes(s) end
---@param s    string
---@param lax? boolean
---@return fun(s: string, p: integer):integer, integer
function utf8.codes(s, lax) end

---@param s    string
---@param i?   integer
---@param j?   integer
---@return integer code
---@return integer ...
---@nodiscard
function utf8.codepoint(s, i, j) end
---@param s    string
---@param i?   integer
---@param j?   integer
---@param lax? boolean
---@return integer code
---@return integer ...
---@nodiscard
function utf8.codepoint(s, i, j, lax) end

---@param s    string
---@param i?   integer
---@param j?   integer
---@return integer?
---@return integer? errpos
---@nodiscard
function utf8.len(s, i, j) end
---@param s    string
---@param i?   integer
---@param j?   integer
---@param lax? boolean
---@return integer?
---@return integer? errpos
---@nodiscard
function utf8.len(s, i, j, lax) end

---@param s string
---@param n integer
---@param i? integer
---@return integer p
---@nodiscard
function utf8.offset(s, n, i) end

---@param s string
---@param n integer
---@param i? integer
---@return integer ps
---@return integer pe
---@nodiscard
function utf8.offset(s, n, i) end

return utf8
