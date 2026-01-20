# 11-isolation.tape - Environment variable isolation tests
# Tests: isolation virtual
#
# Tests that INVOWK_ARG_* and INVOWK_FLAG_* environment variables
# are isolated between nested command invocations.

Output vhs/output/11-isolation.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: isolation virtual - demonstrates env var isolation in virtual runtime
Type "./bin/invowk cmd 'isolation virtual'"
Enter
Sleep 500ms
