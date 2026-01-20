# 05-deps-caps.tape - Capability dependency tests
# Tests: deps cap single, deps cap alternatives
#
# Tests the system capability checking feature.
# Capabilities are system features like internet connectivity, TTY, etc.

Output vhs/output/05-deps-caps.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: deps cap single - requires internet connectivity
# Note: This may fail in offline environments
Type "./bin/invowk cmd 'deps cap single' || echo 'EXPECTED: may fail without internet'"
Enter
Sleep 500ms

# Test 2: deps cap alternatives - requires any network connectivity (LAN or internet)
Type "./bin/invowk cmd 'deps cap alternatives' || echo 'EXPECTED: may fail without network'"
Enter
Sleep 500ms
