# Task Completion Checklist

Run these checks before considering a task complete:

## 1. Code Quality
```bash
make fmt                # Format code
make vet                # Static analysis
make lint               # Linter checks
```

## 2. Testing
```bash
make test               # Run all tests
```

## 3. Build Verification
```bash
make build              # Ensure it compiles
```

## 4. Full Check (All-in-One)
```bash
make check              # Runs fmt, vet, lint, test
```

## Before Committing
1. `make check` passes
2. `make build` succeeds
3. New code has appropriate tests
4. Documentation updated if needed

## Code Review Guidelines
- Follow existing patterns in codebase
- Add package comments for new packages
- Document exported functions
- Handle errors appropriately
- No sensitive data in commits

## Integration Testing
For changes affecting API interactions:
```bash
make test-integration   # Requires F5XC credentials
```
