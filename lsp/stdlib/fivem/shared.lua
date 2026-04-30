---@meta

---FiveM's JSON helper library available in runtime and manifest environments.
---This is a host-provided encoder/decoder surface, not stock Lua.
---@class json
json = {}

---Encodes a Lua value to a JSON string.
---@param value any
---@return string
function json.encode(value) end

---Decodes a JSON string into Lua values.
---@param payload string
---@return any
function json.decode(payload) end

---JSON null sentinel used by the host-provided JSON implementation.
---@type userdata|nil
json.null = nil

---FiveM exposes a custom debug library rather than stock Lua's default implementation.
---Metatable accessors remain available across the runtime boundary.
debug = debug

---Returns the metatable for a value using the custom FiveM debug library.
---@param value any
---@return table|nil
function debug.getmetatable(value) end

---Sets the metatable for a value using the custom FiveM debug library.
---@param value any
---@param mt table|nil
---@return any
function debug.setmetatable(value, mt) end

---Produces a traceback string using the runtime-installed debug formatter.
---@param message? string
---@param level? integer
---@return string
function debug.traceback(message, level) end

---Loads another Lua file from the current resource or an explicitly included dependency.
---This overlays stock Lua's package-searcher semantics with FiveM's resource-aware module loading.
---@param modname string
---@return any
function require(modname) end

---Serialises and deserialises msgpack payloads for events, exports, NUI callbacks, state bags, and function-reference transport.
---@class msgpack
msgpack = {}

---Serialises a single value to a msgpack byte string.
---@param value any
---@return string
function msgpack.pack(value) end

---Serialises a variadic argument list for host-bound bridge calls.
---@param ... any
---@return string
function msgpack.pack_args(...) end

---Deserialises a msgpack byte string into Lua values.
---@param payload string
---@return any
function msgpack.unpack(payload) end

---Associates a Lua type name with a msgpack extension tag.
---@param luaType string
---@param extType integer
function msgpack.settype(luaType, extType) end

---Registers extension-pack and extension-unpack handlers for a msgpack extension tag.
---@param extType integer
---@param codec table
function msgpack.extend(extType, codec) end

---Clears registered extension handlers for one or more msgpack extension tags.
---@param ... integer
function msgpack.extend_clear(...) end

---The host-facing Citizen runtime table that exposes scheduler, bridge, boundary, and native-binding primitives.
---@class Citizen
Citizen = {}

---Registers the host boundary routine used by scheduler wrappers.
---@param routine fun(boundary: any, fn: fun(...: any), ...: any): any
function Citizen.SetBoundaryRoutine(routine) end

---Registers the runtime tick routine used by the scheduler.
---@param routine fun(...: any)
function Citizen.SetTickRoutine(routine) end

---Registers the host event dispatch routine.
---@param routine fun(eventName: string, payload: string, eventSource: string|number): nil
function Citizen.SetEventRoutine(routine) end

---Creates a scheduler coroutine and resumes it on the next host tick.
---@param handler fun(...: any)
function Citizen.CreateThread(handler) end

---Creates a scheduler coroutine and resumes it immediately.
---@param handler fun(...: any)
function Citizen.CreateThreadNow(handler) end

---Yields the current scheduler coroutine for the given delay in milliseconds.
---@param milliseconds? integer
function Citizen.Wait(milliseconds) end

---Schedules a callback to run after the given delay in milliseconds.
---@param milliseconds integer
---@param handler fun(...: any)
---@return integer
function Citizen.SetTimeout(milliseconds, handler) end

---Cancels a timeout created by `Citizen.SetTimeout`.
---@param timeoutId integer
function Citizen.ClearTimeout(timeoutId) end

---Invokes a native through the FiveM marshaling bridge.
---The active runtime selects the later native-wrapper catalog from the resource manifest:
---GTA Five resources use `natives_21e43a33.lua`, `natives_0193d0af.lua`, or `natives_universal.lua`,
---RDR3 uses `rdr3_universal.lua`, NY uses `ny_universal.lua`, and server runtimes use `natives_server.lua`.
---@param native any
---@param ... any
---@return any
function Citizen.InvokeNative(native, ...) end

---Loads a native wrapper by name and returns either a direct callable or generated wrapper source.
---When `use_experimental_fxv2_oal` is enabled for a supported client resource, this can take the direct OAL-style callable path instead of returning generated wrapper source.
---@param nativeName string
---@return function|string|nil
function Citizen.LoadNative(nativeName) end

---Configures the host callback used to invoke function references.
---@param routine fun(ref: string, payload: string): string
function Citizen.SetCallRefRoutine(routine) end

---Configures the host callback used to delete function references.
---@param routine fun(ref: string): nil
function Citizen.SetDeleteRefRoutine(routine) end

---Configures the host callback used to duplicate function references.
---@param routine fun(ref: string): string
function Citizen.SetDuplicateRefRoutine(routine) end

