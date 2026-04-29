# FiveM Lua resource management reference

## Scope

This document describes the resource-management path that governs Lua resources in FiveM.

It covers discovery, mounting, metadata loading, script selection, runtime creation, lifecycle events, dependencies, provides, exports, client download flow, and teardown behaviour.

Primary implementation files:

- `code/components/citizen-resources-core/include/Resource.h`
- `code/components/citizen-resources-core/src/Resource.cpp`
- `code/components/citizen-resources-core/include/ResourceManager.h`
- `code/components/citizen-resources-core/src/ResourceManager.cpp`
- `code/components/citizen-resources-core/src/ResourceMetaDataComponent.cpp`
- `code/components/citizen-resources-core/src/ResourceEventComponent.cpp`
- `code/components/citizen-resources-core/src/ResourceDependencyLoader.cpp`
- `code/components/citizen-scripting-core/src/ResourceScriptingComponent.cpp`
- `code/components/citizen-scripting-core/src/ScriptHost.cpp`
- `code/components/citizen-server-impl/src/ServerResourceList.cpp`
- `code/components/citizen-server-impl/src/ServerResourceMounter.cpp`
- `code/components/citizen-server-impl/src/ServerResources.cpp`
- `code/components/citizen-legacy-net-resources/src/ResourceNetBindings.cpp`
- `code/components/citizen-resources-client/src/CachedResourceMounter.cpp`

## Core entities

### Resource states

The core resource state enum includes:

- `Uninitialized`
- `Stopped`
- `Starting`
- `Started`
- `Stopping`

The traced `ResourceImpl` lifecycle code actively sets:

- `Uninitialized`
- `Stopped`
- `Starting`
- `Started`

`Stopping` exists in the enum and in Lua-visible state mapping, but the traced implementation path does not actively set it during `Stop()`.

### Lifecycle hooks

The core resource contract includes hooks such as:

- `OnBeforeStart`
- `OnStart`
- `OnStop`
- `OnCreate`
- `OnActivate`
- `OnDeactivate`
- `OnBeforeLoadScript`
- `OnRemove`

Lua resource behaviour is built by wiring scripting and event components into those hooks.

## Server-side discovery and creation

### Scan roots

Server discovery scans:

- `<root>/resources/`
- `citizen/system_resources/`

### Folder rules

The server scanner:

- recurses into `[category]` folders
- warns if a category folder also contains a manifest
- skips hidden folders
- skips `txadmin`

### Duplicate and path-change handling

If two resources with the same name are discovered, the scanner emits a duplicate-resource warning.

If an existing resource name is found at a different path, the scanner:

1. unmounts `@name/`
2. stops the old resource
3. removes it from the manager
4. scans the new path as a new resource instance

### Server mounter

`ServerResourceMounter::LoadResource` creates the server-side resource object and calls `LoadFrom(path, &error)`.

Failure classes include:

- missing manifest -> warning path
- other load failures -> error path

Failed loads remove the partially created resource from the manager.

## Manifest loading and metadata storage

`ResourceImpl::LoadFrom` delegates metadata loading to `ResourceMetaDataComponent::LoadMetaData(rootPath)`.

The Lua manifest parser is described in `specs/lua-manifest-reference.md`.

### Observable metadata properties

- metadata records preserve insertion order in the underlying multimap iteration model
- `_extra` metadata keys emitted by the manifest DSL are real metadata entries
- source-location information is preserved for manifest-defined metadata when debug information is available

## Manifest glob expansion

`ResourceMetaDataComponent` expands script and file metadata.

### Expansion rules

- each manifest entry is expanded independently
- wildcard expansion within one entry deduplicates matches and sorts them lexicographically
- duplicate files may still appear again if separate manifest entries expand to the same path
- `@otherResource/...` paths are preserved literally and are not glob-expanded as local paths
- empty values are ignored

### Relevant metadata keys

Relevant keys include:

- `shared_script`
- `client_script`
- `server_script`
- `file`
- `export`
- `server_export`
- `dependency`
- legacy `dependencie`
- `provide`

## Script selection and load order

`ResourceScriptingComponent::CreateEnvironments()` determines which scripts will be loaded into runtimes.

### Side-specific key selection

- client side loads `shared_script` then `client_script`
- server side loads `shared_script` then `server_script`

### Load ordering

For Lua resources, the observable load order is:

