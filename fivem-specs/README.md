# FiveM custom Lua specification set

This directory contains reference specifications for FiveM's custom Lua implementation.

These files are intended to describe the implementation precisely enough to support tooling such as language servers.

The implementation described here is not stock upstream Lua. It is FiveM's patched Lua 5.4-based runtime together with its manifest loader, resource lifecycle wiring, native-binding code generation, and runtime-isolation model.

## Files

- `lua-runtime-reference.md`
  - runtime bootstrap
  - opened libraries
  - injected globals
  - scheduler model
  - custom `io` and `os` behaviour on server
- `lua-native-binding-reference.md`
  - native-build selection
  - lazy native global materialisation
  - argument marshaling
  - result coercion
- `lua-manifest-reference.md`
  - `fxmanifest.lua` and `__resource.lua`
  - manifest loader state
  - directive interception model
  - metadata emission and source locations
- `lua-resource-management-reference.md`
  - resource discovery
  - metadata expansion
  - runtime attachment
  - lifecycle events
  - dependencies, provides, exports, client download flow
- `lua-function-reference-reference.md`
  - callable proxy tables
  - function-reference serialisation
  - async return adaptation
  - events, exports, NUI, and state-bag callback transport
- `lua-environment-isolation-reference.md`
  - resource-level runtime boundaries
  - client/server separation
  - cross-resource bridges
  - cleanup guarantees and non-guarantees

## Source basis

The reference set is derived from the implementation in:

- `code/components/citizen-scripting-lua/`
- `code/components/citizen-scripting-core/`
- `code/components/citizen-resources-core/`
- `code/components/citizen-resources-metadata-lua/`
- `code/components/citizen-server-impl/`
- `code/components/citizen-legacy-net-resources/`
- `code/vendor/lua.lua`
- `data/shared/citizen/scripting/`
- `ext/natives/`

## Documentation type

All files in this directory are Reference documentation.

They describe behaviour and structure. They do not provide tutorials or how-to workflows.
