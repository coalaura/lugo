---@meta
os = {}

---Returns an approximation of the amount in seconds of CPU time used by the program.
---@return number
function os.clock() end

---Returns a string or a table containing date and time, formatted according to the given string format.
---@param format? string
---@param time? integer
---@return string|table
function os.date(format, time) end

---Returns the difference, in seconds, from time t1 to time t2.
---@param t2 integer
---@param t1 integer
---@return number
function os.difftime(t2, t1) end

---Passes command to be executed by an operating system shell.
---@param command? string
---@return boolean|nil, string?, integer?
function os.execute(command) end

---Calls the ISO C function exit to terminate the host program.
---@param code? boolean|integer
---@param close? boolean
function os.exit(code, close) end

---Returns the value of the process environment variable varname.
---@param varname string
---@return string|nil
function os.getenv(varname) end

---Deletes the file (or empty directory, on POSIX systems) with the given name.
---@param filename string
---@return boolean|nil, string?, integer?
function os.remove(filename) end

---Renames the file or directory named oldname to newname.
---@param oldname string
---@param newname string
---@return boolean|nil, string?, integer?
function os.rename(oldname, newname) end

---Sets the current locale of the program.
---@param locale string
---@param category? "all"|"collate"|"ctype"|"monetary"|"numeric"|"time"
---@return string|nil
function os.setlocale(locale, category) end

---Returns the current time when called without arguments, or a time representing the local date and time specified by the given table.
---@param table? table
---@return integer
function os.time(table) end

---Returns a string with a file name that can be used for a temporary file.
---@return string
function os.tmpname() end
