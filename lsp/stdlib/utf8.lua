---@meta
utf8 = {}

---The pattern (a string) which matches exactly one UTF-8 byte sequence, assuming that the subject is a valid UTF-8 string.
---@type string
utf8.charpattern = "[-┬-²][Ç-┐]*"

---Receives zero or more integers, converts each one to its corresponding UTF-8 byte sequence and returns a string with the concatenation of all these sequences.
---@param ... integer
---@return string
function utf8.char(...) end

---Returns an iterator function so that `for p, c in utf8.codes(s) do body end` will iterate over all characters in string s, with p being the position (in bytes) and c the code point of each character.
---@param s string
---@param lax? boolean
---@return function, string, integer
function utf8.codes(s, lax) end

---Returns the codepoints (as integers) from all characters in s that start between byte position i and j (both included).
---@param s string
---@param i? integer
---@param j? integer
---@param lax? boolean
---@return integer ...
function utf8.codepoint(s, i, j, lax) end

---Returns the number of UTF-8 characters in string s that start between positions i and j (both inclusive). If it finds any invalid byte sequence, returns false plus the position of the first invalid byte.
---@param s string
---@param i? integer
---@param j? integer
---@param lax? boolean
---@return integer|false, integer?
function utf8.len(s, i, j, lax) end

---Returns the position (in bytes) where the encoding of the n-th character of s (counting from position i) starts.
---@param s string
---@param n integer
---@param i? integer
---@return integer|nil
function utf8.offset(s, n, i) end
