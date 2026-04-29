# FiveM Lua native binding reference

## Scope

This document describes the custom Lua native-binding layer used by FiveM.

It covers native-build selection, eager versus lazy wrapper loading, global native resolution, argument marshaling, result coercion, and the additional runtime helpers exposed for native calls.

Primary implementation files:

- `code/components/citizen-scripting-lua/src/LuaScriptNatives.cpp`
- `code/components/citizen-scripting-lua/src/LuaScriptRuntime.cpp`
- `code/components/citizen-scripting-lua/src/LuaNativesLoader.cpp`
- `data/shared/citizen/scripting/lua/natives_loader.lua`
- `ext/natives/README.md`
- `ext/natives/natives_stash/`

## Native-binding architecture

The FiveM Lua runtime does not ship one static table containing all native wrappers eagerly defined in `_G`.

Instead, native bindings are selected by runtime build and are then exposed either:

- by directly loading a generated Lua wrapper file
- or by lazily materialising wrappers and closures on first global lookup

The native bridge therefore consists of:

1. build selection in the runtime
2. optional archive mounting of generated wrappers
3. a Lua `_G` metatable-based lazy resolver
4. a C++ marshaling layer that converts Lua values to `ScriptNativeContext`

## Build selection

`LuaScriptRuntime::Create()` chooses a native-wrapper build according to target platform and manifest metadata.

### GTA Five builds

The GTA Five runtime can select:

- `natives_21e43a33.lua`
- `natives_0193d0af.lua`
- `natives_universal.lua`

### Other targets

Other targets use:

- `rdr3_universal.lua`
- `ny_universal.lua`
- `natives_server.lua`

### Manifest-version effect

If manifest V2 `fx_version` is at least `adamant`, the runtime selects the universal build for the current target.

## Wrapper source origin

Generated wrapper definitions originate from the native-declaration pipeline under `ext/natives/`.

Representative declaration stashes include:

- `ext/natives/natives_stash/gta_universal.lua`
- `ext/natives/natives_stash/gta_21E43A33.lua`
- `ext/natives/natives_stash/gta_0193D0AF.lua`

These are inputs to the generated wrapper assets consumed by the Lua runtime.

## Eager versus lazy loading

### Direct system-file mode

If archive-backed native mounts are unavailable, `LoadNativesBuild` loads the generated native wrapper file directly from:

- `citizen:/scripting/lua/<build>`

### Lazy archive-backed mode

If native archives are mounted:

1. `m_nativesDir` is set to `nativesLua:/<build-without-.lua>/`
2. `citizen:/scripting/lua/natives_loader.lua` is loaded

In this mode, wrappers are resolved lazily on first use.

## Native archive mounting

`LuaNativesLoader.cpp` mounts in-memory zip devices containing generated Lua native wrappers.

Mounted archive namespaces include:

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

A marker device at `nativesLua:/marker/` prevents duplicate mounting.

## Global native resolution

`natives_loader.lua` installs a metatable on the real runtime `_G` table.

### Unknown global lookup

When an unknown global name is read:

1. check whether the name is already memoised as missing in `nilCache`
2. call `Citizen.LoadNative(name)`
3. if the result is a function, store that function directly in `_G[name]`
4. if the result is a string, compile and execute it inside `nativeEnv`
5. read the created binding back from `_G`

Missing native names are cached in `nilCache` to avoid repeated host lookups.

### Source naming for generated wrappers

Generated wrapper source returned as text is compiled with source name:

`@<nativeName>.lua`

This affects debug source information and stack traces.

## `nativeEnv`

Generated Lua wrapper source is executed in a dedicated environment rather than directly in raw `_G`.

That environment exposes helper bindings including:

- `Global`
- `_mfr`
- `_obj`
- `_ch`
- `msgpack`
- result and pointer helper bindings
- `Citizen.InvokeNative`
- `Citizen.InvokeNative2` when available
- `Citizen.GetNative` when available

`Global` mirrors assignments into the actual runtime global table.

This is how generated wrappers publish their public symbols back into `_G`.

## `Citizen` native-facing helpers

The runtime exposes the following native-facing helpers through `Citizen`:

- `InvokeNative`
- `LoadNative`
- `GetNative` on client builds
- `InvokeNative2` on client builds
- pointer helpers such as `PointerValueInt`, `PointerValueFloat`, `PointerValueVector`
- result helpers such as `ResultAsInteger`, `ResultAsLong`, `ResultAsFloat`, `ResultAsString`, `ResultAsVector`, `ResultAsObject`, `ResultAsObject2`, `ReturnResultAnyway`

