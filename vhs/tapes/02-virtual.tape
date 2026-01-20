# 02-virtual.tape - Virtual shell runtime tests
# Tests: virtual hello, multi runtime
#
# Tests the built-in virtual shell interpreter (mvdan/sh)
# which provides cross-platform shell execution.

Output vhs/output/02-virtual.txt

Set Shell "bash"
Set FontSize 14
Set Width 120
Set Height 30
Set TypingSpeed 0ms

# Test 1: virtual hello - simple command using virtual shell runtime
Type "./bin/invowk cmd 'virtual hello'"
Enter
Sleep 500ms

# Test 2: multi runtime - command that can run in native or virtual
Type "./bin/invowk cmd 'multi runtime'"
Enter
Sleep 500ms
