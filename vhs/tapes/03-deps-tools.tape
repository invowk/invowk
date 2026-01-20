# 03-deps-tools.tape - Tool dependency tests
# Tests: deps tool single, deps tool alternatives, deps tools mixed
#
# Tests the tool/binary dependency checking feature.
# Tool dependencies validate that required binaries exist in PATH.

Output vhs/output/03-deps-tools.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: deps tool single - requires a single tool (sh)
Type "./bin/invowk cmd 'deps tool single'"
Enter
Sleep 500ms

# Test 2: deps tool alternatives - requires any of: podman, docker, nerdctl
# This may fail if no container runtime is installed, which is expected
Type "./bin/invowk cmd 'deps tool alternatives' || echo 'EXPECTED: may fail without container runtime'"
Enter
Sleep 500ms

# Test 3: deps tools mixed - multiple tool dependencies with mixed alternatives
Type "./bin/invowk cmd 'deps tools mixed'"
Enter
Sleep 500ms
