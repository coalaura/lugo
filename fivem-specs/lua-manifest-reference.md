# FiveM Lua manifest reference

## Scope

This document describes the Lua-based resource-manifest implementation used for:

- `fxmanifest.lua`
- `__resource.lua`

It covers the loader environment, directive interception model, metadata emission semantics, source-location tracking, and manifest-version effects that influence Lua runtime behaviour.

Primary implementation files:

- `code/components/citizen-resources-metadata-lua/src/LuaMetaDataLoader.cpp`
- `data/shared/citizen/scripting/resource_init.lua`
- `code/components/citizen-resources-core/src/ResourceMetaDataComponent.cpp`

## Manifest files and probe order

The Lua metadata loader probes manifest files in this order:

1. `fxmanifest.lua`
2. `__resource.lua`

This behaviour is implemented by `LuaMetaDataLoader::LoadMetaData()`.

If `fxmanifest.lua` is found and executed successfully, the resource is marked as CfxV2 by adding metadata key `is_cfxv2 = true`.

If no manifest file can be opened, the loader reports:

`Could not open resource metadata file - no such file.`

## Loader state model

Manifest parsing uses a dedicated temporary Lua state.

This state is separate from the runtime state later used for resource scripts.

The manifest loader opens only a restricted library set:

- `_G`
- `table`
- `string`
- `math`
- `debug`
- `coroutine`
- `utf8`
- `json`

The loader does not open the runtime `Citizen` table.

The loader does not open the server runtime `io` or `os` libraries.

## Loader bootstrap sequence

The authoritative bootstrap sequence is:

1. Create a fresh Lua state.
2. Open the restricted library set.
3. Register global `AddMetaData` as a C closure bound to the current loader instance.
4. Execute `citizen:/scripting/resource_init.lua` and keep its returned function.
5. Remove:
   - `dofile`
   - `load`
   - `loadfile`
6. Attempt to load a manifest file.
7. If the manifest file compiles successfully, call the function returned by `resource_init.lua` and pass the compiled manifest chunk as its single argument.
8. If the manifest file was `fxmanifest.lua`, append metadata `is_cfxv2 = true`.
9. Destroy the temporary Lua state.

## Directive interception model

`resource_init.lua` implements the manifest DSL.

It returns a wrapper function that accepts the compiled manifest chunk.

Before the manifest chunk is executed, the wrapper temporarily assigns a metatable to `_G`.

That metatable defines `__index`, so any unknown global identifier resolves to a generated function that forwards manifest directives into `AddMetaData`.

This means manifest files are executed as Lua code, but unknown top-level names are interpreted as manifest directives instead of causing unresolved-global failures.

After the manifest chunk finishes, the wrapper removes the `_G` metatable again by assigning `nil`.

## Directive value semantics

### Scalar values

If a directive is called with a scalar argument, the wrapper emits one metadata entry:

- key = directive name
- value = scalar argument coerced to string by the host-side metadata path

### Table values

If a directive is called with a table argument, the wrapper performs plural normalisation:

- if the directive name ends with `s`, the trailing `s` is removed
- one metadata entry is emitted for each array element in the table

Example normalisation:

- `client_scripts` -> `client_script`
- `server_scripts` -> `server_script`
- `files` -> `file`

The wrapper treats the table as an ordered array for emission.

### Chained extra metadata

The manifest wrapper returns a compatibility closure after each directive call.

That closure accepts:

1. a metadata key suffix source
2. an arbitrary Lua value

It emits a second metadata record whose key is `<directive>_extra` and whose value is the JSON encoding of the supplied extra value.

## `AddMetaData` semantics

`AddMetaData` is implemented in `LuaMetaDataLoader.cpp` as a host closure.

### Reserved key protection

User code may not add metadata key `is_cfxv2` directly.

The loader rejects that key with a case-insensitive comparison.

Only the loader itself may append `is_cfxv2` after successful `fxmanifest.lua` loading.

### Source-location capture

When metadata is added, the loader attempts to record source-location information.

It uses the Lua debug stack and `lua_getinfo` to capture:

- `location.file`
- `location.line`

If the source name begins with `@`, the stored file path is normalised to:

`<resource path>/<manifest-relative filename>`

The stored line number is the current Lua source line for the directive call site.

This makes manifest metadata location-aware rather than value-only.

## Compile-time and execution failures

The loader distinguishes parse failures from runtime failures.

### Parse failure

If a manifest file cannot be compiled, the loader reports a message of the form:

`Could not parse resource metadata file ...`

### Runtime failure

If the compiled manifest chunk or wrapper execution fails, the loader reports a message of the form:

`Could not execute resource metadata file ...`

The loader stops immediately on these errors.

## Metadata expansion after loading

The manifest loader is responsible for producing metadata records.

Expansion of those records into concrete script/file lists is handled later by resource metadata consumers such as `ResourceMetaDataComponent`.

Relevant keys include:

- `shared_script`
- `client_script`
- `server_script`
- `file`
- `export`
- `server_export`

The metadata component expands glob patterns, supports recursive `**` matching, and preserves `@resource/...` references as cross-resource paths rather than local filesystem globs.

## Manifest version effects on Lua runtime behaviour

Manifest metadata affects later runtime selection.

The Lua runtime consults manifest-version state to choose the native-wrapper build.

For GTA Five resources, the runtime can select among:

- `natives_21e43a33.lua`
- `natives_0193d0af.lua`
- `natives_universal.lua`

For other targets, the runtime uses:

- `rdr3_universal.lua`
- `ny_universal.lua`
- `natives_server.lua`

If the manifest V2 `fx_version` is at least `adamant`, the runtime selects the universal build for the target platform.

The metadata key `use_experimental_fxv2_oal` also influences later Lua native binding behaviour by enabling the direct OAL-style native path for supported resources.

## Guarantees

- Manifest parsing uses a dedicated temporary Lua state.
- Manifest files do not execute inside the later script runtime state.
- Unknown global names in the manifest are treated as metadata directives through `_G.__index` interception.
- `dofile`, `load`, and `loadfile` are removed from the manifest state before user manifest execution.
- `is_cfxv2` is reserved for loader-controlled insertion.
- Metadata records carry source-location information when debug source information is available.

## Not guaranteed

- Manifest files are not treated as declarative data only; they are still executable Lua chunks running inside the restricted manifest state.
- The manifest wrapper does not provide file-level isolation inside one manifest execution.
- Directive arguments are not type-restricted to strings; table and extra-value paths are interpreted by the wrapper and host encoder.

## Relationship to other spec files

- Runtime bootstrap and script-execution semantics are documented in `specs/lua-runtime-reference.md`.
- Resource lifecycle semantics are documented in `specs/lua-resource-management-reference.md`.
- Runtime environment boundaries and cross-resource interaction rules are documented in `specs/lua-environment-isolation-reference.md`.
