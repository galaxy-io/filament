package main

import (
	"fmt"
	"os"
	"strings"
)

func environmentReferenceName(value string) (string, bool) {
	var name string
	switch {
	case strings.HasPrefix(value, "env:"):
		name = strings.TrimPrefix(value, "env:")
	case strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}"):
		name = strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
	case strings.HasPrefix(value, "$"):
		name = strings.TrimPrefix(value, "$")
	default:
		return "", false
	}
	return name, envNamePattern.MatchString(name)
}

func normalizeEnvironmentName(value string) (string, error) {
	if name, ok := environmentReferenceName(value); ok {
		return name, nil
	}
	if envNamePattern.MatchString(value) {
		return value, nil
	}
	return "", fmt.Errorf("environment variable must be NAME, $NAME, ${NAME}, or env:NAME")
}

func resolveEnvironmentReference(value string) (string, error) {
	name, referenced := environmentReferenceName(value)
	if !referenced {
		if strings.HasPrefix(value, "env:") {
			return "", fmt.Errorf("environment reference must use env:NAME")
		}
		return value, nil
	}
	resolved, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("environment variable %s is not set", name)
	}
	return resolved, nil
}
