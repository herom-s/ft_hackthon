# ft_hackthon User Guide

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd <repo-dir>

# Build and install
make install

# Verify installation
ft_hackthon --version
```

### Building from Source (Manual)

```bash
git clone <repository-url>
cd <repo-dir>
go build -o ft_hackthon ./cmd/ft_hackthon
```

## Getting Started

### 1. Create an Account

If you don't have an account yet:

```bash
ft_hackthon register
```

You'll be prompted for:
- **Username**: Your unique identifier
- **Password**: Your password (input is masked)

After registering, you'll be asked:
- **What hackathon are you participating in?** - Select from available test suites

This creates a `ft_hackthon.yml` file in your workspace that tells the grader which test suite to use.

### 2. Login

```bash
ft_hackthon login
```

If you haven't selected a hackathon yet, you'll be prompted to choose one.

Your login token will be securely stored in `~/.ft_hackthon/config.json`.

### 3. Verify Login

```bash
ft_hackthon whoami
```

Output: `Logged in as: your_username`

## Submitting Your Project

### Prerequisites

- Your project must be a valid Git repository
- You must be logged in to ft_hackthon
- Your local branch must have committed changes

### Step 1: Navigate to Your Project

```bash
cd /path/to/your/hackathon-project
git status  # Verify it's a git repository
```

### Step 2: Ensure Your Changes Are Committed

```bash
git add .
git commit -m "Describe your changes"
```

### Step 3: Submit for Grading

```bash
ft_hackthon grademe
```

### Step 4: Wait for Results

The CLI will display real-time updates:

```
Pushed commit: abc123def456
+ Job ID: job-uuid-1234

Waiting for grading to complete...

STATUS: Queued - Waiting for grader availability...
STATUS: Processing - Running benchmarks and tests...
STATUS: Completed!

==================================================
             GRADING RESULTS
==================================================

 Parser Success ........................... YES
 Benchmark Speed ......................... 150 ms
 Final Score ............................ 95 points

==================================================

+ Grading completed successfully!
```

## Checking Job Status

Check all your submissions:

```bash
ft_hackthon status
```

If you want to check the status of a specific submission:

```bash
ft_hackthon status job-uuid-1234
```

Replace `job-uuid-1234` with your actual job ID from the submission.

## Common Commands

### Authentication Commands

```bash
# Login to ft_hackthon
ft_hackthon login

# Create a new account
ft_hackthon register

# Show current user
ft_hackthon whoami

# Logout (clears token)
ft_hackthon logout
```

### Grading Commands

```bash
# Submit current project for grading
ft_hackthon grademe

# List all grading jobs or check a specific job
ft_hackthon status
ft_hackthon status <job_id>

# View top scorers for a hackathon
ft_hackthon leaderboard <hackathon_name>

# View submission history per challenge
ft_hackthon submissions <challenge>

# View code submitted for a specific job
ft_hackthon diff <job_id>

# Check for duplicate submissions
ft_hackthon plagiarism <hackathon>
```

### Batch Submission

```bash
# Submit multiple project directories
ft_hackthon batch ../project1 ../project2 ../project3

# Submit all commits in a project
ft_hackthon batch --all-commits .
```

### Analytics and Reports

```bash
# Show submission stats for the last 30 days
ft_hackthon report

# Show stats for a specific challenge
ft_hackthon report factorial

# Show stats with trend chart for last 7 days
ft_hackthon report --days=7 --trend
```

### System Commands

```bash
# Display help
ft_hackthon help

# Display version
ft_hackthon version

# Show current Elo rating
ft_hackthon rating

# Show available commands
ft_hackthon --help
```

### Non-Interactive Mode

All REPL commands work in non-interactive mode for scripting and CI/CD:

```bash
# Submit for grading
ft_hackthon --non-interactive grademe

# Check status
ft_hackthon --non-interactive status
ft_hackthon --non-interactive status <job_id>

# Batch submission
ft_hackthon --non-interactive batch ~/projects/project-a
ft_hackthon --non-interactive batch --all-commits .

