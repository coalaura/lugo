---@meta
string = {}

---Returns the internal numeric codes of the characters s[i], s[i+1], ..., s[j].
---@param s string
---@param i? integer
---@param j? integer
---@return integer ...
function string.byte(s, i, j) end

---Receives zero or more integers. Returns a string with length equal to the number of arguments, in which each character has the internal numeric code equal to its corresponding argument.
---@param ... integer
---@return string
function string.char(...) end

---Returns a string containing a binary representation (a binary chunk) of the given function.
---@param func function
---@param strip? boolean
---@return string
function string.dump(func, strip) end

---Looks for the first match of pattern in the string s.
---@param s string
---@param pattern string
---@param init? integer
---@param plain? boolean
---@return integer|nil, integer?, any ...
function string.find(s, pattern, init, plain) end

---Returns a formatted version of its variable number of arguments following the description given in its first argument (which must be a string).
---@param formatstring string
---@param ... any
---@return string
function string.format(formatstring, ...) end

---Returns an iterator function that, each time it is called, returns the next captures from pattern over the string s.
---@param s string
---@param pattern string
---@param init? integer
---@return function
function string.gmatch(s, pattern, init) end

---Returns a copy of s in which all (or the first n, if given) occurrences of the pattern have been replaced by a replacement string specified by repl.
---@param s string
---@param pattern string
---@param repl string|table|function
---@param n? integer
---@return string, integer
function string.gsub(s, pattern, repl, n) end

---Receives a string and returns its length.
---@param s string
---@return integer
function string.len(s) end

---Receives a string and returns a copy of this string with all uppercase letters changed to lowercase.
---@param s string
---@return string
function string.lower(s) end

---Looks for the first match of pattern in the string s. If it finds one, then match returns the captures from the pattern; otherwise it returns nil.
---@param s string
---@param pattern string
---@param init? integer
---@return string|nil ...
function string.match(s, pattern, init) end

---Returns a binary string containing the values v1, v2, etc. serialized in binary form (packed) according to the format string fmt.
---@param fmt string
---@param ... any
---@return string
function string.pack(fmt, ...) end

---Returns the size of a string resulting from string.pack with the given format.
---@param fmt string
---@return integer
function string.packsize(fmt) end

---Returns a string that is the concatenation of n copies of the string s separated by the string sep.
---@param s string
---@param n integer
---@param sep? string
---@return string
function string.rep(s, n, sep) end

---Returns a string that is the string s reversed.
---@param s string
---@return string
function string.reverse(s) end

---Returns the substring of s that starts at i and continues until j.
---@param s string
---@param i integer
---@param j? integer
---@return string
function string.sub(s, i, j) end

---Returns the values packed in string s according to the format string fmt.
---@param fmt string
---@param s string
---@param pos? integer
---@return any ..., integer
function string.unpack(fmt, s, pos) end

---Receives a string and returns a copy of this string with all lowercase letters changed to uppercase.
---@param s string
---@return string
function string.upper(s) end
