---@meta os

---@class oslib
os = {}

---@return number
---@nodiscard
function os.clock() end

---@class osdate:osdateparam
---@field year  integer|string
---@field month integer|string
---@field day   integer|string
---@field hour  integer|string
---@field min   integer|string
---@field sec   integer|string
---@field wday  integer|string
---@field yday  integer|string
---@field isdst boolean

---@param format? string
---@param time?   integer
---@return string|osdate
---@nodiscard
function os.date(format, time) end

---@param t2 integer
---@param t1 integer
---@return integer
---@nodiscard
function os.difftime(t2, t1) end

---@param command? string
---@return integer code
function os.execute(command) end
---@param command? string
---@return boolean?  suc
---@return exitcode? exitcode
---@return integer?  code
function os.execute(command) end

---@param code? integer
function os.exit(code, close) end
---@param code?  boolean|integer
---@param close? boolean
function os.exit(code, close) end

---@param varname string
---@return string?
---@nodiscard
function os.getenv(varname) end

---@param filename string
---@return boolean suc
---@return string? errmsg
function os.remove(filename) end

---@param oldname string
---@param newname string
---@return boolean suc
---@return string? errmsg
function os.rename(oldname, newname) end

---@alias localecategory
---|>"all"
---| "collate"
---| "ctype"
---| "monetary"
---| "numeric"
---| "time"

---@param locale    string|nil
---@param category? localecategory
---@return string localecategory
function os.setlocale(locale, category) end

---@class osdateparam
---@field year  integer|string
---@field month integer|string
---@field day   integer|string
---@field hour  (integer|string)?
---@field min   (integer|string)?
---@field sec   (integer|string)?
---@field wday  (integer|string)?
---@field yday  (integer|string)?
---@field isdst boolean?

---@param date? osdateparam
---@return integer
---@nodiscard
function os.time(date) end

---@return string
---@nodiscard
function os.tmpname() end

return os
