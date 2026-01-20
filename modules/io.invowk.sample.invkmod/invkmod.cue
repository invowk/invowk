// Sample invowk module metadata - demonstrates the invkmod.cue format.
// This file contains module identity and dependency declarations.
// Command definitions are in invkfile.cue (separate file).

module: "io.invowk.sample"
version: "1.0"
description: "Sample invowk module with a simple cross-platform greeting command"

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invkmod.git"
//         version: "^1.0.0"
//     },
// ]