# View submissions and diffs
ft_hackthon --non-interactive submissions
ft_hackthon --non-interactive diff <job_id>

# Leaderboard, plagiarism check, reports
ft_hackthon --non-interactive leaderboard libft-tester
ft_hackthon --non-interactive plagiarism libft-tester
ft_hackthon --non-interactive report --trend

# Account info
ft_hackthon --non-interactive whoami
ft_hackthon --non-interactive rating
ft_hackthon --non-interactive logout

# JSON output (for programmatic consumption)
ft_hackthon --json status
ft_hackthon --json leaderboard libft-tester
ft_hackthon --json submissions

# Quiet mode (suppress non-essential output)
ft_hackthon --quiet grademe

# Combined for CI
ft_hackthon --non-interactive --json --insecure status
```

## Customizing API Endpoint

If you're using a custom ft_hackthon server:

```bash
ft_hackthon --api-url https://grader.example.com/api/v1 login
ft_hackthon --api-url https://grader.example.com/api/v1 grademe
```

## Workspace

Your project workspace is at `~/ft_hackthon/workspace/`. This is your Gitea repository clone:

- The workspace starts empty (just a `.git` directory)
- It contains a `.gitignore` that ignores `ft_hackthon.yml`
- A `ft_hackthon.yml` file specifies which test suite (hackathon) to use
- When you run `grademe`, your project files are copied here, committed, and pushed to Gitea

### ft_hackthon.yml

This file configures which test suite the grader should use:

```yaml
suite: libft-tester
```

The suite name must match a directory under `testsuites/` on the server.

## Leaderboard

View top scorers for a hackathon:

```bash
ft_hackthon leaderboard libft-tester
```

Output:
```
Leaderboard - libft-tester
------------------------------------------------------------------------
Rank User                 Score    Rating  Benchmark
------------------------------------------------------------------------
1    hermarti             70       1200    286ms
2    another-user         45       1100    512ms
```

## Understanding Grading Results

### Parser Success

- **YES** - Your code parser passed validation tests
- **NO** - Your code parser failed validation tests

### Benchmark Speed

- Time in milliseconds for your parser to process the test data
- Lower is better

### Final Score

- Total points earned (0-100)
- Based on parser success and benchmark performance

### Detailed Breakdown

Specific information about what was tested and any failures will be displayed in the details section.

## Error Messages

### "Not authenticated: please login first"

```bash
ft_hackthon login
# Then try your command again
```

### "Current directory is not a git repository"

```bash
cd /path/to/correct/repo
# Or initialize git
git init
git add .
git commit -m "Initial commit"
```

### "Failed to get git commit SHA"

Make sure you have committed your changes:

```bash
git add .
git commit -m "Your changes"
```

### "API connection failed"

Check if the API server is running:

```bash
curl http://localhost:8000/api/v1/health

# If using custom endpoint
curl https://grader.example.com/api/v1/health
```

## Tips and Tricks

### 1. Check Your Work Before Submitting

```bash
# Review your changes
git diff

# See commit history
git log

# Verify git state
git status
```

### 2. Multiple Submissions

You can submit multiple times:

```bash
# Make changes
vim my-parser.go
git add my-parser.go
git commit -m "Improved parser"

# Submit again
ft_hackthon grademe
```

### 3. Viewing Previous Results

View all your past submissions:

```bash
ft_hackthon status
ft_hackthon submissions
```

View code from a specific submission:

```bash
ft_hackthon diff <job_id>
```

Check for duplicate submissions:

```bash
ft_hackthon plagiarism <hackathon_name>
```

### 4. Submission Analytics

Track your performance over time:

```bash
# Overall stats
ft_hackthon report

# Per-challenge breakdown with trends
ft_hackthon report --trend --days=7
```

### 5. Batch Processing

Submit multiple projects at once:

```bash
ft_hackthon batch ~/projects/project-a ~/projects/project-b

# Or submit every commit in your history
ft_hackthon batch --all-commits .
```

### 7. Scripting

The CLI can be used in scripts with `--non-interactive` and `--json`:

```bash
#!/bin/bash
# CI/CD pipeline integration

