---@meta
io = {}

---@type file*
io.stderr = nil

---@type file*
io.stdin = nil

---@type file*
io.stdout = nil

---Equivalent to file:close(). Without a file, closes the default output file.
---@param file? file*
---@return boolean|nil, string?, integer?
function io.close(file) end

---Equivalent to io.output():flush().
function io.flush() end

---When called with a file name, it opens the named file (in text mode), and sets its handle as the default input file.
---@param file? string|file*
---@return file*
function io.input(file) end

---Opens the given file name in read mode and returns an iterator function that works like file:lines(...).
---@param filename? string
---@param ... string|integer
---@return function
function io.lines(filename, ...) end

---This function opens a file, in the mode specified in the string mode.
---@param filename string
---@param mode? "r"|"w"|"a"|"r+"|"w+"|"a+"|"rb"|"wb"|"ab"|"r+b"|"w+b"|"a+b"
---@return file*|nil, string?, integer?
function io.open(filename, mode) end

---Similar to io.input, but operates over the default output file.
---@param file? string|file*
---@return file*
function io.output(file) end

---Starts program prog in a separated process and returns a file handle that you can use to read data from this program (if mode is "r") or to write data to this program (if mode is "w").
---@param prog string
---@param mode? "r"|"w"
---@return file*|nil, string?, integer?
function io.popen(prog, mode) end

---Equivalent to io.input():read(...).
---@param ... string|integer
---@return any
function io.read(...) end

---In case of success, returns a handle for a temporary file.
---@return file*
function io.tmpfile() end

---Checks whether obj is a valid file handle.
---@param obj any
---@return "file"|"closed file"|nil
function io.type(obj) end

---Equivalent to io.output():write(...).
---@param ... string|number
---@return file*|nil, string?
function io.write(...) end
