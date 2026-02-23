# Go Conventions

## Stack

- **CLI Framework**: Cobra
- **Testing**: testify
- **Linting**: golangci-lint

## Architecture

- Place interfaces close to their consumers when needed
- Follow Go idioms - avoid Java-style patterns
- Keep it simple: DRY, KISS, YAGNI

## Code Style

Write idiomatic Go:

- No unnecessary abstractions or enterprise patterns
- Prefer composition over complex hierarchies
- Use standard library patterns where appropriate
- Let golangci-lint guide style decisions
