---@meta

---Server-only FiveM runtime metadata bundle.

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
