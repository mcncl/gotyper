# Pull Request

## Description

<!-- Provide a clear and concise description of the changes in this PR -->

## Type of Change

<!-- Please check the type of change your PR introduces -->

- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New feature (non-breaking change which adds functionality)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📚 Documentation update
- [ ] 🔧 Code refactoring (no functional changes)
- [ ] ⚡ Performance improvement
- [ ] 🧪 Test improvements
- [ ] 🏗️ Build/CI improvements

## Related Issues

<!-- Link to related issues -->
Fixes #(issue_number)
Closes #(issue_number)
Related to #(issue_number)

## Changes Made

<!-- List the main changes made in this PR -->

- 
- 
- 

## Sample Usage (if applicable)

<!-- Show how the new feature/fix works -->

### Input JSON:
```json
{
  "example": "data"
}
```

### Command:
```bash
gotyper -i example.json -p mypackage
```

### Generated Output:
```go
package mypackage

type RootType struct {
    Example string `json:"example"`
}
```

## Testing

<!-- Describe the tests you've added/run -->

- [ ] Added new unit tests
- [ ] Added new integration tests  
- [ ] Updated existing tests
- [ ] All tests pass locally
- [ ] Manual testing performed

### Test Commands Run:
```bash
# List the test commands you executed
go test ./...
go test -race ./...
```

## Configuration Changes

<!-- If this PR involves configuration changes -->

- [ ] Updated configuration schema
- [ ] Updated example configuration file
- [ ] Backward compatibility maintained
- [ ] Migration guide provided (if needed)

## Documentation

<!-- Documentation updates made -->

- [ ] Updated README.md
- [ ] Updated code comments
- [ ] Updated configuration examples
- [ ] Added usage examples

## Breaking Changes

<!-- If this is a breaking change, describe the impact -->

- [ ] This PR introduces breaking changes
- [ ] Migration guide provided
- [ ] Changelog updated

**Breaking changes description:**
<!-- Describe what breaks and how to migrate -->

## Checklist

<!-- Please review and check off completed items -->

### Code Quality
- [ ] Code follows the project's coding standards
- [ ] Self-review completed
- [ ] Complex code sections are commented
- [ ] No debug code or console.log statements left
- [ ] Error handling is appropriate

### Testing
- [ ] New tests added for new functionality
- [ ] Existing tests still pass
- [ ] Edge cases are tested
- [ ] Performance impact considered

### Dependencies
- [ ] No unnecessary dependencies added
- [ ] All new dependencies are justified
- [ ] Dependency licenses are compatible

### Compatibility
- [ ] Backward compatibility maintained (or breaking changes documented)
- [ ] Works with supported Go versions
- [ ] Cross-platform compatibility considered

## Performance Impact

<!-- Describe any performance implications -->

- [ ] No performance impact
- [ ] Performance improved
- [ ] Performance impact acceptable
- [ ] Performance benchmarks provided

## Screenshots (if applicable)

<!-- Add screenshots for UI changes or CLI output examples -->

## Additional Notes

<!-- Any additional information that reviewers should know -->

## Reviewer Notes

<!-- @mcncl - Add any specific areas you'd like reviewers to focus on -->