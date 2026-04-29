# FiveM custom Lua runtime reference

## Scope

This document describes the custom Lua runtime implemented by FiveM in the following components:

- `code/components/citizen-scripting-lua/`
- `data/shared/citizen/scripting/lua/`
- `code/tests/server/TestLua.cpp`

It is a reference document. It describes the runtime surface, initialisation sequence, built-in globals, host integration points, scheduler behaviour, native-binding semantics, and server-specific standard-library replacements.

Unless otherwise stated, behaviour is described from the implementation in:

- `code/components/citizen-scripting-lua/src/LuaScriptRuntime.cpp`
- `code/components/citizen-scripting-lua/src/LuaScriptNatives.cpp`
- `code/components/citizen-scripting-lua/src/LuaNativesLoader.cpp`
- `code/components/citizen-scripting-lua/src/LuaIO.cpp`
- `code/components/citizen-scripting-lua/src/LuaOS.cpp`
- `data/shared/citizen/scripting/lua/scheduler.lua`
- `data/shared/citizen/scripting/lua/natives_loader.lua`

## Runtime version and state model

### Lua version

The runtime is based on Lua 5.4.

`code/tests/server/TestLua.cpp` asserts `LUA_VERSION_NUM == 504`.

This runtime must not be treated as stock upstream Lua 5.4.

Build configuration under `code/vendor/lua.lua` and `code/components/citizen-scripting-lua/component.lua` shows that FiveM builds a patched Lua dialect with additional compile-time features, including:

- `LUA_COMPAT_5_3`
- `LUA_SANDBOX`
- GRIT power extensions
- GLM integration
- bundled `cmsgpack`, `rapidjson`, and `lmprof` integration

Language-server consumers should therefore model the runtime as FiveM custom Lua rather than plain Lua 5.4.

### State construction

`LuaStateHolder` owns the `lua_State*` for each runtime instance.

On construction:

1. A new Lua state is created.
2. Garbage collection is switched to generational mode using `lua_gc(m_state, LUA_GCGEN, 0, 0)`.
3. On Windows builds with `LUA_USE_RPMALLOC`, the state allocator may use a dedicated rpmalloc heap stored as opaque allocator data and freed when the state closes.

Each `LuaScriptRuntime` owns one state. The state is closed during `LuaScriptRuntime::Destroy()` while the current runtime environment is still pushed, so Lua finalisers can still observe an active runtime context.

## Implemented runtime interfaces

`LuaScriptRuntime` implements the following host-facing interfaces:

- `IScriptRuntime`
- `IScriptFileHandlingRuntime`
- `IScriptTickRuntimeWithBookmarks`
- `IScriptEventRuntime`
- `IScriptRefRuntime`
- `IScriptMemInfoRuntime`
- `IScriptStackWalkingRuntime`
- `IScriptDebugRuntime`
- `IScriptProfiler`
- `IScriptWarningRuntime`

Important runtime members visible from the implementation include:

- host pointers for basic script hosting, bookmark hosting, resource data, and manifest data
- `m_boundaryRoutine`
- `m_instanceId`
- `m_nativesDir`
- `m_runningThreads`
- `m_nonExistentNatives`
- `m_pendingBookmarks`

`GetResourceName()` resolves the resource name through `IScriptHostWithResourceData::GetResourceName`. If resource data is unavailable, it returns an empty string.

`GetNativesDir()` returns the active lazy-native mount root, if one is being used.

## State initialisation sequence

`LuaScriptRuntime::Create(IScriptHost*)` performs the runtime initialisation sequence.

### Opened libraries

The runtime opens a curated library set rather than the stock Lua distribution.

Common libraries:

- `_G`
- `table`
- `string`
- `math`
- `coroutine`
- `utf8`
- custom debug library via `lua_fx_opendebug`
- `msgpack`
- `json`

The stock `io` and `os` libraries are not generally opened.

On FXServer builds only, the runtime installs custom replacements:

- `fx::lua_fx_openio`
- `fx::lua_fx_openos`

### Runtime bootstrap order

The runtime initialises in this order:

