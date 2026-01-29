# PRMate

AI-powered Pull Request reviewer that learns your codebase conventions and automatically reviews PRs against your team's standards.

## Features

- 🔍 **Automatic PR Reviews** - Analyzes code changes against your project's rules
- 📝 **Inline Comments** - Posts specific feedback on exact lines that need attention
- 🧠 **Context-Aware** - Understands file dependencies and imports for smarter reviews
- 📊 **Review Summaries** - Tracks what's been reviewed for incremental updates
- 🔄 **Incremental Reviews** - Only re-reviews changed files when PRs are updated
- 🤖 **Multiple LLM Providers** - Works with GitHub Copilot or OpenAI-compatible APIs

## How It Works

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  GitHub PR      │────▶│  PRMate         │────▶│  LLM (Copilot/  │
│  Webhook        │     │  Review Engine  │     │  OpenAI)        │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                               │
                               ▼
                        ┌─────────────────┐
                        │  GitHub API     │
                        │  (Comments)     │
                        └─────────────────┘
```

1. **PR Created/Updated** → GitHub sends webhook to PRMate
2. **Load Rules** → Reads `.prmate.md` for project conventions
3. **Analyze Files** → For each changed file:
   - Fetches file content and dependencies (imports)
   - Sends to LLM with rules and context
   - Parses violations
4. **Post Feedback** → Creates inline review comments on specific lines
5. **Track Progress** → Posts summary with tracking data for incremental reviews

## Setup

### 1. Create `.prmate.md` in Your Repository

PRMate reads your coding standards from a `.prmate.md` file at the root of your repo:

```markdown
# PRMate Context

## Review Checklist
- [ ] All errors must be wrapped with context using fmt.Errorf
- [ ] New functions must have unit tests
- [ ] No hardcoded credentials or secrets
- [ ] Public APIs must have documentation comments

## Learned Rules
- Use dependency injection for services
- Follow clean architecture patterns
- Error messages should be lowercase
- Use context.Context as first parameter
```

You can generate this file automatically using the `@scan` directive (see below).

### 2. Configure Environment Variables

```bash
# Required
GITHUB_TOKEN=ghp_xxxx          # GitHub token with repo access

# Webhook (optional but recommended)
WEBHOOK_SECRET=your-secret     # For validating webhook signatures

# LLM Provider (choose one)
LLM_PROVIDER=copilot           # Use GitHub Copilot (default)
COPILOT_MODEL=gpt-5-mini       # Copilot model to use

# OR use OpenAI-compatible API
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-xxxx
OPENAI_BASE_URL=https://api.openai.com/v1  # Optional, for custom endpoints
OPENAI_MODEL=gpt-4             # Model to use

# Server Configuration
PORT=8080                      # HTTP server port
PR_WORK_BASE_DIR=/tmp/prmate   # Working directory for PR processing
WEBHOOK_QUEUE_SIZE=100         # Async webhook queue size
WEBHOOK_WORKERS=2              # Number of webhook processing workers
```

### 3. Set Up GitHub Webhook

1. Go to your repository **Settings** → **Webhooks** → **Add webhook**
2. Set **Payload URL** to `https://your-server.com/webhook`
3. Set **Content type** to `application/json`
4. Set **Secret** to match your `WEBHOOK_SECRET`
5. Select events: **Pull requests**, **Issue comments**

### 4. Run PRMate

```bash
# Build and run
go build -o prmate .
./prmate

# Or with Docker
docker build -t prmate .
docker run -p 8080:8080 --env-file .env prmate
```

## Usage

### Automatic Reviews

Once configured, PRMate automatically reviews PRs when they are:
- **Opened** - Full review of all changed files
- **Synchronized** (new commits pushed) - Incremental review of newly changed files
- **Reopened** - Full review

### Manual Trigger

Comment `@prmate` on any PR to trigger a review or re-scan.

### Scanning Codebase

To generate or update your `.prmate.md` with learned conventions, add this comment block to the file:

```markdown
<!-- PRMate
@scan
github.com/org/other-repo
-->
```

This will:
1. Scan your codebase for patterns and conventions
2. Optionally scan external repos for additional context
3. Generate/update `.prmate.md` with detected rules

## Review Output

### Inline Comments

PRMate posts inline comments on specific lines:

> ⚠️ **Error Handling**: Error not wrapped with context. Use `fmt.Errorf("context: %w", err)`

### Summary Comment

Each review posts a summary table:

| Metric | Value |
|--------|-------|
| Files Reviewed | 5 |
| Rules Applied | 12 |
| Issues Found | 3 |
| Commit | `abc123d` |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/webhook` | POST | GitHub webhook receiver |
| `/health` | GET | Health check |
| `/api/weather-joke` | POST | Demo endpoint (LLM test) |

## Project Structure

```
prmate/
├── main.go                    # Application entry point
├── internal/
│   ├── config/               # Configuration management
│   ├── copilot/              # GitHub Copilot SDK integration
│   ├── github/               # GitHub API client
│   ├── handlers/             # HTTP handlers
│   ├── llm/                  # LLM provider abstraction
│   │   ├── provider.go       # Interfaces
│   │   └── openai.go         # OpenAI-compatible provider
│   ├── review/               # PR Review Engine
│   │   ├── service.go        # Main review logic
│   │   └── types.go          # Data types
│   ├── scan/                 # Codebase scanning
│   ├── scanner/              # Code analysis
│   ├── server/               # HTTP server
│   └── webhook/              # Webhook processing
└── tools/                    # CLI utilities
```

## Development

```bash
# Run tests
go test ./...

# Build
go build ./...

# Run locally
export GITHUB_TOKEN=ghp_xxxx
go run main.go
```

## License

MIT