1. all expanded `shared_script` entries in metadata order
2. all expanded side-specific entries in metadata order

Within each expanded entry, wildcard matches are sorted lexicographically.

### Runtime routing

Each discovered runtime whose `HandlesFile()` matches a script path receives that script.

For the Lua runtime, any filename containing `.lua` matches.

## Runtime creation for Lua resources

When a resource starts, the scripting component:

1. enumerates `IScriptFileHandlingRuntime` implementations
2. keeps only runtimes needed by the resource's script list
3. creates each runtime with the resource-specific script host
4. loads each matching script into that runtime

The Lua runtime bootstrap sequence itself is documented in `specs/lua-runtime-reference.md`.

## Script host behaviour

`GetScriptHostForResource` provides the host used by scripting runtimes.

The host is responsible for:

- opening host files
- opening system files
- bookmark scheduling
- function-reference routing
- active runtime push/pop

Active runtime push/pop triggers `resource->OnActivate()` and `resource->OnDeactivate()` around host-driven runtime activity.

## Client-side deferred initialisation

On the client side, script-environment creation may be deferred until network initialisation completes.

If the resource stops before that point, the deferred startup is ignored.

This means client resource existence and client script-runtime creation are not always simultaneous.

## Core start path

`ResourceImpl::Start()` performs the canonical start transition.

Observed sequence:

1. If the resource is already started, do nothing.
2. Set state to `Starting`.
3. Invoke `OnBeforeStart`.
4. Invoke `OnStart`.
5. If start did not fail and state was not forcefully changed away, set state to `Started`.
6. On failure, revert to `Stopped`.

## Core stop path

`ResourceImpl::Stop()`:

1. returns immediately if already stopped
2. invokes `OnStop`
3. sets state to `Stopped`

## Core destroy path

`ResourceImpl::Destroy()`:

1. sets state to `Uninitialized`
2. fires `OnRemove`

This is the final removal path used by manager reset and removal operations.

## Resource events

`ResourceEventComponent` publishes built-in lifecycle events.

### Built-in start events

- `onResourceStarting` on `OnBeforeStart`
  - immediate
  - cancellable
  - very early priority
- `onResourceStart` on `OnStart`
  - immediate
- `onClientResourceStart` or `onServerResourceStart` on `OnStart`
  - queued

### Built-in stop events

- `onResourceStop` on `OnStop`
  - immediate
- `onClientResourceStop` or `onServerResourceStop` on `OnStop`
  - queued

### Handler cleanup

Resource event subscriptions are cleared on stop through a final reset hook.

### Immediate versus queued timing

`onResourceStart` and `onResourceStop` happen immediately during lifecycle hooks.

`onClientResourceStart`, `onServerResourceStart`, `onClientResourceStop`, and `onServerResourceStop` are queued and are drained later by the resource-manager tick loop.

This timing difference is observable and affects export cache invalidation and event ordering.

## Dependency model

`ResourceDependencyLoader.cpp` implements dependency handling.

### Supported metadata keys

- `dependency`
- legacy `dependencie`

### Start-time checks

During `OnBeforeStart`:

- dependency strings beginning with `/` are treated as constraints, not resource names
- ordinary dependency names must resolve to resources
- stopped dependencies are auto-started
- if a dependency cannot be started, the dependant start fails

### Constraint evaluation

Constraints are matched through `ResourceManagerConstraintsComponent`.

Server-side registrations include constraints such as:

- `server`
- `onesync`
- `gameBuild`
- `native`

### Dependant retry on dependency start

If a dependant was blocked on a dependency, it is retried when the dependency resource later starts.

### Dependant stop propagation

When a dependency resource stops, dependants are recursively stopped.

The implementation contains an explicit caveat that cyclic dependencies are problematic here.

## `provide` alias model

The resource manager records `provide` aliases.

`GetResource(name, withProvides=true)` resolves in this order:

1. direct resource with that name, if present and not stopped
2. a started provider resource
3. any provider if there is no direct resource at all

This means `provide` is fallback resolution, not unconditional name shadowing.

## Lua-visible metadata and resource queries

Lua runtime code can query resource metadata through natives exposed by `MetadataScriptFunctions.cpp` and `ResourceScriptFunctions.cpp`.

Important query surfaces include:

