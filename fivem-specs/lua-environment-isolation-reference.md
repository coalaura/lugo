# FiveM Lua environment isolation reference

## Scope

This document describes how FiveM isolates Lua execution environments between resources, between client and server sides, and between runtime instances.

It also documents the boundaries that are not enforced, so consumers can distinguish guaranteed isolation from convention-only behaviour.

Primary implementation files:

- `code/components/citizen-scripting-lua/include/LuaScriptRuntime.h`
- `code/components/citizen-scripting-lua/src/LuaScriptRuntime.cpp`
- `code/components/citizen-scripting-lua/src/LuaScriptNatives.cpp`
- `code/components/citizen-scripting-lua/src/LuaDebug.cpp`
- `data/shared/citizen/scripting/lua/scheduler.lua`
- `data/shared/citizen/scripting/lua/natives_loader.lua`
- `code/components/citizen-scripting-core/src/ResourceScriptingComponent.cpp`
- `code/components/citizen-scripting-core/src/ScriptHost.cpp`
- `code/components/citizen-scripting-core/src/EventScriptFunctions.cpp`
- `code/components/citizen-scripting-core/src/RefScriptFunctions.cpp`
- `code/components/citizen-scripting-core/src/FilesystemPermissions.cpp`

## Isolation unit

The isolation unit for Lua execution is `fx::LuaScriptRuntime`.

Each `LuaScriptRuntime` owns exactly one `LuaStateHolder m_state`, and each `LuaStateHolder` owns one `lua_State*`.

This means the state boundary is:

- one Lua state per runtime instance
- one runtime instance per resource side that uses the Lua runtime

## Resource-level runtime allocation

`ResourceScriptingComponent` creates runtimes per resource side.

For a resource that contains Lua scripts:

- one client-side resource instance gets its own Lua runtime
- one server-side resource instance gets its own Lua runtime

All `shared_script` and side-specific Lua files selected for that resource side are loaded into that same runtime state.

Result:

- globals are shared between all Lua files in one resource on one side
- globals are not shared between different resources
- globals are not shared between client and server because those are separate resource instances in separate processes or execution targets

## Side separation

`IS_DUPLICITY_VERSION` exposes whether the current runtime is running in a server build.

The same general state-isolation model is used on both sides, but the injected library surface differs:

- client: no server `io` or `os` library is opened by the Lua runtime bootstrap
- server: custom `io` and `os` libraries are opened

This means client isolation is stricter by default at the standard-library level.

## Intra-resource sharing model

### Shared global table

The runtime loads user chunks into the runtime global environment.

There is no per-file `_ENV` sandboxing layer for normal resource scripts.

Consequences:

- one script can assign globals consumed by another script in the same resource runtime
- one script can overwrite helper functions, event tables, or export functions created by another script in the same resource runtime
- top-level local variables remain file-local in the usual Lua sense, but `_G` mutations are resource-global

### Shared scheduler state

`scheduler.lua` keeps state such as:

- `eventHandlers`
- function-reference tables
- export callback caches

These live inside one runtime state and therefore are shared by all scripts loaded into that resource runtime.

### Shared dynamic event context

The scheduler sets `_G.source` while dispatching events.

`source` is therefore dynamic shared runtime state, not a thread-local or file-local variable in the Lua implementation.

## Cross-resource isolation

### No shared `_G`

Different resources do not share one Lua global table.

They have separate `lua_State*` instances.

Direct table access from one resource into another resource's `_G` is not part of the Lua runtime model.

### Explicit bridges only

Cross-resource interaction is implemented through explicit host-supported bridges:

1. resource events
2. exports
3. function references
4. host-mediated file access

There is no implicit namespace import of another resource’s globals.

## Event boundary semantics

### Resource registration

Lua `AddEventHandler` calls native `REGISTER_RESOURCE_AS_EVENT_HANDLER`.

This binds event handling to the current resource rather than to a global Lua handler registry shared across resources.

### Runtime-local handler storage

Within one runtime, handlers are stored in the local scheduler table `eventHandlers`.

This means:

- inter-resource event routing is host-controlled
- intra-resource handler lists are runtime-local

### Dispatch shape

For a delivered event, the scheduler:

