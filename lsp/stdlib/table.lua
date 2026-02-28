---@meta
table = {}

---Given a list where all elements are strings or numbers, returns the string list[i]..sep..list[i+1] ··· sep..list[j].
---@param list table
---@param sep? string
---@param i? integer
---@param j? integer
---@return string
function table.concat(list, sep, i, j) end

---Inserts element value at position pos in list, shifting up the elements list[pos], list[pos+1], ···, list[#list].
---@param list table
---@param pos? integer
---@param value any
function table.insert(list, pos, value) end

---Moves elements from table a1 to table a2.
---@param a1 table
---@param f integer
---@param e integer
---@param t integer
---@param a2? table
---@return table
function table.move(a1, f, e, t, a2) end

---Returns a new table with all arguments stored into keys 1, 2, etc. and with a field "n" with the total number of arguments.
---@param ... any
---@return table
function table.pack(...) end

---Removes from list the element at position pos, returning the value of the removed element.
---@param list table
---@param pos? integer
---@return any
function table.remove(list, pos) end

---Sorts the list elements in a given order, in-place, from list[1] to list[#list].
---@param list table
---@param comp? function
function table.sort(list, comp) end

---Returns the elements from the given list.
---@param list table
---@param i? integer
---@param j? integer
---@return any ...
function table.unpack(list, i, j) end