- `GET_NUM_RESOURCE_METADATA`
- `GET_RESOURCE_METADATA`
- `LOAD_RESOURCE_FILE`
- `GET_RESOURCE_PATH` on server
- `GET_CURRENT_RESOURCE_NAME`
- `GET_INVOKING_RESOURCE`
- `GET_RESOURCE_STATE`

`GET_RESOURCE_METADATA` returns `nullptr` for missing resources or out-of-range indices.

## Exports

Lua exports are implemented by the scheduler rather than by direct manager table sharing.

### Metadata-driven exports

- client uses metadata key `export`
- server uses metadata key `server_export`

At startup, the resource registers event-backed export providers for each metadata-defined export name.

### Resolution and caching

`exports[resource][name](...)` lazily resolves the export callback and caches it.

If the export is missing, the caller receives an error such as `No such export ...`.

The cache is cleared when the queued resource-stop event for that resource is processed.

### Manual export registration

The scheduler also supports manual export registration through the callable `exports(...)` helper.

## Server control operations

`ServerResources.cpp` wires the server control path.

### Commands and natives

- `start`
- `stop`
- `restart`
- `ensure`
- `refresh`
- `START_RESOURCE`
- `STOP_RESOURCE`
- `SCHEDULE_RESOURCE_TICK`

### Notable behaviour

- `restart` only applies to resources that are currently started
- `STOP_RESOURCE` refuses to stop certain managed resources
- `STOP_RESOURCE` refuses to stop the calling resource itself
- `ensure` restarts an already started resource only after initial configuration completes

### Refresh behaviour

`refresh` rescans the filesystem, remounts `@resource/` devices for discovered resources, and triggers `onResourceListRefresh` on the server side.

The server preserves `g_resourceStartOrder`, and client configuration generation emits resources in that order first.

## Client download and mount path

Client resource updates are handled through `ResourceNetBindings.cpp` and `CachedResourceMounter.cpp`.

### Content-manifest path

The client receives a content manifest, builds `global://<resource>` URIs, prepares cached file entries, downloads required resources, and starts them later on the game thread.

### Cached mount path

`CachedResourceMounter::LoadResourceWithError`:

1. creates the resource object
2. mounts `cache:/<name>/resource.rpf` at `resources:/<name>/`
3. calls `resource->LoadFrom(...)`
4. unmounts the resource when `OnRemove` fires

### Server-to-client targeted control

- resource start packets trigger update or download, then start
- resource stop packets stop known resources directly

## Resource package contents sent to clients

Server-side packaging includes:

- the manifest file
- `file` entries
- `client_script` entries
- `shared_script` entries

`server_script` entries are not included in the client package.

## Teardown behaviour for Lua resources

When a Lua resource stops:

1. resource stop hooks fire
2. built-in resource stop events are emitted
3. scripting runtimes are destroyed
4. event and runtime registries are cleared
5. the Lua state is closed by the runtime

This fully tears down resource-global Lua state.

## Observable edge cases

- refreshing an existing resource reloads metadata in place; the traced refresh path may clear old metadata before a failing reload is fully surfaced through the same new-resource error path
- wildcard deduplication is per manifest entry, not cross-entry
- queued resource-stop events mean export caches are invalidated after stop emission reaches that queue-processing point, not strictly before every caller observation
- `OnBeforeLoadScript` runs for both the referenced resource and the current resource when loading `@otherResource/...` host files

## Guarantees

- Resource scripts are selected from metadata, expanded, and loaded in a deterministic order derived from metadata order plus per-entry lexicographic wildcard ordering.
- Lua runtime creation is per resource side.
- Dependencies can block resource start and can auto-start stopped dependencies.
- Stopping a dependency stops its dependants.
- Built-in start and stop events are always wired by the resource event component.
- Lua state teardown occurs when the runtime is destroyed.

## Not guaranteed

- Cross-entry deduplication of script paths.
- Use of `ResourceState::Stopping` in the traced runtime path.
- Automatic recovery from cyclic dependency stop chains.
- Instantaneous export-cache invalidation before queued stop events are processed.

## Relationship to other spec files

- Manifest DSL and metadata loading are documented in `specs/lua-manifest-reference.md`.
- Lua runtime bootstrap and standard-library behaviour are documented in `specs/lua-runtime-reference.md`.
- Isolation boundaries between resources and runtimes are documented in `specs/lua-environment-isolation-reference.md`.
