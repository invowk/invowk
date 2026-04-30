// Sample invowk module metadata - demonstrates the invowkmod.cue format.
// This file contains module identity and dependency declarations.
// Command definitions are in invowkfile.cue (separate file).

module: "io.invowk.sample"
version: "1.0.0"
description: "Sample invowk module with a simple cross-platform greeting command"
license: "MPL-2.0"
repository: "https://github.com/invowk/invowk"

// No external dependencies. To add one, declare a requires block:
//   requires: [{git_url: "https://github.com/org/name.invowkmod.git", version: "^1.0.0"}]
// Then run: invowk module sync
