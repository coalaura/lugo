---@meta

---Opens the named file and executes its contents as a Lua chunk.
---@param filename? string
---@return any
function dofile(filename) end

---Similar to load, but gets the chunk from file filename.
---@param filename? string
---@param mode? "b"|"t"|"bt"
---@param env? table
---@return function|nil, string?
function loadfile(filename, mode, env) end
