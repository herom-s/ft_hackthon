#include <stdio.h>

extern int fibonacci(int n);

static int tests_run = 0;
static int tests_failed = 0;

#define TEST(name, expr) do { \
    tests_run++; \
    if (!(expr)) { \
        printf("  \u2717 %s\n", name); \
        tests_failed++; \
    } else { \
        printf("  \u2713 %s\n", name); \
    } \
} while(0)

int main(void) {
    printf("[Fibonacci]\n");
    TEST("fibonacci(0)", fibonacci(0) == 0);
    TEST("fibonacci(1)", fibonacci(1) == 1);
    TEST("fibonacci(10)", fibonacci(10) == 55);
    TEST("fibonacci(20)", fibonacci(20) == 6765);
    TEST("fibonacci(30)", fibonacci(30) == 832040);

    printf("\n%d tests, %d passed, %d failed\n", tests_run, tests_run - tests_failed, tests_failed);
    return tests_failed > 0 ? 1 : 0;
}
