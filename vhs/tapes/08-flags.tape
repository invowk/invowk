# 08-flags.tape - Command flag tests
# Tests: flags simple, flags defaults, flags typed, flags short, flags validation
#
# Tests the command-line flags feature including:
# - Simple flags with no defaults
# - Flags with default values
# - Typed flags (bool, int, float, string)
# - Short flag aliases
# - Regex-validated flags

Output vhs/output/08-flags.txt

Set Shell "bash"
Set FontSize 14
Set Width 120
Set Height 30
Set TypingSpeed 0ms

# Test 1: flags simple - basic flags without defaults
Type "./bin/invowk cmd 'flags simple'"
Enter
Sleep 500ms

# Test 2: flags defaults - flags with default values
Type "./bin/invowk cmd 'flags defaults'"
Enter
Sleep 500ms

# Test 3: flags typed - typed flags (bool, int, float, string)
Type "./bin/invowk cmd 'flags typed'"
Enter
Sleep 500ms

# Test 4: flags short - flags with short aliases
Type "./bin/invowk cmd 'flags short' -v"
Enter
Sleep 500ms

# Test 5: flags validation - regex-validated flags
Type "./bin/invowk cmd 'flags validation' --env=staging"
Enter
Sleep 500ms
