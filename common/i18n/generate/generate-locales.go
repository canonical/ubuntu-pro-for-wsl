//go:build tools
// +build tools

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/common/internal/generators"
)

const usage = `Usage of %s:

   create-po POT DIRECTORY LOC [LOC...]
     Create new LOC.po file(s) in DIRECTORY from an existing pot file.
   update-po POT DIRECTORY PACKAGE [PACKAGES...]
     Create/Update a pot file and refresh any existing po files in DIRECTORY.
   generate-mo DOMAIN PODIR DIRECTORY
     Create .mo files for any .po in PODIR in an structured hierarchy in DIRECTORY.
`

func main() {
	if len(os.Args) < 2 {
		log.Fatalf(usage, os.Args[0])
	}

	switch os.Args[1] {
	case "help":
		fmt.Printf(usage, os.Args[0])
		return
	case "create-po":
		if len(os.Args) < 5 {
			log.Fatalf(usage, os.Args[0])
		}
		if generators.InstallOnlyMode() {
			return
		}
		if err := createPo(os.Args[2], os.Args[3], os.Args[4:]); err != nil {
			log.Fatalf("Error when creating po files: %v", err)
		}

	case "update-po":
		if len(os.Args) < 5 {
			log.Fatalf(usage, os.Args[0])
		}
		if generators.InstallOnlyMode() {
			return
		}
		if err := updatePo(os.Args[2], os.Args[3], os.Args[4:]...); err != nil {
			log.Fatalf("Error when updating po files: %v", err)
		}

	case "generate-mo":
		if len(os.Args) != 4 {
			log.Fatalf(usage, os.Args[0])
		}
		if err := generateMo(os.Args[2], filepath.Join(generators.DestDirectory(os.Args[3]), "usr", "share")); err != nil {
			log.Fatalf("Error when generating mo files: %v", err)
		}
	default:
		log.Fatalf(usage, os.Args[0])
	}
}

// createPo creates new po files.
func createPo(potfile, localeDir string, locs []string) error {
	if _, err := os.Stat(potfile); err != nil {
		return fmt.Errorf("%q can't be read: %v", potfile, err)
	}

	for _, loc := range locs {
		pofile := filepath.Join(localeDir, loc+".po")
		if _, err := os.Stat(pofile); err == nil {
			log.Printf("Skipping %q as already exists. Please use update-po to refresh it or delete it first.", loc)
			continue
		}

		if out, err := exec.Command("msginit",
			"--input="+potfile, "--locale="+loc+".UTF-8", "--no-translator", "--output="+pofile).CombinedOutput(); err != nil {
			return fmt.Errorf("couldn't create %q: %v.\nCommand output: %s", pofile, err, out)
		}
	}

	return nil
}

// updatePo creates pot files and update any existing .po ones.
func updatePo(potfile, localeDir string, packages ...string) error {
	if err := os.MkdirAll(localeDir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory for %q: %v", localeDir, err)
	}

	_, current, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("could not get repository root")
	}

	// ROOT/common/i18n
	root := dir(current, 4)

	// Create pot file
	var files []string
	var err error
	for _, dir := range packages {
		err = errors.Join(err, filepath.WalkDir(dir, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("fail to access %q: %v", p, err)
			}
			// Only deal with files
			if de.IsDir() {
				return nil
			}

			if !strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, ".go.template") {
				return nil
			}

			files = append(files, p)
			return nil
		}))
	}

	if err != nil {
		return err
	}

	var potcreation string
	// if already existed: extract POT creation date to keep it (xgettext always updates it)
	if _, err := os.Stat(potfile); err == nil {
		if potcreation, err = getPOTCreationDate(potfile); err != nil {
			log.Fatal(err)
		}
	}
	args := append([]string{
		"--keyword=G", "--keyword=GN", "--add-comments", "--sort-output", "--package-name=" + strings.TrimSuffix(filepath.Base(potfile), ".pot"),
		"-D", root, "--output=" + potfile}, files...)
	if out, err := exec.Command("xgettext", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("couldn't compile pot file: %v\nCommand output: %s", err, out)
	}
	if potcreation != "" {
		if err := rewritePOTCreationDate(potcreation, potfile); err != nil {
			log.Fatalf("couldn't change POT Creation file: %v", err)
		}
	}

	// Merge existing po files
	poCandidates, err := os.ReadDir(localeDir)
	if err != nil {
		log.Fatalf("couldn't list content of %q: %v", localeDir, err)
	}
	for _, f := range poCandidates {
		if !strings.HasSuffix(f.Name(), ".po") {
			continue
		}

		pofile := filepath.Join(localeDir, f.Name())

		// extract POT creation date to keep it (msgmerge always updates it)
		potcreation, err := getPOTCreationDate(pofile)
		if err != nil {
			log.Fatal(err)
		}

		if out, err := exec.Command("msgmerge", "--update", "--backup=none", pofile, potfile).CombinedOutput(); err != nil {
			return fmt.Errorf("couldn't refresh %q: %v.\nCommand output: %s", pofile, err, out)
		}

		if err := rewritePOTCreationDate(potcreation, pofile); err != nil {
			log.Fatalf("couldn't change POT Creation file: %v", err)
		}
	}

	return nil
}

func dir(path string, levels uint) string {
	if levels == 0 {
		return path
	}
	return dir(filepath.Dir(path), levels-1)
}

// generateMo generates a locale directory structure with a mo for each po in localeDir.
func generateMo(in, out string) error {
	baseLocaleDir := filepath.Join(out, "locale")
	if err := generators.CleanDirectory(baseLocaleDir); err != nil {
		log.Fatalln(err)
	}

	poCandidates, err := os.ReadDir(in)
	if err != nil {
		log.Fatalf("couldn't list content of %q: %v", in, err)
	}
	for _, f := range poCandidates {
		if !strings.HasSuffix(f.Name(), ".po") {
			continue
		}

		candidate := filepath.Join(in, f.Name())
		outDir := filepath.Join(baseLocaleDir, strings.TrimSuffix(f.Name(), ".po"), "LC_MESSAGES")
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("couldn't create %q: %v", out, err)
		}
		if out, err := exec.Command("msgfmt",
			"--output-file="+filepath.Join(outDir, common.TEXTDOMAIN+".mo"),
			candidate).CombinedOutput(); err != nil {
			return fmt.Errorf("couldn't compile mo file from %q: %v.\nCommand output: %s", candidate, err, out)
		}
	}
	return nil
}

const potCreationDatePrefix = `"POT-Creation-Date:`

func getPOTCreationDate(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("couldn't open %q: %v", p, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), potCreationDatePrefix) {
			return scanner.Text(), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error while reading %q: %v", p, err)
	}

	return "", fmt.Errorf("didn't find %q in %q", potCreationDatePrefix, p)
}

func rewritePOTCreationDate(potcreation, p string) error {
	f, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("couldn't open %q: %v", p, err)
	}
	defer f.Close()
	out, err := os.Create(p + ".new")
	if err != nil {
		return fmt.Errorf("couldn't open %q: %v", p+".new", err)
	}
	defer out.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, potCreationDatePrefix) {
			t = potcreation
		}
		if _, err := out.WriteString(t + "\n"); err != nil {
			return fmt.Errorf("couldn't write to %q: %v", p+".new", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error while reading %q: %v", p, err)
	}
	f.Close()
	out.Close()

	if err := os.Rename(p+".new", p); err != nil {
		return fmt.Errorf("couldn't rename %q to %q: %v", p+".new", p, err)
	}
	return nil
}
