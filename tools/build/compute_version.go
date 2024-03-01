// Package main is a tool to compute the version of the project.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	var numeric bool
	flag.BoolVar(&numeric, "numeric", false, "Print a numeric version")
	flag.Parse()

	if numeric {
		fmt.Println(computeNumericVersion())
		return
	}

	fmt.Println(computeFullVersion())
}

func computeFullVersion() string {
	tag, err := getTag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if dirty, err := isDirty(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	} else if dirty {
		tag += "-dirty"
	}

	return tag
}

func computeNumericVersion() string {
	tag, err := getTag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	expr := regexp.MustCompile(`^v(\d+\.\d+\.\d+)-`)
	matches := expr.FindStringSubmatch(tag)
	if len(matches) != 2 {
		fmt.Fprintf(os.Stderr, "Error: tag %s does not match the expected format\n", tag)
		os.Exit(1)
	}

	return matches[1]
}

func getTag() (string, error) {
	// Note: we cannot use --dirty because it does not detect untracked files.
	out, err := exec.Command("git", "describe", "--tags", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git describe: %v. %s", err, out)
	}

	return strings.TrimSpace(string(out)), nil
}

func isDirty() (bool, error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return false, fmt.Errorf("git status: %v. %s", err, out)
	}
	if len(out) != 0 {
		return true, nil
	}

	return false, nil
}
