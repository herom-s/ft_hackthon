#include <stdio.h>

extern int is_prime(int n);

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
    printf("[Prime]\n");
    TEST("is_prime(2)", is_prime(2) == 1);
    TEST("is_prime(3)", is_prime(3) == 1);
    TEST("is_prime(4)", is_prime(4) == 0);
    TEST("is_prime(17)", is_prime(17) == 1);
    TEST("is_prime(1)", is_prime(1) == 0);
    TEST("is_prime(97)", is_prime(97) == 1);
    TEST("is_prime(100)", is_prime(100) == 0);

    printf("\n%d tests, %d passed, %d failed\n", tests_run, tests_run - tests_failed, tests_failed);
    return tests_failed > 0 ? 1 : 0;
}
