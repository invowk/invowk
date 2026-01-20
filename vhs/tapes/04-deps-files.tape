# 04-deps-files.tape - Filepath dependency tests
# Tests: deps file single, deps file alternatives, deps file permissions
#
# Tests the filepath dependency checking feature.
# Filepath dependencies validate that required files/directories exist.

Output vhs/output/04-deps-files.txt

Set Shell "bash"
Set FontSize 14
Set Width 120
Set Height 30
Set TypingSpeed 0ms

# Test 1: deps file single - requires README.md to exist
Type "./bin/invowk cmd 'deps file single'"
Enter
Sleep 500ms

# Test 2: deps file alternatives - requires any README variant
Type "./bin/invowk cmd 'deps file alternatives'"
Enter
Sleep 500ms

# Test 3: deps file permissions - requires specific file permissions
Type "./bin/invowk cmd 'deps file permissions'"
Enter
Sleep 500ms
