---@meta
math = {}

---The float value HUGE_VAL, a value larger than any other numeric value.
---@type number
math.huge = math.huge -- Evaluates to inf

---An integer with the maximum value for an integer.
---@type integer
math.maxinteger = 9223372036854775807

---An integer with the minimum value for an integer.
---@type integer
math.mininteger = -9223372036854775808

---The value of pi.
---@type number
math.pi = 3.1415926535898

---Returns the absolute value of x.
---@param x number
---@return number
function math.abs(x) end

---Returns the arc cosine of x (in radians).
---@param x number
---@return number
function math.acos(x) end

---Returns the arc sine of x (in radians).
---@param x number
---@return number
function math.asin(x) end

---Returns the arc tangent of x (in radians).
---@param x number
---@return number
function math.atan(x) end

---Returns the arc tangent of y/x (in radians), using the signs of both parameters to find the quadrant.
---@param y number
---@param x number
---@return number
function math.atan2(y, x) end

---Returns the smallest integral value larger than or equal to x.
---@param x number
---@return integer
function math.ceil(x) end

---Returns the cosine of x (assumed to be in radians).
---@param x number
---@return number
function math.cos(x) end

---Returns the hyperbolic cosine of x.
---@param x number
---@return number
function math.cosh(x) end

---Converts the angle x from radians to degrees.
---@param x number
---@return number
function math.deg(x) end

---Returns the value e^x (where e is the base of natural logarithms).
---@param x number
---@return number
function math.exp(x) end

---Returns the largest integral value smaller than or equal to x.
---@param x number
---@return integer
function math.floor(x) end

---Returns the remainder of the division of x by y that rounds the quotient towards zero.
---@param x number
---@param y number
---@return number
function math.fmod(x, y) end

---Decomposes x into tails and exponent. Returns m and e such that x = m * (2 ^ e).
---@param x number
---@return number, integer
function math.frexp(x) end

---Returns m * (2 ^ e).
---@param m number
---@param e integer
---@return number
function math.ldexp(m, e) end

---Returns the logarithm of x in the given base (default is natural logarithm, e).
---@param x number
---@param base? number
---@return number
function math.log(x, base) end

---Returns the base-10 logarithm of x.
---@param x number
---@return number
function math.log10(x) end

---Returns the argument with the maximum value.
---@param x number
---@param ... number
---@return number
function math.max(x, ...) end

---Returns the argument with the minimum value.
---@param x number
---@param ... number
---@return number
function math.min(x, ...) end

---Returns the integral part of x and the fractional part of x.
---@param x number
---@return integer, number
function math.modf(x) end

---Returns x ^ y.
---@param x number
---@param y number
---@return number
function math.pow(x, y) end

---Converts the angle x from degrees to radians.
---@param x number
---@return number
function math.rad(x) end

---When called without arguments, returns a pseudo-random float with uniform distribution in the range [0,1). When called with two integers m and n, returns a pseudo-random integer with uniform distribution in the range [m, n].
---@param m? integer
---@param n? integer
---@return number|integer
function math.random(m, n) end

---Sets x as the "seed" for the pseudo-random generator.
---@param x? integer
---@param y? integer
function math.randomseed(x, y) end

---Returns the sine of x (assumed to be in radians).
---@param x number
---@return number
function math.sin(x) end

---Returns the hyperbolic sine of x.
---@param x number
---@return number
function math.sinh(x) end

---Returns the square root of x.
---@param x number
---@return number
function math.sqrt(x) end

---Returns the tangent of x (assumed to be in radians).
---@param x number
---@return number
function math.tan(x) end

---Returns the hyperbolic tangent of x.
---@param x number
---@return number
function math.tanh(x) end

---If the value x is convertible to an integer, returns that integer. Otherwise, returns nil.
---@param x any
---@return integer|nil
function math.tointeger(x) end

---Returns "integer" if x is an integer, "float" if it is a float, or nil if x is not a number.
---@param x any
---@return "integer"|"float"|nil
function math.type(x) end

---Returns a boolean, true if and only if integer m is below integer n when they are compared as unsigned integers.
---@param m integer
---@param n integer
---@return boolean
function math.ult(m, n) end