1. unpacks msgpack payloads
2. updates `_G.source`
3. enforces net-event safety rules
4. dispatches each handler via `Citizen.CreateThreadNow`

The handler coroutine is isolated from the dispatcher stack, but not from the resource-global state.

Additional observable event-boundary details:

- network-originated events are blocked unless the runtime-local event entry was marked `safeForNet` by `RegisterNetEvent`
- `net:<id>` and server-side `internal-net:<id>` sources are normalised to numeric identifiers before Lua handlers run
- the previous `_G.source` value is restored after dispatch, so event source is dynamic shared state rather than persistent per-handler state
- host cancellation state is tracked per active dispatch chain, not as mutable state stored inside the Lua handler table

### Event routing boundary

The host event manager routes manually triggered events only to resources registered as handlers for:

- the exact event name
- the wildcard event name `*`

Legacy NUI event names beginning with `__cfx_nui:` are excluded from the generic broadcast path and are routed through the explicit UI callback bridge instead.

## Coroutine isolation semantics

### Coroutine ownership

Lua coroutines are owned by the runtime state that created them.

Bookmark data stored by the host scheduler points back into that runtime state.

### Per-resource scheduler boundary

Bookmarks and resumed threads are not shared across resources.

When a resource runtime is destroyed:

- its bookmarks are removed from the host queue
- its Lua state is closed
- its coroutines, registry entries, and scheduler tables disappear with that state

### No per-file coroutine isolation

Coroutines created by different files in the same resource all live in the same runtime state.

They can observe the same globals and shared scheduler tables.

## Native-global injection boundary

### Per-runtime lazy injection

`natives_loader.lua` installs a metatable on the runtime’s own `_G` table.

Missing globals are resolved by calling `Citizen.LoadNative` in that runtime.

This behaviour is per runtime, not global across the entire process.

### Missing-native cache

Non-existent native names are cached per runtime.

A native lookup failure in one resource runtime does not by itself populate another resource runtime’s missing-native cache.

## Module-loading boundary

### No standard package system

The Lua runtime does not expose the stock package searcher chain.

`require` is custom and only recognises specific built-in modules such as `lmprof` and `glm`.

Therefore there is no stock cross-file or cross-resource module loading mechanism based on `package.path` or `package.cpath`.

### Removed file-loading helpers

The runtime removes:

- `dofile`
- `loadfile`

This blocks the usual stock-Lua file execution helpers inside resource scripts.

## Metatable and debug boundary

### Exposed debug surface

The custom debug library exposes metatable accessors including:

- `debug.getmetatable`
- `debug.setmetatable`

### Consequence

Metatable mutation is not sandbox-blocked within one runtime.

A script can therefore alter metatables for values reachable inside its own resource runtime.

This is a deliberate non-guarantee for intra-resource isolation.

## Filesystem boundary

### Client side

The client runtime does not receive the server custom `io` or `os` libraries from Lua bootstrap.

This removes the custom VFS-backed file API from the normal client-side Lua environment.

### Server side

The server runtime exposes VFS-backed `io` and `os` libraries.

Write operations are gated by `ScriptingFilesystemAllowWrite`.

Observable consequences:

- writes are permission-checked
- deletes and renames are permission-checked
- directory creation is host-controlled

### Non-guarantee for reads

The examined permission implementation gates writes but does not provide an equivalent general read-permission barrier in the same path.

Therefore server-side filesystem isolation is stronger for writes than for reads.

## Cross-resource file loading boundary

The host file loader recognises `@resourceName/path.lua` syntax.

When such a path is loaded through host-mediated script loading:

1. the referenced resource is resolved
2. the referenced resource’s `OnBeforeLoadScript` hook is invoked
3. the current resource’s `OnBeforeLoadScript` hook is invoked

This is an explicit host bridge, not shared-state execution.

The normal Lua helpers `dofile` and `loadfile` are absent, so user code does not receive the stock cross-file execution path.

## Function-reference boundary

### Canonical representation

Cross-runtime function references are encoded as canonical strings of the form:

`resource:instanceId:refId`

### Lookup model

Resolution is performed by host-side reference logic, not by sharing Lua closures directly between runtime states.

The receiving runtime invokes the referenced function through the call-ref bridge.

This means function references cross runtime boundaries as host-managed handles rather than as shared Lua closure objects.

