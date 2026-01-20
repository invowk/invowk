# 10-env.tape - Environment configuration tests
# Tests: env files basic, env vars override
#
# Tests the env block feature which provides environment configuration
# through files (.env) and inline vars.

Output vhs/output/10-env.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: env files basic - load environment from .env file
Type "./bin/invowk cmd 'env files basic'"
Enter
Sleep 500ms

# Test 2: env vars override - inline vars override file values
Type "./bin/invowk cmd 'env vars override'"
Enter
Sleep 500ms
