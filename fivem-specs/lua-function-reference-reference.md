# FiveM Lua function-reference and callable-proxy reference

## Scope

This document describes how FiveM transports callable values across Lua bridge boundaries.

It covers the implementation in:

- `data/shared/citizen/scripting/lua/scheduler.lua`
- `code/components/citizen-scripting-core/src/RefScriptFunctions.cpp`
- `code/components/citizen-scripting-core/src/ResourceCallbackComponent.cpp`
- `code/components/citizen-scripting-core/src/EventScriptFunctions.cpp`
- `code/components/nui-resources/src/ResourceUICallbacks.cpp`

It is a Reference document. It describes the runtime representation, serialisation rules, invocation semantics, and bridge-specific uses of function references.

## Core model

FiveM does not transport Lua closures directly across bridge boundaries.

Instead, callable values cross those boundaries as host-managed function references.

At the Lua level, an imported callable is typically **not** of type `function`.

Instead:

- the imported value is a table
- the table carries raw field `__cfx_functionReference`
- the table has a metatable-like extension object that provides `__call`
- the table is therefore callable even though `type(value) == 'table'`

This is the specific behaviour exposed by events, exports, NUI callbacks, state-bag change handlers, and other msgpack-backed callback bridges.

## Local reference allocation

`scheduler.lua` keeps a runtime-local table named `funcRefs` and an incrementing integer `funcRefIdx`.

`MakeFunctionReference(func)`:

1. allocates the current `funcRefIdx`
2. stores `{ func = func, refs = 0 }` in `funcRefs`
3. increments `funcRefIdx`
4. returns `Citizen.CanonicalizeRef(thisIdx)`

Observable properties:

- the storage is local to one Lua runtime state
- the stored value is the original Lua closure
- reference counting starts at `0` and is adjusted through duplicate/delete routines later

## Canonical string form

Host-side lookup in `RefScriptFunctions.cpp` parses references as:

`resourceName:instanceId:refId`

Examples of the structure:

- normal resource runtime references point at a concrete resource and runtime instance
- callback references created by `ResourceCallbackComponent` use `_cfx_internal:0:<refId>`

`ValidateAndLookUpRef(refString, &refIdx)` resolves the string by:

1. splitting on the first two `:` characters
2. resolving the target resource name or the special `_cfx_internal` runtime
3. resolving the runtime instance ID
4. resolving the numeric reference ID
5. converting the runtime to `IScriptRefRuntime`

If any of those steps fails, resolution fails and no callable target is produced.

## Msgpack type mapping

The scheduler installs msgpack extension handling for callable transport.

Defined extension tags:

- `EXT_FUNCREF = 10`
- `EXT_LOCALFUNCREF = 11`

The scheduler clears previous handlers with `msgpack.extend_clear(EXT_FUNCREF, EXT_LOCALFUNCREF)` and then registers pack/unpack logic for both tags.

It also executes:

`msgpack.settype("function", EXT_FUNCREF)`

This means that when msgpack serialises a plain Lua `function`, the value is encoded using the function-reference extension rather than rejected or stringified.

## Outbound conversion rules

### Plain Lua functions

When msgpack packs a plain Lua function:

1. the value is classified as `EXT_FUNCREF`
2. the extension `__pack` handler calls `Citizen.GetFunctionReference(self)`
3. `Citizen.GetFunctionReference(func)` recognises `type(func) == 'function'`
4. it allocates a local function reference through `MakeFunctionReference(func)`
5. the canonical reference string becomes the msgpack payload

The original closure does not cross the boundary. Only the canonical reference string does.

### Existing callable proxy tables

`Citizen.GetFunctionReference(func)` also accepts a table whose raw field `__cfx_functionReference` exists.

In that case it does **not** simply reuse the incoming reference string.

Instead it creates a new local wrapper closure:

```lua
MakeFunctionReference(function(...)
    return func(...)
end)
```

Consequences:

- forwarding an imported callable through another bridge creates a new local reference entry
- the forwarded reference points at a wrapper closure in the forwarding runtime
- the forwarding runtime becomes part of the invocation path

