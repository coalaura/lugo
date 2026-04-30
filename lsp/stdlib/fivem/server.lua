---@meta

---Server-only FiveM runtime metadata bundle.

---Server-side scheduler helper for HTTP requests.
---@param url string
---@param callback fun(statusCode: integer, body: string, headers: table<string, string>, errorData?: string)
---@param method? string
---@param data? string
---@param headers? table<string, string>
---@param options? table
function PerformHttpRequest(url, callback, method, data, headers, options) end

---Promise-style HTTP request helper that awaits completion inside a scheduler coroutine.
---@param url string
---@param method? string
---@param data? string
---@param headers? table<string, string>
---@param options? table
---@return integer, string, table<string, string>, string?
function PerformHttpRequestAwait(url, method, data, headers, options) end

---Returns all active player identifiers.
---@return integer[]
function GetPlayers() end

---Returns identifier strings for a player.
---@param playerId integer|string
---@return string[]
function GetPlayerIdentifiers(playerId) end

---Returns token strings for a player.
---@param playerId integer|string
---@return string[]
function GetPlayerTokens(playerId) end

---Custom FXServer `os` library.
---This is not stock Lua `os`; it exposes filesystem and timing helpers implemented by the host.
---@class os
os = os

---@param command? string
---@return boolean|integer|string|nil
function os.execute(command) end

---@param path string
---@return boolean|integer|string|nil
function os.createdir(path) end

---@param path string
---@return boolean|integer|string|nil
function os.remove(path) end

---@param from string
---@param to string
---@return boolean|integer|string|nil
function os.rename(from, to) end

---@return string
function os.tmpname() end

---@param key string
---@return string|nil
function os.getenv(key) end

---@param locale string
---@param category? string
---@return string|nil
function os.setlocale(locale, category) end

---@return number
function os.deltatime() end

---@return integer
function os.microtime() end

---@return integer
function os.nanotime() end

---@return integer
function os.rdtsc() end

---@return integer
function os.rdtscp() end

---Custom FXServer `io` library backed by host VFS and pipe helpers.
---@class io
io = io

---@param path string
---@param mode? string
---@return file*|nil
function io.open(path, mode) end

---@param command string
---@param mode? string
---@return userdata|nil
function io.popen(command, mode) end

---@param path string
---@return userdata|nil
function io.readdir(path) end

---@return file*
function io.tmpfile() end

---@param ... any
function io.write(...) end

---Dynamic event source for the current handler dispatch.
---Network event sources are normalised to numeric peer or player identifiers before handler code runs.
---@type integer
source = 0

---Triggers a client event for one player or a broadcast target using msgpack-backed bridge delivery.
---@param eventName string
---@param playerId integer
---@param ... any
function TriggerClientEvent(eventName, playerId, ...) end

---Triggers a latent client event with an explicit bytes-per-second throttle.
---@param eventName string
---@param playerId integer
---@param bps number|string
---@param ... any
function TriggerLatentClientEvent(eventName, playerId, bps, ...) end

---Server-side alias of `RegisterNetEvent`.
---@param eventName string
---@param callback? fun(...: any)
function RegisterServerEvent(eventName, callback) end

---Server-side alias of `Citizen.Trace` for RCON output.
---@param message string
function RconPrint(message) end