---Formats a runtime-local function-reference id into its canonical `resource:instance:ref` string form.
---@param refId integer
---@return string
function Citizen.CanonicalizeRef(refId) end

---Creates a canonical function-reference string for a closure or imported callable proxy.
---@param fn function|table
---@return string
function Citizen.GetFunctionReference(fn) end

---Invokes a function reference with a msgpack-encoded payload and returns a msgpack-encoded result payload.
---@param ref string
---@param payload string
---@return string
function Citizen.InvokeFunctionReference(ref, payload) end

---Marks the beginning of a host boundary for stack and scheduler bookkeeping.
---@param boundaryName string
---@return any
function Citizen.SubmitBoundaryStart(boundaryName) end

---Marks the end of a host boundary and returns a boundary-wrapped callable.
---@param boundary any
---@param fn fun(...: any)
---@return fun(...: any): any
function Citizen.SubmitBoundaryEnd(boundary, fn) end

---Registers the formatter used to capture Lua stack frames for host-side script errors.
---@param routine fun(...: any): string
function Citizen.SetStackTraceRoutine(routine) end

---Writes a formatted trace line through the host script warning channel.
---@param ... any
function Citizen.Trace(...) end

---Awaits a FiveM scheduler promise inside a scheduler coroutine.
---@param promise any
---@return any
function Citizen.Await(promise) end

---Creates a pointer sentinel for an output integer initialised to zero.
---@return userdata
function Citizen.PointerValueIntInitialized() end

---Creates a pointer sentinel for an output float initialised to zero.
---@return userdata
function Citizen.PointerValueFloatInitialized() end

---Creates a pointer sentinel for an output integer.
---@return userdata
function Citizen.PointerValueInt() end

---Creates a pointer sentinel for an output float.
---@return userdata
function Citizen.PointerValueFloat() end

---Creates a pointer sentinel for an output vector.
---@return userdata
function Citizen.PointerValueVector() end

---Requests that the native bridge return the raw host result alongside coerced helpers.
---@return userdata
function Citizen.ReturnResultAnyway() end

---Requests integer result coercion for the active native call.
---@return userdata
function Citizen.ResultAsInteger() end

---Requests 64-bit integer result coercion for the active native call.
---@return userdata
function Citizen.ResultAsLong() end

---Requests floating-point result coercion for the active native call.
---@return userdata
function Citizen.ResultAsFloat() end

---Requests string result coercion for the active native call.
---@return userdata
function Citizen.ResultAsString() end

---Requests vector result coercion for the active native call.
---@return userdata
function Citizen.ResultAsVector() end

---Requests object result coercion for the active native call.
---@return userdata
function Citizen.ResultAsObject() end

---Requests object result coercion using the runtime object-deserialisation callback.
---@param callback? fun(value: any): any
---@return userdata
function Citizen.ResultAsObject2(callback) end

---Special yield sentinel used by `Citizen.Await` and bookmark reattachment.
---@type userdata
Citizen.AwaitSentinel = nil

---Root-level alias of `Citizen.Wait` that yields the current scheduler coroutine for the given delay in milliseconds.
---@param milliseconds? integer
function Wait(milliseconds) end

---Root-level alias of `Citizen.CreateThread`.
---@param handler fun(...: any)
function CreateThread(handler) end

---Root-level alias of `Citizen.SetTimeout`.
---@param milliseconds integer
---@param handler fun(...: any)
---@return integer
function SetTimeout(milliseconds, handler) end

---Root-level alias of `Citizen.ClearTimeout`.
---@param timeoutId integer
function ClearTimeout(timeoutId) end

---Registers an event handler in the runtime-local scheduler registry and returns a removal token.
---@param eventName string
---@param handler fun(...: any)
---@return { key: any, name: string }
function AddEventHandler(eventName, handler) end

---Removes a previously registered event handler token.
---@param token { key: any, name: string }
function RemoveEventHandler(token) end

---Marks an event as safe for network delivery and optionally wires a handler immediately.
---@param eventName string
---@param callback? fun(...: any)
function RegisterNetEvent(eventName, callback) end

---Triggers a local resource event using msgpack-backed host delivery.
---@param eventName string
---@param ... any
function TriggerEvent(eventName, ...) end

---Cancels the active event-dispatch chain in the host event manager.
function CancelEvent() end

---Returns whether the currently active triggered event was cancelled.
---@return boolean
function WasEventCanceled() end

---Replicated state-bag proxy backed by host reads and writes rather than raw Lua table storage.
GlobalState = {}

---Writes a replicated state-bag value with an explicit replication flag override.
---@param key string
---@param value any
---@param replicated? boolean
function GlobalState:set(key, value, replicated) end

---Registers a state-bag change handler backed by the function-reference bridge.
---@param keyFilter? string
---@param bagFilter? string
---@param handler fun(bagName: string, key: string, value: any, source: string|number, replicated: boolean)
---@return integer
function AddStateBagChangeHandler(keyFilter, bagFilter, handler) end