### Unsupported values for funcref packing

If the extension packer receives a value that `Citizen.GetFunctionReference` cannot convert, it raises:

`Unknown funcref type: <tag> <lua-type>`

## Inbound conversion rules

When msgpack unpacks either `EXT_FUNCREF` or `EXT_LOCALFUNCREF`:

1. the raw payload is treated as the canonical reference string
2. `DuplicateFunctionReference(ref)` is called immediately
3. a Lua table `{ __cfx_functionReference = ref }` is created
4. the function-reference metatable/extension object is attached
5. the table is returned to user code

This means imported callables arrive as callable proxy tables rather than as native Lua closures.

## Callable proxy semantics

The imported table uses `funcref_mt` with the following observable behaviour.

### Raw storage

- canonical reference string is stored in raw field `__cfx_functionReference`

### `__call`

Calling the proxy performs these steps:

1. read the canonical ref string from `rawget(t, '__cfx_functionReference')`
2. serialise call arguments with `msgpack_pack_args(...)`
3. invoke `Citizen.InvokeFunctionReference(ref, args)` inside `runWithBoundaryEnd(...)`
4. unpack the returned msgpack blob with `msgpack_unpack(rv)`
5. adapt async sentinel results when possible
6. return the unpacked result tuple with `table_unpack(rvs)`

### `__gc`

Garbage collection triggers:

`DeleteFunctionReference(rawget(t, '__cfx_functionReference'))`

This allows imported references to release their duplicated host/runtime reference when the proxy object is collected.

### `__index`

Any field read other than raw access fails with:

`Cannot index a funcref`

### `__newindex`

Any field write fails with:

`Cannot set indexes on a funcref`

### Type shape

The proxy remains a table for normal Lua type checks.

Tooling should therefore distinguish between:

- native Lua closures: `type(value) == 'function'`
- imported callable proxies: `type(value) == 'table'` plus raw `__cfx_functionReference` plus callable metatable behaviour

## Async return adaptation

`Citizen.SetCallRefRoutine` may return either synchronous values or an async sentinel.

### Synchronous path

The target closure is executed inside `Citizen.CreateThreadNow(...)` and `xpcall(...)`.

If the call completes without waiting:

- successful results become `msgpack_pack(retvals)` where `retvals` is an array of return values
- failure becomes `msgpack_pack(nil)` after the error has been traced or captured

### Waiting path

If the invoked function waits or yields, `Citizen.SetCallRefRoutine` returns:

`msgpack_pack({ { __cfx_async_retval = function(rvcb) ... end } })`

The proxy `__call` method then behaves differently depending on coroutine context.

#### Called from a scheduler coroutine

If `coroutine_running()` is truthy and the first unpacked value contains raw key `__cfx_async_retval`:

1. a promise is created
2. the async callback resolves or rejects that promise
3. `Citizen.Await(p)` is used
4. the awaited results are unpacked and returned

#### Called outside a scheduler coroutine

If no coroutine is running, the proxy does **not** auto-await.

It falls through to the normal return path and returns the unpacked sentinel table as an ordinary Lua result value.

Tooling should therefore not model every callable-proxy invocation as synchronously returning the final callback results.

## Error behaviour

### Invalid incoming ref call

If `Citizen.SetCallRefRoutine` receives an unknown `refId`, it traces:

`Invalid ref call attempt: <refId>`

and returns `msgpack_pack(nil)`.

### Target execution failure

The call-ref routine executes the target inside `xpcall(..., doStackFormat)`.

If execution fails:

- formatted script error text is produced through `doStackFormat`
- synchronous callers receive packed `nil`
- async callers receive completion through the async callback once available

### Proxy call failure on empty return payload

If `Citizen.InvokeFunctionReference` returns no decodable payload, proxy `__call` executes `error()`.

## Reference counting and lifetime

### Runtime-local reference table

Each `funcRefs[refId]` stores:

- `func`: the actual local Lua closure
- `refs`: duplicate count tracked by the scheduler

### Duplicate routine