1. Store host pointers.
2. Initialise bookmark infrastructure.
3. Open the curated libraries.
4. Store the `debug.traceback` C function pointer in `m_dbTraceback`.
5. Create the global `Citizen` table from `g_citizenLib`.
6. Load the native wrapper build through `LoadNativesBuild`.
7. Load system scripts:
   - `citizen:/scripting/lua/deferred.lua`
   - `citizen:/scripting/lua/scheduler.lua`
   - `citizen:/scripting/lua/graph.lua`
8. Remove `dofile`.
9. Remove `loadfile`.
10. Replace global `print` with `Lua_Print`.
11. Replace global `require` with `Lua_Require`.
12. Install the Lua warning callback through `lua_setwarnf` using `Lua_Warn`.

The runtime does not retain a general-purpose stock module loader.

## File handling and chunk loading

### Supported file types

`HandlesFile()` returns true for any file name containing `.lua`.

### Script loading pipeline

`LoadFileInternal` is the authoritative file-loading path for Lua source.

Behaviour:

1. Read bytes from a host stream or system stream.
2. Allow `OnBeforeLoadScript` hooks to mutate the script bytes before compilation.
3. Append a terminating NUL byte.
4. Compile the chunk with a source name prefixed as `@<scriptFile>`.

When the script name follows the `@resourceName/...` form, the runtime additionally resolves that resource from `ResourceManager::GetCurrent()` and invokes that resource's `OnBeforeLoadScript` hook before invoking the current parent resource hook.

### Host-file execution

`LoadHostFileInternal` wraps a successfully loaded chunk inside `Lua_CreateHostFileThread`, so host files execute through `CreateThreadNow` semantics rather than as a direct top-level call.

### Running a script

`RunFileInternal` always:

1. Pushes `debug.traceback`.
2. Invokes the load function.
3. Executes the loaded chunk with `lua_pcall`.

On failure it reports an error using the form:

`Error loading script %s in resource %s`

## Global environment

### Removed or replaced globals

The runtime explicitly removes:

- `dofile`
- `loadfile`

The runtime replaces:

- `print` with `Lua_Print`
- `require` with `Lua_Require`

No general module search path or package loader is installed.

### `require`

`Lua_Require` supports only two built-in module names directly:

- `lmprof`
- `glm`

Any other module name fails with:

`module '%s' not found`

The implementation contains an explicit TODO for future VFS-backed module loading. That TODO is not active behaviour.

## `Citizen` global table

The runtime creates a global table named `Citizen` from `g_citizenLib`.

### Scheduler and lifecycle members

- `SetBoundaryRoutine`
- `SetTickRoutine`
- `SetEventRoutine`
- `CreateThread`
- `CreateThreadNow`
- `Wait`
- `SetTimeout`
- `ClearTimeout`

`SetTickRoutine` is currently a no-op setter function that returns `0` from the C binding.

### Native invocation members

- `InvokeNative`
- `LoadNative`

Client builds additionally expose:

- `GetNative`
- `InvokeNative2`

### Function-reference members

- `SetCallRefRoutine`
- `SetDeleteRefRoutine`
- `SetDuplicateRefRoutine`
- `CanonicalizeRef`
- `InvokeFunctionReference`

### Boundary and stack-trace members

- `SubmitBoundaryStart`
- `SubmitBoundaryEnd`
- `SetStackTraceRoutine`

### Pointer and result helpers

- `PointerValueIntInitialized`
- `PointerValueFloatInitialized`
- `PointerValueInt`
- `PointerValueFloat`
- `PointerValueVector`
- `ReturnResultAnyway`
- `ResultAsInteger`
- `ResultAsLong`
- `ResultAsFloat`
- `ResultAsString`
- `ResultAsVector`
- `ResultAsObject`
- `ResultAsObject2`
- `AwaitSentinel`

`ResultAsObject` is a compatibility no-op at the Lua-facing surface unless the runtime has an object-deserialisation callback installed. If no callback is installed, object results become `nil`.

## Native wrapper loading

### Build selection

The runtime selects a native-wrapper build according to manifest-version checks.

Baseline:

