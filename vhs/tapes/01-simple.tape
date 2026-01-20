# 01-simple.tape - Basic command execution tests
# Tests: hello, env hierarchy
#
# These are the simplest native runtime commands demonstrating
# basic invowk execution.

Output vhs/output/01-simple.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: hello command - simplest possible invowk command
Type "./bin/invowk cmd hello"
Enter
Sleep 500ms

# Test 2: env hierarchy - demonstrates environment variable precedence
Type "./bin/invowk cmd 'env hierarchy'"
Enter
Sleep 500ms
