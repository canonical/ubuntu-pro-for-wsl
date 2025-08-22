// Package main is a tool to compute the version of the project.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	var numeric bool
	var isStable bool
	flag.BoolVar(&numeric, "numeric", false, "Print a numeric version")
	flag.BoolVar(&isStable, "is-stable", false, "Print whether the version is a stable release or pre-release")
	flag.Parse()

	tag := getTag()

	if numeric {
		fmt.Println(computeNumericVersion(tag))
		return
	}

	if isStable {
		fmt.Println("pre-release")
		return
	}

	fmt.Println(computeFullVersion(tag))
}

func computeFullVersion(tag string) string {
	if dirty, err := isDirty(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	} else if dirty {
		tag += "-dirty"
	}

	return tag
}

func computeNumericVersion(tag string) string {
	expr := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := expr.FindStringSubmatch(tag)
	if len(matches) != 2 {
		fmt.Fprintf(os.Stderr, "Error: tag %s does not match the expected format\n", tag)
		os.Exit(1)
	}

	return matches[1]
}

func getTag() string {
	// Note: we cannot use --dirty because it does not detect untracked files.
	out, err := exec.Command("git", "describe", "--tags", "HEAD").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: git describe: %v. %s\n", err, out)
		os.Exit(1)
	}

	return strings.TrimSpace(string(out))
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
