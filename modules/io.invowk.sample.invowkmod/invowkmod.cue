// Sample invowk module metadata - demonstrates the invowkmod.cue format.
// This file contains module identity and dependency declarations.
// Command definitions are in invowkfile.cue (separate file).

module: "io.invowk.sample"
version: "1.0.0"
description: "Sample invowk module with a simple cross-platform greeting command"

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invowkmod.git"
//         version: "^1.0.0"
//     },
// ]
