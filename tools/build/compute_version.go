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
		cut, _, isPostTag := strings.Cut(tag, "-")
		if isPostTag {
			fmt.Fprintf(os.Stderr, "Warning: Working tree at %s contains commits after the latest tag %s\n", tag, cut)
		}
		if isTagStable, err := isStableReleaseTag(cut); err != nil {
			fmt.Fprintf(os.Stderr, "Error: couldn't determine if the tag '%s' means a stable release or not: %v\n", tag, err)
			os.Exit(1)
		} else if isTagStable {
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

// isStableReleaseTag reports whether the supplied tag implies a stable relase or a beta tag.
// The stable one is shaped as M.m.p, while a beta-like pre-release is of shape M.m.p[ab]r, where:
//
// M - is the major version number
// m - is the minor version number
// p - is the patch version number
// a or b - is a hardcoded suffix, a for alpha and b for beta (we don't really care about whether
// alpha or beta at this point)
// r - is the revision number
//
// Note that neither the a/b suffix nor the revision number make into the MSIX version
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
// 1.9999.15b4:
//
//	pre-release (v2.0 to be), MSIX is 1.9999.15.0 and must not be published in the regular
//	channel. RTD doesn't publish this tag, but because it's in the main branch its contents are
//	presented in the 'latest' version of the documentation.
func isStableReleaseTag(tag string) (bool, error) {
	stableExpr := regexp.MustCompile(`^(\d+\.(\d+)\.\d+)$`)
	matches := stableExpr.FindStringSubmatch(tag)
	if len(matches) == 3 {
		minor, err := strconv.ParseInt(matches[2], 10, 32)
		if err != nil {
			return false, fmt.Errorf("could not parse minor version number: %v", err)
		}
		if minor == 9999 {
			return false, errors.New("the minor version number 9999 is reserved for pre-releases")
		}

		return true, nil
	}
	unstableExpr := regexp.MustCompile(`^(\d+\.9999\.\d+[ab]\d+)$`)
	matches = unstableExpr.FindStringSubmatch(tag)
	if len(matches) != 2 {
		return false, errors.New("tag does not match the stable (M.m.p) nor the pre-release (M.9999.p[ab]r) versioning patterns")
	}

	return false, nil
}
