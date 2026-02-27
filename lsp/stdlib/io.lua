---@meta io

---@class iolib
---@field stdin  file*
---@field stdout file*
---@field stderr file*
io = {}

---@alias openmode
---|>"r"   # ---| "w"   # ---| "a"   # ---| "r+"  # ---| "w+"  # ---| "a+"  # ---| "rb"  # ---| "wb"  # ---| "ab"  # ---| "r+b" # ---| "w+b" # ---| "a+b" #
---@param file? file*
---@return boolean?  suc
---@return exitcode? exitcode
---@return integer?  code
function io.close(file) end

function io.flush() end

---@overload fun():file*
---@param file string|file*
function io.input(file) end

---@param filename string?
---@param ... readmode
---@return fun():any, ...
function io.lines(filename, ...) end

---@param filename string
---@param mode?    openmode
---@return file*?
---@return string? errmsg
---@nodiscard
function io.open(filename, mode) end

---@overload fun():file*
---@param file string|file*
function io.output(file) end

---@alias popenmode
---| "r" # ---| "w" #
---@param prog  string
---@param mode? popenmode
---@return file*?
---@return string? errmsg
function io.popen(prog, mode) end

---@param ... readmode
---@return any
---@return any ...
---@nodiscard
function io.read(...) end

---@return file*
---@nodiscard
function io.tmpfile() end

---@alias filetype
---| "file"        # ---| "closed file" # ---| `nil`         #
---@param file file*
---@return filetype
---@nodiscard
function io.type(file) end

---@return file*
---@return string? errmsg
function io.write(...) end

---@class file*
local file = {}

---@alias readmode integer|string
---| "n"  # ---| "a"  # ---|>"l"  # ---| "L"  # ---| "*n" # ---| "*a" # ---|>"*l" # ---| "*L" #
---@alias exitcode "exit"|"signal"

---@return boolean?  suc
---@return exitcode? exitcode
---@return integer?  code
function file:close() end

function file:flush() end

---@param ... readmode
---@return fun():any, ...
function file:lines(...) end

---@param ... readmode
---@return any
---@return any ...
---@nodiscard
function file:read(...) end

---@alias seekwhence
---| "set" # ---|>"cur" # ---| "end" #
---@param whence? seekwhence
---@param offset? integer
---@return integer offset
---@return string? errmsg
function file:seek(whence, offset) end

---@alias vbuf
---| "no"   # ---| "full" # ---| "line" #
---@param mode vbuf
---@param size? integer
function file:setvbuf(mode, size) end

---@param ... string|number
---@return file*?
---@return string? errmsg
function file:write(...) end

return io
