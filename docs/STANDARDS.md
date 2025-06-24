# Language Implementation Standards

This document outlines the standards and patterns that all language implementations in `pkg/repository/languages` should follow for consistency.

## Standardization Summary

The following standardizations have been applied to ensure consistency across all language implementations:

### ✅ Completed Standardizations

1. **Registry Organization**: Reorganized language registration in alphabetical order with clear categorization
2. **Constructor Naming**: Standardized base language names to lowercase (e.g., "golang", "node", "python")
3. **Method Consistency**: Ensured all languages have required core methods
4. **Health Check Standardization**: Converted duplicate `CheckEnvironmentHealth` methods to standardized `CheckHealth` methods
5. **Missing Method Addition**: Added missing `GetDefaultVersion` methods to utility languages
6. **Comment Standardization**: Ensured consistent comment patterns across implementations

### 🔧 Languages Standardized

- **golang.go**: Fixed constructor name from "Go" to "golang"
- **node.go**: Fixed constructor name from "Node" to "node"  
- **swift.go**: Removed duplicate `CheckEnvironmentHealth`, kept `CheckHealth`
- **dotnet.go**: Removed duplicate `CheckEnvironmentHealth`, enhanced `CheckHealth`
- **haskell.go**: Converted `CheckEnvironmentHealth` to `CheckHealth`
- **system.go**: Added missing `GetDefaultVersion` method
- **script.go**: Added missing `GetDefaultVersion` method
- **fail.go**: Added missing `GetDefaultVersion` method
- **docker.go**: Added missing `GetDefaultVersion` method
- **docker_image.go**: Added missing `GetDefaultVersion` method

## Required Methods

Every language implementation MUST implement these methods with consistent signatures:

### 1. Constructor
```go
func NewXxxLanguage() *XxxLanguage {
    return &XxxLanguage{
        Base: language.NewBase(
            "languagename",  // lowercase language name (matches registry key)
            "executable",    // primary executable name
            "--version",     // version flag
            "https://...",   // installation URL
        ),
    }
}
```

### 2. GetDefaultVersion
```go
// GetDefaultVersion returns the default [Language] version
// Following Python pre-commit behavior: returns 'system' if [Language] is installed, otherwise 'default'
func (l *XxxLanguage) GetDefaultVersion() string {
    if l.IsRuntimeAvailable() {
        return language.VersionSystem
    }
    return language.VersionDefault
}
```

### 3. SetupEnvironmentWithRepo  
```go
// SetupEnvironmentWithRepo sets up a [Language] environment for a specific repository
func (l *XxxLanguage) SetupEnvironmentWithRepo(
    cacheDir, version, repoPath, _ string, // repoURL is unused
    additionalDeps []string,
) (string, error) {
    // Implementation
}
```

### 4. InstallDependencies
```go
// InstallDependencies installs [Language] packages/dependencies
func (l *XxxLanguage) InstallDependencies(envPath string, deps []string) error {
    // Implementation or return nil if not supported
}
```

### 5. CheckHealth
```go
// CheckHealth checks if the [Language] environment is healthy
func (l *XxxLanguage) CheckHealth(envPath, version string) error {
    // Implementation
}
```

## Optional Methods

Languages MAY implement these methods if they provide additional functionality:

### SetupEnvironmentWithRepoInfo
```go
// SetupEnvironmentWithRepoInfo sets up a [Language] environment with repository URL information
func (l *XxxLanguage) SetupEnvironmentWithRepoInfo(
    cacheDir, version, repoPath, repoURL string,
    additionalDeps []string,
) (string, error) {
    return l.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}
```

### PreInitializeEnvironmentWithRepoInfo
```go
// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (l *XxxLanguage) PreInitializeEnvironmentWithRepoInfo(
    cacheDir, version, repoPath, repoURL string,
    additionalDeps []string,
) error {
    // Implementation
}
```

## Naming Conventions

1. **Language Names**: Use lowercase names that match the registry keys:
   - `"golang"` not `"Go"`
   - `"node"` not `"Node"`
   - `"python"` not `"Python"`

2. **Struct Names**: Use PascalCase with "Language" suffix:
   - `GoLanguage`, `NodeLanguage`, `PythonLanguage`

3. **File Names**: Use lowercase with language name:
   - `golang.go`, `node.go`, `python.go`

4. **Method Comments**: Follow consistent pattern:
   ```go
   // GetDefaultVersion returns the default [Language] version
   // Following Python pre-commit behavior: returns 'system' if [Language] is installed, otherwise 'default'
   ```

## Error Handling

1. **Return Patterns**: 
   - Methods that can fail should return `(string, error)` or `error`
   - Use descriptive error messages with `fmt.Errorf()`
   - Wrap errors with context: `fmt.Errorf("failed to do X: %w", err)`

2. **Health Checks**:
   - Use `CheckHealth(envPath, version string) error` instead of `CheckEnvironmentHealth`
   - Return `nil` for healthy environments
   - Provide specific error messages for debugging

## Environment Setup Patterns

1. **Environment Paths**: Use `language.GetRepositoryEnvironmentName()` for consistency
2. **Version Handling**: Support `"default"`, `"system"`, and specific versions
3. **Cache Directories**: Handle empty `repoPath` by using `cacheDir`
4. **Additional Dependencies**: Always accept `[]string` even if not used

## Registry Integration

All languages are registered in `registry.go` with:

```go
// Primary programming languages (alphabetical order)
registry.languages["conda"] = NewCondaLanguage()
registry.languages["coursier"] = NewCoursierLanguage()
registry.languages["dart"] = NewDartLanguage()
// ... etc

// Container technologies
registry.languages["docker"] = NewDockerLanguage()
registry.languages["docker_image"] = NewDockerImageLanguage()

// System and utility languages  
registry.languages["fail"] = NewFailLanguage()
registry.languages["pygrep"] = NewPygrepLanguage()
registry.languages["script"] = NewScriptLanguage()
registry.languages["system"] = NewSystemLanguage()
```

## Testing

Each language should have corresponding test files following the pattern:
- `languagename_test.go`
- Test all public methods
- Use consistent test naming: `TestLanguageNameMethod`

## Validation

All standardizations have been validated and tests are passing. The registry properly initializes all languages and maintains compatibility with existing functionality.