`Citizen.SetDuplicateRefRoutine(function(refId) ... end)`:

- finds the local entry
- increments `refs`
- returns the same numeric `refId`
- returns `-1` if the entry no longer exists

### Delete routine

`Citizen.SetDeleteRefRoutine(function(refId) ... end)`:

- decrements `refs`
- removes `funcRefs[refId]` once `refs <= 0`

### Host-side duplicate/delete

`DUPLICATE_FUNCTION_REFERENCE` and `DELETE_FUNCTION_REFERENCE` in `RefScriptFunctions.cpp` bridge these operations across canonical string references.

`DELETE_FUNCTION_REFERENCE` may defer actual deletion through a concurrent queue retried on `ResourceManager` tick if immediate deletion throws runtime errors.

## Bridge-specific behaviour

### Events

`TriggerEvent`, `TriggerServerEvent`, `TriggerLatentServerEvent`, `TriggerClientEvent`, and `TriggerLatentClientEvent` all serialise arguments with `msgpack_pack_args(...)` before crossing the host boundary.

Consequences for function arguments:

- a plain Lua function argument is packed as `EXT_FUNCREF`
- the receiver unpacks it as a callable proxy table
- handler code therefore observes a callable table rather than a native closure
- the same conversion rule applies whether the destination is same-side, cross-resource, client-to-server, or server-to-client, because the payload path is msgpack-based

### Exports

Exports are resolved through synthetic events, and provider callbacks are cached by the caller.

When the provider passes `setCB(func)`:

- the callback itself is transported through the function-reference machinery
- the caller caches the imported callable proxy
- later export invocation calls that proxy through its `__call` logic

Forwarding an export callback again creates a fresh local wrapper reference, not a zero-cost alias to the original provider closure.

### NUI callbacks

`ResourceUICallbacks.cpp` creates result channels through `ResourceCallbackComponent::CreateCallback(...)`.

That callback reference is canonicalised as `_cfx_internal:0:<refId>` and passed into Lua either:

- as the legacy `resultCallback` argument to `__cfx_nui:<type>` handlers
- or through direct reference invocation for ref-backed NUI callbacks

The Lua-side callback parameter is therefore another callable proxy backed by the same funcref system.

### State-bag change handlers

Host-side `ADD_STATE_BAG_CHANGE_HANDLER` stores the Lua callback as a function reference.

When the host later emits a change, it calls the stored reference with:

- `bagName`
- `key`
- `value`
- `source`
- `replicated`

The callback transport still follows the same canonical-reference and callable-proxy rules.

### Internal callbacks

`ResourceCallbackComponent` provides a special `_cfx_internal` callback runtime used for internal bridge callbacks such as NUI result channels.

Its runtime:

- stores callbacks in its own `m_refs`
- canonicalises them as `_cfx_internal:0:<refId>`
- unpacks incoming msgpack payloads before invoking the stored C++ callback functor

## Guarantees

The implementation guarantees the following observable properties:

- function-like values can cross msgpack-backed bridges without becoming plain strings or failing serialisation
- imported function-like values are callable tables, not native Lua closures
- imported callable proxies reject normal table indexing and assignment
- imported proxies duplicate and later delete the underlying host/runtime reference
- forwarding a callable proxy creates a new local wrapper reference rather than directly reusing the original target entry
- async call-ref results are auto-awaited only when invoked from a running scheduler coroutine

## Non-guarantees

The implementation does **not** guarantee the following:

- that `type(value)` remains `function` after a callable crosses a bridge
- that a forwarded callable preserves identity with the original source reference
- that proxy invocation outside a scheduler coroutine yields final async results instead of the async sentinel table
- that proxy lifetime is deterministic, because final deletion may depend on Lua GC timing and deferred host cleanup

## Relationship to other specifications

- `specs/lua-runtime-reference.md` describes the wider scheduler and bridge surface.
- `specs/lua-environment-isolation-reference.md` describes the runtime boundary that these references cross.
- `specs/lua-resource-management-reference.md` describes how resources expose exports and lifecycle events.