- `natives_21e43a33.lua`

Possible upgrades:

- `natives_0193d0af.lua`
- `natives_universal.lua`
- `rdr3_universal.lua`
- `ny_universal.lua`
- `natives_server.lua`

If manifest version V2 is at least `adamant`, the runtime selects the universal build for the current platform.

### Eager versus lazy native loading

`LoadNativesBuild` behaves in one of two modes.

#### Direct system-file mode

If `fx::mountedAnyNatives` is false, the runtime loads the generated native wrapper file directly from:

- `citizen:/scripting/lua/<build>`

#### Lazy archive-backed mode

If `fx::mountedAnyNatives` is true:

1. `m_nativesDir` is set to `nativesLua:/<build-without-.lua>/`.
2. The runtime loads `citizen:/scripting/lua/natives_loader.lua`.

### Archive mounting

`LuaNativesLoader.cpp` mounts in-memory zip devices for native-wrapper archives under `nativesLua:/<build-name>/`.

Mounted archives include:

- GTA Five:
  - `natives_universal`
  - `natives_21e43a33`
  - `natives_0193d0af`
- RDR3:
  - `rdr3_universal`
- GTA NY:
  - `ny_universal`
- FXServer:
  - `natives_server`

A `MarkerDevice` mounted at `nativesLua:/marker/` prevents duplicate mounting.

## Lazy native global materialisation

`data/shared/citizen/scripting/lua/natives_loader.lua` installs a metatable on the real `_G` table.

On lookup of an unknown global:

1. If the name is in `nilCache`, the lookup resolves as missing.
2. Otherwise, call `Citizen.LoadNative(name)`.
3. If the result is a function, cache that function directly in `_G`.
4. If the result is a string, compile it as Lua source using:
   - source name `@<name>.lua`
   - text mode `t`
   - environment `nativeEnv`
5. Execute the compiled wrapper.
6. Read the resulting global back from `_G`.

Missing native names are memoised in `nilCache` to avoid repeated host lookups.

### `nativeEnv`

The wrapper environment used by `natives_loader.lua` exposes helper bindings including:

- `Global`
- `_mfr`
- `_obj`
- `_ch`
- `msgpack`
- pointer/result helper bindings
- `Citizen.InvokeNative`
- `Citizen.InvokeNative2` when available
- `Citizen.GetNative` when available

`Global` mirrors assignments into the actual global table.

The key observable property is that native globals are materialised lazily rather than being defined eagerly at runtime startup.

## Scheduler layer

`data/shared/citizen/scripting/lua/scheduler.lua` is loaded by every Lua runtime instance and provides the high-level coroutine, event, promise, and stack-trace layer.

### Compatibility configuration

The scheduler configures compatibility options for:

- `msgpack`
- `json`

### Boundary wrappers

The scheduler uses `Citizen.SubmitBoundaryStart` and `Citizen.SubmitBoundaryEnd` to create boundary wrappers.

Boundaries serve two purposes:

- host-side runtime bookkeeping
- Lua stack-trace delimiting

### Root aliases

The scheduler publishes root-level aliases:

- `Wait`
- `CreateThread`
- `SetTimeout`
- `ClearTimeout`

### Promise awaiting

`Citizen.Await(promise)` must execute inside a scheduler coroutine.

If it is called outside a scheduler coroutine, it asserts with guidance instructing the caller to use `CreateThread`, `SetTimeout`, or an event handler.

### Event dispatch layer

The scheduler stores handlers in a local `eventHandlers` registry.

`Citizen.SetEventRoutine` installs the host callback used for event delivery.

Event payloads are transported as msgpack byte strings and are unpacked inside the scheduler before Lua handler dispatch.

Observed behaviour of the installed event routine:

1. Unpack the incoming payload from msgpack.
2. Set global `_G.source`.
3. Enforce `safeForNet` for network-originated events.
4. Normalise `source` for `net:` and `internal-net` origins.
5. Dispatch each handler inside `Citizen.CreateThreadNow`.

`_G.source` handling is shape-dependent:

