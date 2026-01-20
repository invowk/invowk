# 06-deps-custom.tape - Custom check dependency tests
# Tests: deps check exitcode, deps check output
#
# Tests the custom validation script feature.
# Custom checks allow running arbitrary scripts to validate prerequisites.

Output vhs/output/06-deps-custom.txt

Set Shell "bash"
Set FontSize 14
Set Width 120
Set Height 30
Set TypingSpeed 0ms

# Test 1: deps check exitcode - validates exit code of a check script
Type "./bin/invowk cmd 'deps check exitcode'"
Enter
Sleep 500ms

# Test 2: deps check output - validates output pattern of a check script
Type "./bin/invowk cmd 'deps check output'"
Enter
Sleep 500ms
