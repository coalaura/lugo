# Lugo

A ridiculously fast, zero-allocation Lua 5.4 parser and Language Server (LSP) written in Go. 

Lugo is built from the ground up for maximum performance. By iterating over source code using a flat-array/arena architecture (`[]Node`) and storing only byte offsets, it heavily eliminates pointer allocations, heap strings and garbage collection pressure.

## Features

* **Zero-Allocation Architecture:** A fully Lua 5.4 compliant Pratt parser that builds a flat AST without allocating strings or pointers on the heap.
* **Deep Module Aliasing & Resolution:** If a local table is exported (`MySQL = this`), Lugo dynamically resolves all deeply nested fields (e.g., `MySQL.Await.Execute`) across the workspace in real-time.
* **Microscopic Memory Footprint:** Only actively open files are kept in memory during re-indexing. Easily handles 2000+ file workspaces without breaking a sweat.
* **AST-Depth Collision Heuristics:** Strictly protects true root-level definitions from being hijacked by deep runtime overrides.
* **Resilient Autocomplete:** Uses fast backward byte-scanning to provide context-aware fields, locals and globals even when the surrounding syntax tree is broken.
* **Virtual Standard Library:** Click on any standard library function to open a syntax-highlighted, read-only virtual tab streaming directly from the Go server's embedded filesystem (`std:///`).
* **Smart Ignores:** Automatically inherits VS Code's native `files.exclude` and `search.exclude` settings to instantly skip parsing ignored directories (like `node_modules` or `.git`).

## Language Server Capabilities

* Workspace-wide Diagnostics (Undefined globals, with wildcard support)
* Go to Definition & Hover (Supports LuaDoc `@param`, `@return`, `@field`)
* Intelligent Autocomplete (Context-aware member access)
* Workspace Symbol Renaming
* Find References
* Document Outline / Symbols

## Installation

The VS Code extension automatically detects your OS and architecture and runs the correct bundled Go binary. No external dependencies or runtimes are required.

Open the command palette and run `Lugo: Re-index Workspace` if you ever need to manually refresh the index.

## Configuration

You can configure Lugo via your VS Code `settings.json`:

* `lugo.libraryPaths`: An array of absolute paths to external Lua libraries you want to index alongside your workspace.
* `lugo.knownGlobals`: Global variables to ignore when reporting undefined globals. Supports wildcards (e.g., `N_0x*`).
* `lugo.ignoreGlobs`: Additional glob patterns to ignore during indexing (merges with VS Code's native excludes).