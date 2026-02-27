---@meta math

---@class mathlib
---@field huge       number
---@field maxinteger integer
---@field mininteger integer
---@field pi         number
math = {}

---@generic Number: number
---@param x Number
---@return Number
---@nodiscard
function math.abs(x) end

---@param x number
---@return number
---@nodiscard
function math.acos(x) end

---@param x number
---@return number
---@nodiscard
function math.asin(x) end

---@param y number
---@return number
---@nodiscard
function math.atan(y) end
---@param y  number
---@param x? number
---@return number
---@nodiscard
function math.atan(y, x) end

---@version <5.2
---@param y number
---@param x number
---@return number
---@nodiscard
function math.atan2(y, x) end

---@param x number
---@return integer
---@nodiscard
function math.ceil(x) end

---@param x number
---@return number
---@nodiscard
function math.cos(x) end

---@version <5.2
---@param x number
---@return number
---@nodiscard
function math.cosh(x) end

---@param x number
---@return number
---@nodiscard
function math.deg(x) end

---@param x number
---@return number
---@nodiscard
function math.exp(x) end

---@param x number
---@return integer
---@nodiscard
function math.floor(x) end

---@param x number
---@param y number
---@return number
---@nodiscard
function math.fmod(x, y) end

---@version <5.2, >5.5
---@param x number
---@return number m
---@return number e
---@nodiscard
function math.frexp(x) end

---@version <5.2, >5.5
---@param m number
---@param e number
---@return number
---@nodiscard
function math.ldexp(m, e) end

---@param x     number
---@return number
---@nodiscard
function math.log(x) end
---@param x     number
---@param base? integer
---@return number
---@nodiscard
function math.log(x, base) end

---@version <5.1
---@param x number
---@return number
---@nodiscard
function math.log10(x) end

---@generic Number: number
---@param x Number
---@param ... Number
---@return Number
---@nodiscard
function math.max(x, ...) end

---@generic Number: number
---@param x Number
---@param ... Number
---@return Number
---@nodiscard
function math.min(x, ...) end

---@param x number
---@return integer
---@return number
---@nodiscard
function math.modf(x) end

---@version <5.2
---@param x number
---@param y number
---@return number
---@nodiscard
function math.pow(x, y) end

---@param x number
---@return number
---@nodiscard
function math.rad(x) end

---@overload fun():number
---@overload fun(m: integer):integer
---@param m integer
---@param n integer
---@return integer
---@nodiscard
function math.random(m, n) end

---@param x? integer
---@param y? integer
function math.randomseed(x, y) end
---@param x integer
function math.randomseed(x) end

---@param x number
---@return number
---@nodiscard
function math.sin(x) end

---@version <5.2
---@param x number
---@return number
---@nodiscard
function math.sinh(x) end

---@param x number
---@return number
---@nodiscard
function math.sqrt(x) end

---@param x number
---@return number
---@nodiscard
function math.tan(x) end

---@version <5.2
---@param x number
---@return number
---@nodiscard
function math.tanh(x) end

---@version >5.3
---@param x any
---@return integer?
---@nodiscard
function math.tointeger(x) end

---@version >5.3
---@param x any
---@return
---| '"integer"'
---| '"float"'
---| 'nil'
---@nodiscard
function math.type(x) end

---@version >5.3
---@param m integer
---@param n integer
---@return boolean
---@nodiscard
function math.ult(m, n) end

return math
