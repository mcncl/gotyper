# Contributing to GoTyper

Thank you for your interest in contributing to GoTyper! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Project Structure](#project-structure)
- [Release Process](#release-process)

## Code of Conduct

This project adheres to a code of conduct that we expect all contributors to follow:

- **Be respectful**: Treat everyone with respect and kindness
- **Be inclusive**: Welcome contributions from everyone regardless of background
- **Be collaborative**: Work together constructively to improve the project
- **Be patient**: Help newcomers and answer questions thoughtfully

## Getting Started

### Prerequisites

- Go 1.25 or later
- Git
- A GitHub account

### First Steps

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/gotyper.git
   cd gotyper
   ```
3. **Add the upstream remote**:
   ```bash
   git remote add upstream https://github.com/mcncl/gotyper.git
   ```

## Development Setup

### Install Dependencies

```bash
# Download Go modules
go mod download

# Install development tools
go install honnef.co/go/tools/cmd/staticcheck@latest
```

### Verify Setup

```bash
# Run tests to ensure everything works
go test ./...

# Build the binary
go build -o gotyper .

# Test basic functionality
./gotyper --help
```

### Git Hooks (Optional)

We use Lefthook for Git hooks. To install:

```bash
# Install lefthook if you don't have it
go install github.com/evilmartians/lefthook@latest

# Install hooks
lefthook install
```

## Making Changes

### Before You Start

1. **Check existing issues** to see if your idea is already being discussed
2. **Create an issue** for new features or significant changes to discuss the approach
3. **Create a feature branch** from main:
   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feature/your-feature-name
   ```

### Development Guidelines

#### Adding New Features

1. **Start with tests** - Write tests that describe the expected behavior
2. **Implement incrementally** - Make small, focused commits
3. **Update documentation** - Update README.md and code comments
4. **Add configuration** - If the feature needs configuration, update the config system

#### Fixing Bugs

1. **Write a test** that reproduces the bug
2. **Fix the bug** while ensuring the test passes
3. **Verify** that existing functionality still works

#### Example Development Flow

```bash
# 1. Create feature branch
git checkout -b feature/add-custom-types

# 2. Write tests
# Edit internal/analyzer/analyzer_test.go

# 3. Run tests (should fail initially)
go test ./internal/analyzer -v

# 4. Implement feature
# Edit internal/analyzer/analyzer.go

# 5. Run tests (should pass now)
go test ./internal/analyzer -v

# 6. Run full test suite
go test ./...

# 7. Update documentation
# Edit README.md, add examples

# 8. Commit changes
git add .
git commit -m "feat: add support for custom type mappings"
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View coverage in browser

# Run specific package tests
go test ./internal/analyzer -v

# Run specific test
go test ./internal/analyzer -run TestAnalyze_EnhancedTimeFormats -v
```

### Writing Tests

#### Unit Tests

- Place tests in the same package as the code being tested
- Use descriptive test names: `TestAnalyze_ComplexNestedObjects`
- Use table-driven tests for multiple scenarios
- Test both happy path and edge cases

Example:
```go
func TestAnalyzer_NewFeature(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expected    string
        description string
    }{
        {
            name:        "basic case",
            input:       `{"field": "value"}`,
            expected:    "expectedOutput",
            description: "Should handle basic input correctly",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### Integration Tests

- Test complete workflows from JSON input to Go output
- Use real-world JSON examples
- Verify configuration behavior
- Test CLI functionality

### Test Requirements

- **All new code must have tests**
- **Maintain >90% test coverage**
- **Integration tests for new CLI features**
- **Configuration tests for new options**

## Submitting Changes

### Pull Request Process

1. **Update your branch** with the latest changes:
   ```bash
   git checkout main
   git pull upstream main
   git checkout your-feature-branch
   git rebase main  # or git merge main
   ```

2. **Push your changes**:
   ```bash
   git push origin your-feature-branch
   ```

3. **Create a Pull Request** on GitHub with:
   - Clear title describing the change
   - Detailed description of what was changed and why
   - Link to related issues
   - Screenshots/examples if applicable

4. **Address review feedback** and update the PR as needed

### PR Requirements

- [ ] All tests pass
- [ ] Code follows project style guidelines
- [ ] Documentation updated if needed
- [ ] No breaking changes (or clearly documented)
- [ ] Commit messages are clear and descriptive

### Commit Message Format

Use clear, descriptive commit messages:

```
type(scope): brief description

Longer explanation if needed, wrapped at 72 characters.

- Key changes made
- Why the change was necessary
- Any important considerations

Fixes #123
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test improvements
- `chore`: Build/maintenance tasks

## Code Style

### Go Guidelines

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Write clear, concise comments for complex logic
- Keep functions focused and small
- Handle errors appropriately

### GoTyper-Specific Guidelines

#### Configuration
- New configuration options should have sensible defaults
- Update the example configuration file
- Add validation for new config options

#### Type Detection
- New type patterns should be thoroughly tested
- Order regex patterns by specificity (most specific first)
- Document new patterns in README.md

#### Code Organization
- Keep related functionality together
- Use clear package structure
- Export only what's necessary

### Example Code Style

```go
// Good: Clear function with good naming and error handling
func (a *Analyzer) detectTimeFormat(s string) (models.TypeInfo, bool) {
    if rfc3339Regex.MatchString(s) {
        a.analysisResult.Imports["time"] = struct{}{}
        return models.TypeInfo{Kind: models.Time, Name: "time.Time"}, true
    }
    return models.TypeInfo{}, false
}

// Good: Table-driven test with clear structure
func TestDetectTimeFormat(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
    }{
        {"RFC3339", "2023-01-15T14:30:00Z", true},
        {"invalid", "not-a-date", false},
    }
    // ... test implementation
}
```

## Project Structure

```
gotyper/
├── .github/                 # GitHub templates and workflows
├── internal/                # Internal packages
│   ├── analyzer/           # JSON analysis and type detection
│   ├── cli/               # Command-line interface
│   ├── config/            # Configuration management
│   ├── formatter/         # Go code formatting
│   ├── generator/         # Go code generation
│   ├── models/            # Data models
│   └── parser/            # JSON parsing
├── testdata/              # Test data files
├── main.go               # Application entry point
├── go.mod               # Go module definition
└── README.md           # Documentation
```

### Package Responsibilities

- **analyzer**: Core logic for analyzing JSON and inferring Go types
- **config**: Configuration file handling and validation
- **generator**: Go struct and code generation
- **formatter**: Code formatting and cleanup
- **parser**: JSON parsing and intermediate representation
- **cli**: Command-line interface and user interaction

## Release Process

Releases are automated through GitHub Actions:

1. **Tag a version**: `git tag v1.2.3`
2. **Push the tag**: `git push origin v1.2.3`
3. **GitHub Actions** automatically:
   - Runs full test suite
   - Builds binaries for multiple platforms
   - Creates GitHub release with changelog
   - Uploads release assets

### Version Numbering

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes

## Getting Help

- **GitHub Issues**: For bug reports and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Code Review**: Don't hesitate to ask for feedback on PRs

## Recognition

Contributors will be recognized in:
- GitHub contributors page
- Release notes for significant contributions
- Special thanks in README for major features

Thank you for contributing to GoTyper! 🚀