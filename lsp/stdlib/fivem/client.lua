---@meta

---Client-only FiveM runtime metadata bundle.

---Invokes a native through the client-side secondary marshaling entry point.
---This follows the same manifest-selected GTA/RDR3/NY client native build as `Citizen.InvokeNative`.
---@param native any
---@param ... any
---@return any
function Citizen.InvokeNative2(native, ...) end

---Returns a direct native callable binding for the given native identifier on client builds.
---This is the client-only direct binding surface used by OAL-capable resources and native helper loaders.
---@param native any
---@return function|nil
function Citizen.GetNative(native) end

---Triggers a server event using msgpack-backed bridge delivery.
---@param eventName string
---@param ... any
function TriggerServerEvent(eventName, ...) end

---Triggers a latent server event using msgpack-backed bridge delivery with an explicit bytes-per-second throttle.
---@param eventName string
---@param bps number|string
---@param ... any
function TriggerLatentServerEvent(eventName, bps, ...) end

---Registers a direct NUI callback bridge for a browser callback type.
---@param callbackType string
---@param callback fun(data: any, reply: funcref(response?: any): nil)
function RegisterNuiCallback(callbackType, callback) end

---Registers the legacy event-backed NUI callback bridge for a browser callback type.
---@param callbackType string
---@param callback fun(data: any, reply: funcref(response?: any): nil)
function RegisterNUICallback(callbackType, callback) end

---Registers a NUI callback type so the legacy `__cfx_nui:<type>` bridge can receive events.
---@param callbackType string
function RegisterNuiCallbackType(callbackType) end

---Sends a JSON-encoded message payload to the active NUI frame.
---@param message any
---@return boolean
function SendNUIMessage(message) end

---Sets NUI keyboard focus and cursor focus state.
---@param hasFocus boolean
---@param hasCursor boolean
function SetNuiFocus(hasFocus, hasCursor) end

---Controls whether NUI focus keeps normal game input enabled.
---@param keepInput boolean
function SetNuiFocusKeepInput(keepInput) end

---Returns whether the NUI layer currently has focus.
---@return boolean
function IsNuiFocused() end

---Returns whether the focused NUI layer keeps normal game input enabled.
---@return boolean
function IsNuiFocusKeepingInput() end
