// Sample invowk pack metadata - demonstrates the invkpack.cue format.
// This file contains pack identity and dependency declarations.
// Command definitions are in invkfile.cue (separate file).

pack: "io.invowk.sample"
version: "1.0"
description: "Sample invowk pack with a simple cross-platform greeting command"

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invkpack.git"
//         version: "^1.0.0"
//     },
// ]
