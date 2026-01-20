# 09-args.tape - Positional argument tests
# Tests: args simple, args optional, args typed, args validated
#
# Tests the positional arguments feature including:
# - Required arguments
# - Optional arguments with defaults
# - Typed arguments (int, float)
# - Regex-validated arguments

Output vhs/output/09-args.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: args simple - single required argument
Type "./bin/invowk cmd 'args simple' Alice"
Enter
Sleep 500ms

# Test 2: args optional - required + optional arguments
Type "./bin/invowk cmd 'args optional' Bob"
Enter
Sleep 500ms

# Test 3: args typed - typed arguments (int, float)
Type "./bin/invowk cmd 'args typed' 800 600 1.5"
Enter
Sleep 500ms

# Test 4: args validated - regex-validated arguments
Type "./bin/invowk cmd 'args validated' staging 2.1.0"
Enter
Sleep 500ms