- local or host-originated events initially expose the original `eventSource` string
- `net:<id>` sources are converted to numeric player or peer identifiers before handler execution
- on server builds, `internal-net:<id>` is also normalised to a numeric identifier
- after dispatch, the previous `_G.source` value is restored

If an event arrives from the network and the event entry was not marked `safeForNet`, the scheduler traces `event <name> was not safe for net` and skips handler dispatch.

### Event API

#### `AddEventHandler(eventName, handler)`

- Registers the handler in the local registry.
- Calls `RegisterResourceAsEventHandler(eventName)`.
- Returns a token table shaped as `{ key = ..., name = eventName }`.

#### `RemoveEventHandler(token)`

- Validates the token structure.
- Removes the handler by key.

#### `RegisterNetEvent(eventName[, callback])`

- Marks the event as `safeForNet` unless it belongs to the ignored internal-event set.
- When a callback is supplied, registration also wires the handler.

The ignored internal-event set includes `__cfx_internal:commandFallback`, which is intentionally not marked through the normal safe-for-net path.

#### `TriggerEvent(...)`

- Wraps `TriggerEventInternal` with `runWithBoundaryEnd`.
- Serialises Lua arguments with `msgpack.pack_args(...)` before host delivery.

#### Event cancellation

The host event layer exposes:

- `CancelEvent()`
- `WasEventCanceled()`

`TRIGGER_EVENT_INTERNAL` returns whether the last triggered event was cancelled. Cancellation is tracked by the host event manager using a thread-local cancellation stack, so cancellation state is scoped to the active event-dispatch chain rather than being a permanent property on the event name.

#### Resource event routing

`AddEventHandler` registration has a host-side routing effect in addition to local table insertion.

`REGISTER_RESOURCE_AS_EVENT_HANDLER` records the resource as a handler for the event name inside the resource manager. The host event manager later routes manually triggered events to:

- resources registered for the exact event name
- resources registered for the wildcard name `*`

Manually emitted `__cfx_nui:` events are excluded from that broadcast path and are instead routed through the explicit NUI callback bridge.

### Server-side scheduler additions

When `isDuplicityVersion` is true, the scheduler additionally publishes server-oriented helpers including:

- `TriggerClientEvent`
- `TriggerLatentClientEvent`
- `RegisterServerEvent` as an alias of `RegisterNetEvent`
- `RconPrint` as an alias of `Citizen.Trace`
- helper functions such as `GetPlayerIdentifiers`, `GetPlayerTokens`, `GetPlayers`, `PerformHttpRequest`, and related server helpers

`TriggerClientEvent` and `TriggerLatentClientEvent` serialise arguments with msgpack before calling the host natives `TriggerClientEventInternal` and `TriggerLatentClientEventInternal`.

`TriggerLatentClientEvent` adds a bytes-per-second throttle argument. The scheduler coerces that `bps` argument with `tonumber(bps)` before host delivery.

`PerformHttpRequest` is a server-side scheduler helper, not stock Lua networking. It builds a request table, calls `PerformHttpRequestInternalEx`, and dispatches completion through the internal event `__cfx_internal:httpResponse`. `PerformHttpRequestAwait` wraps the same path with a promise and `Citizen.Await`.

### Client-side scheduler additions

When `isDuplicityVersion` is false, the scheduler publishes client-to-server event helpers:

- `TriggerServerEvent`
- `TriggerLatentServerEvent`

These helpers serialise Lua arguments with msgpack and then call `TriggerServerEventInternal` or `TriggerLatentServerEventInternal`.

`TriggerLatentServerEventInternal` uses the event-reassembly component on the client. When `sv_enableNetEventReassembly` is disabled, the disabled path emits a warning rather than performing latent reassembly.

The client scheduler also layers additional UI and replication bridges into the Lua environment:

- `RegisterNuiCallback`
- `RegisterNUICallback`
- `SendNUIMessage`
- state-bag helpers such as `GlobalState`

These are networking-adjacent host bridges, not native Lua facilities.

### Function-reference transport

The scheduler configures `Citizen.SetCallRefRoutine`, `Citizen.SetDuplicateRefRoutine`, and `Citizen.SetDeleteRefRoutine` to support host-mediated function-reference transport.

