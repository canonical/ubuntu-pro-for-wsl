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
	var isStable bool
	flag.BoolVar(&numeric, "numeric", false, "Print a numeric version")
	flag.BoolVar(&isStable, "is-stable", false, "Print whether the version is a stable release or pre-release")
	flag.Parse()

	tag := getTag()
	version, stableOr := computeNumericVersion(tag)

	if numeric {
		fmt.Println(version)
		return
	}

	if isStable {
		cut, _, isPostTag := strings.Cut(tag, "-")
		if isPostTag {
			fmt.Fprintf(os.Stderr, "Warning: Working tree at %s contains commits after the latest tag %s\n", tag, cut)
		}
		if stableOr {
			fmt.Println("stable")
			return
		}
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

// computeNumericVersion returns a numeric semantic version number extracted from the Git tag and a
// boolean flag to indicate whether that tag obeys the pattern of a stable release (true) or
// prerelease (false).
//
// A stable release tag is shaped as M.m.p, while a beta-like pre-release is of shape M.9999[ab]p, where:
//
// M - is the major version number
// m - is the minor version number
// p - is the patch version number
// a or b - is a hardcoded suffix, a for alpha and b for beta (we don't really care about whether
// alpha or beta at this point)
//
// Note that neither the a/b suffix or the revision number make into the MSIX version
// (msbuild fails to package if the appxmanifest uses the 4th element of the Identity.Version field)
// so for the Windows packaging purposes we rely on a the m(minor version) being an arbitrarely high
// value (9999) as a convention for MSIX packages that should not be published in the stable channel.
// On the other hand, if we didn't use the suffix, we'd fool Read-The-Docs into considering a tag
// like 1.9999.15 the latest stable tag and publish the wrong documentation contents.
//
// A few examples:
//
// 1.15.8:
//
//	stable, MSIX is 1.15.8.0 and RTD publishes this as the 'stable' docs website.
//
// 1.9999.0:
//
//	we fail and bail out from the publishing workflow.
//
// 1.9999b15:
//
//	pre-release (v2.0 to be), MSIX is 1.9999.15.0 and must not be published in the regular
//	channel. RTD doesn't publish this tag, but because it's in the main branch its contents are
//	presented in the 'latest' version of the documentation.
func computeNumericVersion(tag string) (string, bool) {
	expr := regexp.MustCompile(`(\d+)\.(\d+)([ab]|\.)(\d+)`)
	matches := expr.FindStringSubmatch(tag)
	if len(matches) != 5 {
		fmt.Fprintf(os.Stderr, "Error: tag %s does not match the expected format\n", tag)
		os.Exit(1)
	}
	minor := matches[2]
	separator := matches[3]
	if (separator == "a" || separator == "b") && minor != "9999" {
		fmt.Fprintf(os.Stderr, "Tag %s doesn't follow the prerelease scheme M.9999[ab]P", tag)
		os.Exit(1)
	}
	isStableRelease := separator == "." && minor != "9999"
	major := matches[1]
	patch := matches[4]
	return fmt.Sprintf("%s.%s.%s", major, minor, patch), isStableRelease
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
