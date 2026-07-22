#include <stdio.h>
#include <string.h>

extern int is_palindrome(const char *s);

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
    printf("[Palindrome]\n");
    TEST("is_palindrome('')", is_palindrome("") == 1);
    TEST("is_palindrome('a')", is_palindrome("a") == 1);
    TEST("is_palindrome('racecar')", is_palindrome("racecar") == 1);
    TEST("is_palindrome('hello')", is_palindrome("hello") == 0);
    TEST("is_palindrome('amanaplanacanalpanama')", is_palindrome("amanaplanacanalpanama") == 1);

    printf("\n%d tests, %d passed, %d failed\n", tests_run, tests_run - tests_failed, tests_failed);
    return tests_failed > 0 ? 1 : 0;
}