Observable properties:

- Lua functions are assigned runtime-local entries in `funcRefs`
- references are serialised through msgpack extensions rather than raw closure sharing
- incoming references are wrapped in callable userdata-like tables carrying `__cfx_functionReference`
- invocation uses `Citizen.InvokeFunctionReference`
- asynchronous cross-runtime results are represented using a sentinel table containing `__cfx_async_retval`

More precise callable-proxy behaviour:

- `msgpack.settype("function", EXT_FUNCREF)` causes plain Lua `function` values to be serialised as function references whenever msgpack-backed bridges pack arguments
- unpacking a function reference creates a table with raw field `__cfx_functionReference`, duplicates the underlying reference, and attaches a callable metatable/extension object
- the imported value therefore remains callable through `__call`, but `type(value)` is table-like rather than `function`
- proxy indexing and assignment are rejected with `Cannot index a funcref` and `Cannot set indexes on a funcref`
- garbage collection of the proxy deletes the duplicated reference through `DeleteFunctionReference(...)`
- reserialising an imported callable creates a new local wrapper closure through `Citizen.GetFunctionReference`, rather than forwarding the original canonical reference unchanged
- async call-ref results are auto-awaited only when the proxy is invoked from a running scheduler coroutine; outside that context the async sentinel table is returned as a normal Lua value

This is the mechanism used by callback-style bridges such as exports, NUI callback result handlers, and state-bag change handlers.

`specs/lua-function-reference-reference.md` describes the complete packing, unpacking, invocation, and lifetime rules.

### Export bridge

The scheduler's `exports` object is an event-backed cross-resource bridge layered on top of normal event delivery.

Provider side:

- metadata key `export` is used on client builds
- metadata key `server_export` is used on server builds
- at runtime startup, the scheduler reads metadata for the current resource and registers one event handler per export name using the synthetic event name `__cfx_export_<resource>_<name>`
- provider handlers call the setter callback with `_G[exportName]` when that global exists
- manual registration through `exports(exportName, func)` registers the same synthetic event explicitly

Caller side:

- `exports[resource][name]` lazily triggers the synthetic export event
- the returned callback is cached in `exportsCallbackCache[resource][name]`
- missing exports raise `No such export <name> in resource <resource>`
- invocation wraps the cached callback in `pcall`
- failures are rethrown with an export-specific formatted error message

Cache invalidation:

- the cache is cleared when `onClientResourceStop` or `onServerResourceStop` is observed for the provider resource
- invalidation is therefore queued behind those lifecycle events rather than being synchronous with provider teardown

Special case:

- export event-name generation rewrites resource name `txAdmin` to `monitor`

### NUI callback bridge

On client builds, scheduler-level NUI support exposes two related callback paths:

- `RegisterNuiCallback(type, callback)` wraps the native callback registration path directly
- `RegisterNUICallback(type, callback)` preserves the legacy event-based path by registering `__cfx_nui:<type>` and `RegisterNuiCallbackType(type)`

Observable behaviour:

- inbound JSON POST data is parsed and converted to msgpack by `ResourceUICallbacks.cpp`
- legacy mode queues a resource event named `__cfx_nui:<type>` with event source `nui`
- callback result channels are transported as function references created by the resource callback component
- Lua callback execution is wrapped in `pcall`, and failures are traced as `error during NUI callback <type>: ...`
- `SendNUIMessage(message)` JSON-encodes the supplied Lua value before calling the lower-level native sender

### State-bag bridge

The scheduler exposes state bags as proxy tables backed by host natives, not as normal Lua tables.

Observable behaviour:

- `GlobalState` is created eagerly as `NewStateBag('global')`
- reads call `GetStateBagValue`
- assignment serialises the Lua value with msgpack and calls `SetStateBagValue`
- the `set` helper permits an explicit replicated flag

Host-side `AddStateBagChangeHandler` registration is implemented in `ResourceScriptFunctions.cpp` using function references. Registered handlers are disconnected automatically when the owning resource stops.

### Stack-trace routine

