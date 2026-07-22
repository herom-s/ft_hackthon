#include <stdio.h>

extern int factorial(int n);

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
    printf("[Factorial]\n");
    TEST("factorial(0)", factorial(0) == 1);
    TEST("factorial(1)", factorial(1) == 1);
    TEST("factorial(5)", factorial(5) == 120);
    TEST("factorial(10)", factorial(10) == 3628800);
    TEST("factorial(12)", factorial(12) == 479001600);

    printf("\n%d tests, %d passed, %d failed\n", tests_run, tests_run - tests_failed, tests_failed);
    return tests_failed > 0 ? 1 : 0;
}
