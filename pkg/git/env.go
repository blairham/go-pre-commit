package git

import (
	"os"
	"strings"
)

// NoGitEnv filters out git environment variables that can interfere with npm operations
// Based on the Python pre-commit implementation which removes problematic git environment variables
func NoGitEnv(env []string) []string {
	var filteredEnv []string

	for _, envVar := range env {
		key := strings.SplitN(envVar, "=", 2)[0]

		// Skip problematic git environment variables
		if strings.HasPrefix(key, "GIT_") {
			// Allow certain git environment variables that are safe
			if strings.HasPrefix(key, "GIT_CONFIG_KEY_") ||
				strings.HasPrefix(key, "GIT_CONFIG_VALUE_") ||
				key == "GIT_EXEC_PATH" ||
				key == "GIT_SSH" ||
				key == "GIT_SSH_COMMAND" ||
				key == "GIT_SSL_CAINFO" {
				filteredEnv = append(filteredEnv, envVar)
			}
			// Skip other GIT_ variables as they can cause issues:
			// - GIT_WORK_TREE: Can cause git clone to clone wrong thing
			// - GIT_DIR: Can cause git clone to clone wrong thing
			// - GIT_INDEX_FILE: Can cause 'error invalid object ...' during commit
		} else {
			filteredEnv = append(filteredEnv, envVar)
		}
	}

	return filteredEnv
}

// NoGitEnvMap filters out git environment variables from a map
func NoGitEnvMap(envMap map[string]string) map[string]string {
	filtered := make(map[string]string)

	for key, value := range envMap {
		// Skip problematic git environment variables
		if strings.HasPrefix(key, "GIT_") {
			// Allow certain git environment variables that are safe
			if strings.HasPrefix(key, "GIT_CONFIG_KEY_") ||
				strings.HasPrefix(key, "GIT_CONFIG_VALUE_") ||
				key == "GIT_EXEC_PATH" ||
				key == "GIT_SSH" ||
				key == "GIT_SSH_COMMAND" ||
				key == "GIT_SSL_CAINFO" {
				filtered[key] = value
			}
		} else {
			filtered[key] = value
		}
	}

	return filtered
}

// GetCleanEnvironment returns the current environment with git variables filtered out
func GetCleanEnvironment() []string {
	return NoGitEnv(os.Environ())
}