These are part of the native-binding contract rather than stock Lua.

## `Citizen.LoadNative`

`Citizen.LoadNative` is implemented by `Lua_LoadNative`.

### Lookup path

The binding layer maps a native name to runtime-native metadata and then chooses one of two strategies:

- return a direct callable closure
- return generated Lua source for a wrapper

### OAL path

If the resource is using the supported CfxV2 OAL path, `Lua_LoadNative` can return a direct native function binding instead of generated wrapper source.

The manifest-controlled switch involved is `use_experimental_fxv2_oal`.

### Missing-native tracking

Missing names are recorded in the runtime’s `m_nonExistentNatives` set.

This cache is runtime-local and is cleared when the runtime is destroyed.

## `Citizen.GetNative`

On client builds, `Citizen.GetNative` returns a direct callable binding for a native hash or native identity handled by the runtime.

This surface is available only where the build exports it.

## `Citizen.InvokeNative`

`Citizen.InvokeNative` is implemented by `Lua_InvokeNative` and delegates to the core marshaling path.

The first Lua argument is treated as the native hash or native identifier.

The remaining arguments are marshaled into `ScriptNativeContext`.

Exceptions raised by the host-side invocation path are turned into Lua errors.

## Argument marshaling

The runtime marshals Lua values into `ScriptNativeContext` using custom bridge logic.

Supported argument shapes include:

- `nil` -> numeric zero payload
- `false` -> numeric zero payload
- `true` -> numeric one payload
- integer numbers
- floating-point numbers
- strings
- `vector2`
- `vector3`
- `vector4`
- quaternions
- tables exposing a `__data` field
- tables whose metatable exposes `__data`
- lightuserdata used as pointer or meta sentinels

Unsupported shapes raise script errors such as:

- `invalid lua type`
- `invalid lua type in __data`

## Table `__data` path

The marshaler recognises native-argument carrier tables by reading:

- field `__data`
- or metatable-provided `__data`

This allows generated helpers and wrapper objects to participate in native calls without being plain primitive values.

## Result coercion

The bridge converts host results back into Lua values.

Observed supported result shapes include:

- boolean
- 32-bit integer
- 64-bit integer
- float
- `ScrVector` mapped to GLM `vec3`
- `const char*`
- `ScrString`
- `ScrObject` through the runtime object-result callback

`ResultAsObject2` installs the deserialisation routine used for object results.

If no object-result callback is installed, object-like results collapse to `nil`.

## Result forcing helpers

The Lua-facing result helpers tell the generated wrapper or native call path how to interpret the returned host value.

Observable helper names include:

- `ResultAsInteger`
- `ResultAsLong`
- `ResultAsFloat`
- `ResultAsString`
- `ResultAsVector`
- `ResultAsObject`
- `ResultAsObject2`
- `ReturnResultAnyway`

These helpers are part of the FiveM Lua ABI for generated wrappers.

## Pointer helpers

Pointer helpers are exposed so wrappers can request output values and structured native return channels.

Observable helper names include:

- `PointerValueIntInitialized`
- `PointerValueFloatInitialized`
- `PointerValueInt`
- `PointerValueFloat`
- `PointerValueVector`

These helpers are not stock Lua concepts; they are FiveM-specific native-bridge primitives.

## Client/server differences

### Shared behaviour

Both sides use:

- the `Citizen` native bridge
- generated wrapper assets
- the same lazy-global concept when lazy loader mode is active

### Client-specific additions

Client builds additionally expose:

- `Citizen.GetNative`
- `Citizen.InvokeNative2`

### Server-specific build choice

Server uses `natives_server.lua` for its generated wrapper build.

## Guarantees

- Native globals can be materialised lazily.
- Missing native names are cached per runtime.
- Wrapper build selection depends on target platform and manifest version.
- Native invocation uses a custom marshaling layer rather than stock Lua FFI.
- Pointer and result helpers are part of the supported wrapper ABI.

## Not guaranteed

- Eager predefinition of every native global in `_G`.
- Identical wrapper-build selection across all manifest versions.
- Object result availability without an installed object-result callback.
- Native-name lookup success for every identifier; failed lookups are cached as missing.

## Relationship to other spec files

- Runtime bootstrap and standard-library surface are documented in `specs/lua-runtime-reference.md`.
- Manifest-version and metadata inputs are documented in `specs/lua-manifest-reference.md`.
- Resource lifecycle and runtime ownership are documented in `specs/lua-resource-management-reference.md`.