Function references are also the callback transport used by adjacent networking bridges such as:

- export resolution callbacks
- NUI result callbacks
- state-bag change handlers

Imported function references cross into a runtime as callable proxy tables rather than as native closures. The receiving runtime can call them, but normal Lua type checks observe table-like values with a metatable-backed `__call` path.

`specs/lua-function-reference-reference.md` describes the exact proxy shape and repacking behaviour.

## Export boundary

Exports are not implemented as direct table access into another resource.

Instead:

1. the provider resource registers an event-backed export endpoint
2. the caller resolves a callable export through that event
3. the caller caches the resulting callback until stop invalidation

Exports therefore cross resource boundaries through an event-and-callback bridge rather than through shared Lua tables.

Additional observable properties:

- export discovery uses synthetic event names shaped as `__cfx_export_<resource>_<name>`
- provider-side metadata registration uses `export` on client and `server_export` on server
- callers cache resolved export callbacks locally per runtime
- cache invalidation happens only when the queued side-specific resource-stop event is processed
- resource name `txAdmin` is rewritten to `monitor` for export event naming

This means export visibility is bridge-mediated, runtime-local, and eventually invalidated rather than synchronously tied to another resource's global table lifetime.

## NUI boundary

Client-side NUI callbacks are not plain Lua-to-Lua calls.

They cross a host UI bridge with these properties:

- request bodies originate as JSON payloads from the UI layer
- payloads are converted to msgpack before entering the Lua scheduler
- legacy callback mode enters Lua as a resource event named `__cfx_nui:<type>` with source `nui`
- result callbacks cross back through a function-reference channel rather than direct shared closures

NUI therefore shares the event/callback infrastructure but remains a dedicated host boundary, not a normal cross-resource Lua call.

## State-bag replication boundary

State bags are host-managed replicated data stores exposed to Lua through proxy tables.

Observable boundary properties:

- reads and writes cross the host boundary through `GetStateBagValue` and `SetStateBagValue`
- values are serialised with msgpack
- change handlers are connected at the host state-bag component, not by polling a shared Lua table
- the host passes `(bagName, key, value, source, replicated)` into the callback reference
- handler registrations are disconnected when the owning resource stops

State-bag access therefore exposes replicated shared state, but the replication and callback routing are host-controlled rather than implemented by shared Lua memory between resources.

## Runtime activation boundary

`ScriptHost.cpp` uses runtime push and pop helpers to establish the active runtime context around host callbacks.

This active-runtime context governs:

- current runtime lookup
- invoking runtime lookup
- later pending-bookmark scheduling on scope exit

This context is host-managed and scoped. It is not implemented as a globally shared mutable Lua variable visible to user code in a stable way.

## Cleanup guarantees on stop and restart

When a resource stops:

1. `ResourceScriptingComponent` destroys its runtimes.
2. The Lua runtime removes queued bookmarks.
3. Event, tick, and ref routines are cleared.
4. The Lua state is closed.

Consequences:

- globals do not survive resource restart
- loaded modules in `LUA_LOADED_TABLE` do not survive resource restart
- coroutines do not survive resource restart
- cached native wrappers inside `_G` do not survive resource restart
- scheduler tables do not survive resource restart

Restart produces a fresh runtime state.

## Guarantees

- One Lua runtime owns one Lua state.
- Different resources use different Lua states.
- Client and server resource runtimes are separate.
- Cross-resource interaction is host-mediated rather than global-table sharing.
- Bookmarks, coroutines, globals, and loaded modules are torn down when the runtime state closes.
- Native global materialisation is runtime-local.

## Not guaranteed

- File-level isolation inside one resource runtime.
- Metatable isolation within one resource runtime.
- Immutable scheduler tables within one resource runtime.
- Thread-local isolation for `_G.source`.
- Strong read restrictions comparable to write restrictions for all server-side filesystem access.
- Immediate export-cache invalidation before the relevant resource-stop event is processed.

## Relationship to other spec files

- Runtime bootstrap, standard-library surface, and scheduler details are documented in `specs/lua-runtime-reference.md`.
- Manifest execution semantics are documented in `specs/lua-manifest-reference.md`.
- Resource start/load/stop and dependency behaviour are documented in `specs/lua-resource-management-reference.md`.
