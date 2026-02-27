# Lugo

A ridiculously fast, zero-allocation Lua 5.4 parser and Language Server (LSP) written in Go. 

Lugo is built from the ground up for maximum performance. By iterating over source code using a flat-array/arena architecture (`[]Node`) and storing only byte offsets, it heavily eliminates pointer allocations, heap strings, and garbage collection pressure.

[**Install from the VS Code Marketplace**](https://marketplace.visualstudio.com/items?itemName=coalaura.lugo-vscode)

## Why Lugo?

Most Lua language servers struggle when dropped into massive codebases (like game server environments or large modding frameworks). They consume gigabytes of RAM, take minutes to index, and lag while typing.

**Lugo is different:**
* **Microscopic Memory Footprint:** Lugo only stores flat arrays of integers. Only actively open files keep their source strings in memory. In real-world benchmarks, Lugo completely indexes a **2,400+ file workspace** (including full AST generation, resolution, and publishing workspace-wide diagnostics) in **~1.5 seconds** while consuming only **~270 MB** of RAM.
* **Zero-Allocation Architecture:** The parser, lexer, and symbol resolver are designed to never allocate heap strings during normal typing and querying.
* **Dynamic by Design:** Instead of forcing strict typing on a dynamic language, Lugo embraces Lua. If you do `MySQL = this` in a local file, Lugo dynamically resolves all deep table fields (e.g., `MySQL.Await.Execute`) across your entire workspace in real-time.
* **Standalone Binary:** No NodeJS, no Java, no Lua runtimes. Just a single, blazingly fast compiled Go binary.

## Features & Capabilities

* **Intelligent Autocomplete:** Resilient, context-aware member access (`table.|`), locals, and globals. Works even when the surrounding syntax tree is temporarily broken.
* **Go to Definition & Hover:** Instant cross-file jumps. Supports LuaDoc (`@param`, `@return`, `@field`) and renders formatted function signatures.
* **Signature Help & Inlay Hints:** Real-time parameter tooltips and inline parameter name hints with smart implicit `self` handling.
* **Workspace Symbol Search:** Instantly search for fully qualified names (e.g., `OP.Math.Round`) across your entire workspace.
* **Advanced Diagnostics:**
  * Workspace-wide undefined globals (with wildcard support, e.g., `N_0x*`).
  * Unused local variable detection (fades out unused code).
  * Unreachable code detection.
  * Shadowing warnings with clickable links jumping to the shadowed definition.
  * Ambiguous return warnings (catching Lua's infamous newline evaluation trap).
* **Virtual Standard Library:** Click on any standard library function to open a syntax-highlighted, read-only virtual tab streaming directly from the Go server's embedded filesystem (`std:///`).
* **Smart Ignores:** Automatically inherits VS Code's native `files.exclude` and `search.exclude` settings to instantly skip parsing ignored directories (like `node_modules` or `.git`).

## Installation

### VS Code
Simply install the extension from the [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=coalaura.lugo-vscode). The extension automatically detects your OS and architecture and runs the correct bundled Go binary. No external dependencies are required.

### Other Editors (Neovim, etc.)
Lugo is entirely editor-agnostic. You can download the standalone LSP binaries for Windows, Linux, and macOS from the [GitHub Releases](https://github.com/coalaura/lugo/releases) page and attach them to your client of choice using standard LSP configurations over `stdio`.

## Configuration

You can configure Lugo via your VS Code `settings.json` (also available via the settings UI under **Extensions -> Lugo LSP**):

**Workspace**
* `lugo.workspace.libraryPaths`: An array of absolute paths to external Lua libraries to index.
* `lugo.workspace.ignoreGlobs`: Additional glob patterns to ignore during indexing.

**Environment**
* `lugo.environment.knownGlobals`: Global variables to ignore when reporting undefined globals. Supports wildcards (e.g., `N_0x*`).

**Diagnostics**
* `lugo.diagnostics.undefinedGlobals`: Toggle undefined global warnings.
* `lugo.diagnostics.unusedVariables`: Toggle unused local variable detection.
* `lugo.diagnostics.shadowing`: Toggle warnings when a local shadows an outer scope or global.
* `lugo.diagnostics.unreachableCode`: Toggle graying out unreachable code.
* `lugo.diagnostics.ambiguousReturns`: Toggle warnings for expressions accidentally returned due to newlines.

**Editor**
* `lugo.inlayHints.parameterNames`: Enable inline parameter name hints for function and method calls.