The scheduler configures `Citizen.SetStackTraceRoutine` with a routine that:

- scans Lua frames
- skips scheduler and internal frames
- emits msgpack-packed frame objects shaped as `{ file, line, name }`

Boundary metadata stored in the wrapper closure upvalues is used to delimit the captured stack.

## Coroutine scheduling and bookmarks

The host scheduler and the Lua scheduler cooperate through bookmarks.

### Bookmark payload

Each bookmark stores a Lua registry table containing:

1. the coroutine
2. the profiler or display name
3. the boundary identifier

### Thread creation

`CreateThread` and `SetTimeout` create:

- a coroutine
- a boundary-wrapped entry function
- a registry bookmark

`CreateThreadNow` runs the coroutine immediately.

### Yield protocol

`Wait` yields its numeric argument.

When the host resumes a bookmark through `RunBookmark`:

- a numeric or `nil` yield causes rescheduling through `ScheduleBookmark(this, bookmark, -wakeTime)`
- yielding the special `AwaitSentinel` lightuserdata causes immediate re-resume with a closure that reattaches the bookmark

### Deferred host scheduling

`ScheduleBookmarkSoon` only accumulates `(bookmark, timeout)` tuples in `m_pendingBookmarks`.

Actual host scheduling is performed later by `SchedulePendingBookmarks()`.

`LuaPushEnvironment` and `fx::PushEnvironment` automatically call `SchedulePendingBookmarks()` when the pushed environment scope ends.

### Error handling on resumed threads

If a coroutine resumes with an error:

1. The runtime logs `^1SCRIPT ERROR: ...^7`.
2. It invokes native `FORMAT_STACK_TRACE`.
3. It calls `lua_resetthread` to close to-be-closed variables.
4. If `lua_resetthread` itself fails, the secondary reset error is also logged.

## Native invocation semantics

`LuaScriptNatives.cpp` provides the generic native marshalling layer.

Native wrappers ultimately funnel through `Lua_InvokeNative` and `Lua_DoInvokeNative` using a `ScriptNativeContext`.

### Argument conversions

Supported argument shapes include:

- `nil` or `false` -> `0`
- `true` -> `1`
- integers
- floating-point numbers
- strings
- `vector2`
- `vector3`
- `vector4`
- quaternions
- tables with a `__data` field
- tables whose metatable exposes `__data`
- lightuserdata used for meta pointers

Unsupported shapes raise Lua errors such as:

- `invalid lua type`
- `invalid lua type in __data`

### Result conversions

The implementation converts native results into Lua values for at least the following host result shapes:

- boolean
- 32-bit integer
- 64-bit integer
- float
- `ScrVector` mapped to GLM `vec3`
- `const char*`
- `ScrString`
- `ScrObject` through the runtime object-result callback

Exceptions raised during native invocation are converted into Lua errors.

## Warning and memory reporting

### Warnings

`EmitWarning(channel, message)` forwards warnings through the Lua warning mechanism in this format:

`[channel] message`

### Memory usage

`GetMemoryUsage()` reports Lua heap usage as:

`LUA_GCCOUNT * 1024 + LUA_GCCOUNTB`

## Server-only `os` library

On FXServer, the runtime installs a custom `os` library from `LuaOS.cpp`.

This is not the stock Lua `os` implementation.

### `os.execute(command)`

- `os.execute(nil)` returns `true`.
- Any non-`nil` command is denied and returns `(nil, "Permission denied", fx::Lua_EACCES)`.

### `os.createdir(path)`

- Uses FiveM VFS devices.
- Returns `true` on success.
- On failure it returns `nil`, an explanatory message, and an integer error code.
- The implementation uses code `1` for conditions such as “already exists” and “failed to create directory”, or standard file-result failures when no device resolves.

### `os.remove(path)`

- Succeeds only if the path resolves to a VFS device and `ScriptingFilesystemAllowWrite(path)` permits the operation.
- Otherwise returns `Permission denied` with `fx::Lua_EACCES`.

### `os.rename(from, to)`

- Requires both paths to resolve through the VFS layer.
- Requires write permission for both paths.
- Otherwise returns `Permission denied` with `fx::Lua_EACCES`.