# Submit for grading
ft_hackthon --non-interactive --insecure grademe || exit 1

# Wait and check status
sleep 30
ft_hackthon --json --non-interactive --insecure status | jq .
```

## Configuration

### Config File Location

`~/.ft_hackthon/config.json`

This file contains:
- Auth token (64-character random hex)
- Username
- Gitea repository URL (if configured)
- Gitea token (if configured)

### Clearing Configuration

```bash
ft_hackthon logout
```

Or manually remove the file:

```bash
rm ~/.ft_hackthon/config.json
```

## Privacy and Security

### What ft_hackthon Sends to the Server

- Your username (at login)
- Your password (at login, never stored locally)
- Your current git commit SHA
- Your git repository code (processed on server)

### What ft_hackthon Stores Locally

- Auth token (in `~/.ft_hackthon/config.json`)
- Username
- Gitea repository URL and token (if configured)
- No passwords or sensitive data

### Password Security

- Passwords are masked when typed (not visible on screen)
- Never stored locally
- Only transmitted over HTTPS (in production)

## Performance Expectations

| Operation | Time |
|-----------|------|
| Login | ~500ms |
| Submit | ~1 second |
| Grading (total) | 30 seconds - 10 minutes |
| Status Check | ~200ms |

Grading time depends on:
- Queue wait time
- Project complexity
- System load

## Troubleshooting

### Issue: Token Expiration

If you see authentication errors:

```bash
ft_hackthon logout
ft_hackthon login
```

### Issue: Slow Submissions

Check your network connection:

```bash
ping grader.example.com
```

### Issue: Can't Find ft_hackthon

Ensure it's installed in your PATH:

```bash
which ft_hackthon

# If not found, add to PATH
export PATH="$PATH:$GOPATH/bin"
```

### Issue: Permission Denied

Check file permissions:

```bash
chmod +x ft_hackthon
ls -la ft_hackthon
```

## Advanced Usage

### Custom API Server

```bash
# Single command
ft_hackthon --api-url http://custom-server:8000/api/v1 grademe

# Or set environment variable
export FT_HACKTHON_API_URL="http://custom-server:8000/api/v1"
ft_hackthon grademe
```

### Verbose Output

For debugging:

```bash
ft_hackthon --verbose grademe
```

### Batch Processing

Submit multiple commits:

```bash
#!/bin/bash
dirs=(~/projects/*/)
ft_hackthon batch "${dirs[@]}"
```

## Getting Help

### Built-in Help

```bash
ft_hackthon help
ft_hackthon help login
ft_hackthon help grademe
ft_hackthon help batch
ft_hackthon help report
ft_hackthon help report
```

### Version Information

```bash
ft_hackthon version
```

### Reporting Issues

If you encounter problems:

1. Note the exact error message
2. Try running with `--verbose` flag
3. Check API connectivity
4. Report to the administrator

## FAQ

**Q: How long does grading take?**
A: Usually 30 seconds to 5 minutes, depending on queue and project complexity.

**Q: Can I submit multiple times?**
A: Yes! You can submit as many times as you want. Only your final submission is graded.

**Q: What if I make a mistake in my submission?**
A: Just make the correction, commit, and submit again with `ft_hackthon grademe`.

**Q: Where is my token stored?**
A: In `~/.ft_hackthon/config.json` with restricted permissions (readable only by you).

**Q: Can I use ft_hackthon on Windows?**
A: Yes, but WSL2 is recommended for full Git integration.

**Q: What if I forget my password?**
A: Contact your administrator for password reset options.

**Q: How do I update ft_hackthon?**
A: Download the latest version and reinstall, or rebuild from source.

**Q: Can I use ft_hackthon in CI/CD?**
A: Yes! Use `--non-interactive` and `--json` flags for pipeline integration.

**Q: How do I submit multiple projects at once?**
A: Use `ft_hackthon batch <dir1> <dir2> ...` to submit multiple directories.


