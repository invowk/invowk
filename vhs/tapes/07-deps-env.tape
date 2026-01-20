# 07-deps-env.tape - Environment variable dependency tests
# Tests: deps env single, deps env multiple
#
# Tests the env_vars dependency type which validates that required
# environment variables exist in the user's environment.

Output vhs/output/07-deps-env.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: deps env single - requires HOME environment variable
Type "./bin/invowk cmd 'deps env single'"
Enter
Sleep 500ms

# Test 2: deps env multiple - requires multiple environment variables
Type "./bin/invowk cmd 'deps env multiple'"
Enter
Sleep 500ms