### `os.tmpname()`

- Returns deterministic names in the form `tmp_<n>`.
- It does not expose an operating-system temporary path.

### `os.getenv(key)`

- Lowercases the key before lookup.
- Only exposes the key `os`.
- Returns `Windows` or `Linux`.
- All other keys return `nil`.

### `os.setlocale(locale, category)`

- Validates the category argument.
- Returns the supplied locale string.
- Does not mutate the process locale.

### Time helpers

The custom library also provides host-implemented time-related helpers including:

- `os.deltatime`
- `os.microtime`
- `os.nanotime`
- `os.rdtsc` when supported
- `os.rdtscp` when supported

Standard-looking helpers such as date, time, clock, and difftime are also implemented in host code rather than delegated to a stock platform `os` library.

## Server-only `io` library

On FXServer, the runtime installs a custom `io` library from `LuaIO.cpp`.

This library is backed by FiveM VFS streams rather than C stdio.

### File userdata model

- File userdata stores `vfs::Stream*` inside `luaL_Stream`.
- Closure uses `LuaVFSStreamClose`.

### `io.open(path, mode)`

Behaviour:

1. Reject any path segment equal to `..` with `ENOENT`.
2. Resolve `@` paths with `vfs::GetDevice`.
3. Resolve absolute or relative paths with `vfs::FindDevice`.
4. If `ScriptingFilesystemAllowWrite(path)` denies writes, suppress write, create, and append capabilities.
5. Return an open file handle on success.
6. On resolution or open failure, return `(nil, <strerror>, ENOENT)`.

### `io.popen(command, mode)`

- Does not spawn a process.
- Only recognises commands beginning with `dir` or `ls`.
- Expects a quoted path operand.
- Resolves that path through the VFS layer.
- Returns a custom directory userdata that enumerates entries.
- Any other command form or malformed quoted path fails.

### `io.readdir(path)`

- Enumerates directory contents through the VFS layer.
- Skips `.` and `..`.
- Returns the same directory userdata type used by `io.popen`.

### Directory userdata surface

Directory userdata supports:

- `:close()`
- `:lines()`
- `__gc`
- `__tostring()`

`__tostring()` returns newline-joined filenames or `directory (closed)`.

### `io.tmpfile()`

- Always fails.

### File reads

Supported read modes include:

- default line reads without the trailing newline
- numeric byte counts
- `*n`
- `*l`
- `*L`
- `*a`

`file:lines()` is implemented with a 64 KiB intermediate buffer.

### File writes

- Numeric writes use Lua integer or number formatting.
- String writes are forwarded directly.
- After writing, the VFS file is truncated to the current position if the new contents are shorter than the original stream length.

### Other `io` surface details

- `file:setvbuf(...)` succeeds but is a no-op.
- Global `io.write(...)` delegates to `fx::Lua_Print`, which means it emits script trace output rather than writing to a writable stdout stream.
- `stdin`, `stdout`, and `stderr` are emulated as empty or closed file handles.
- Module-level `io.lines` is an empty iterator rather than the stock file-opening helper.

## Observable behaviour confirmed by tests

`code/tests/server/TestLua.cpp` confirms observable runtime behaviour including:

- `io.open("test.txt", "rb")` on a missing file returns `nil`
- `os.remove("test.txt")` returns `fx::Lua_EACCES`
- renaming non-existent files returns `EACCES`
- `os.tmpname()` returns strings prefixed with `tmp_`
- `os.getenv("OS")` and `os.getenv("Os")` both return the operating-system name
- `os.getenv("%PATH%")` returns `nil`
- `os.setlocale("en-US", "all")` returns the provided locale string

## Current omissions from this document

This document describes the Lua runtime itself.

Resource-manifest execution is specified in `specs/lua-manifest-reference.md`.

Resource lifecycle semantics are specified in `specs/lua-resource-management-reference.md`.

Runtime environment isolation is specified in `specs/lua-environment-isolation-reference.md`.

Native-build selection and marshaling semantics are specified in `specs/lua-native-binding-reference.md`.
