import sys
sys.path.insert(0, '/workspace')

from fibonacci import fibonacci

tests_run = 0
tests_passed = 0

def test(name, expr):
    global tests_run, tests_passed
    tests_run += 1
    if expr:
        tests_passed += 1
        print(f"  \u2713 {name}")
    else:
        print(f"  \u2717 {name}")

print("[Fibonacci (Python)]")
test("fibonacci(0)", fibonacci(0) == 0)
test("fibonacci(1)", fibonacci(1) == 1)
test("fibonacci(10)", fibonacci(10) == 55)
test("fibonacci(20)", fibonacci(20) == 6765)
test("fibonacci(30)", fibonacci(30) == 832040)

print(f"\n{tests_run} tests, {tests_passed} passed, {tests_run - tests_passed} failed")
sys.exit(0 if tests_passed == tests_run else 1)
