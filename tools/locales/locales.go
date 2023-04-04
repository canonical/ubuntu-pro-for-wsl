package locales

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
	"github.com/canonical/ubuntu-pro-for-windows/common/generators"
)

func usage(c Configuration) string {
	return fmt.Sprintf(`Usage of %s:

   create-po LOC [LOC...]
     Create new LOC.po file(s) in %q from pot file %q.
   update-po POT DIRECTORY PACKAGE
     Create/Update a pot file %q and refresh any existing po files in %q.
   generate-mo DOMAIN PODIR DIRECTORY
     Create .mo files for any .po in %q in an structured hierarchy in %q.
`,
		os.Args[0],             // Usage of
		c.LocaleDir, c.PotFile, // create-po
		c.PotFile, c.LocaleDir, // update-po
		c.LocaleDir, c.MoDir, // generate-mo
	)
}

type Configuration struct {
	// Domain is the name of the TEXTDOMAIN to use
	Domain string

	// RootDir is the directory containing the entire module
	RootDir string

	// PotFile is the path to the Portable Object Template (POT) file.
	PotFile string

	// LocaleDir is the directory where all the Portable Object (PO) files are located.
	LocaleDir string

	// MoDir is the directory where all the Machine Object (MO) files are located.
	// It'll be appended to the install dir.
	MoDir string
}

func Generate(verb string, c Configuration, locales ...string) {
	if err := c.validate(); err != nil {
		log.Fatalf("Error in configuration: %v", err)
	}

	_, caller, _, ok := runtime.Caller(1)
	if !ok {
		log.Fatalf("Error fixing paths: could not get caller file")
	}
	dir := filepath.Dir(caller)
	c.prependPaths(dir)

	switch verb {
	case "help":
		fmt.Printf(usage(c))
		return
	case "create-po":
		if generators.InstallOnlyMode() {
			return
		}
		if err := createPo(c.PotFile, c.LocaleDir, locales); err != nil {
			log.Fatalf("Error when creating po files: %v", err)
		}

	case "update-po":
		if generators.InstallOnlyMode() {
			return
		}
		if err := updatePo(c.PotFile, c.LocaleDir, c.RootDir); err != nil {
			log.Fatalf("Error when updating po files: %v", err)
		}

	case "generate-mo":
		if err := generateMo(c.LocaleDir, filepath.Join(generators.DestDirectory(c.MoDir), "usr", "share")); err != nil {
			log.Fatalf("Error when generating mo files: %v", err)
		}
	default:
		log.Fatalf(usage(c))
	}
}

// validate makes a few safety checks on the configuration object.
func (c Configuration) validate() (err error) {
	if len(c.RootDir) == 0 {
		err = errors.Join(err, errors.New("configuration parameter RootDir is empty"))
	}
	if len(c.Domain) == 0 {
		err = errors.Join(err, errors.New("configuration parameter Domain is empty"))
	}
	if len(c.PotFile) == 0 {
		err = errors.Join(err, errors.New("configuration parameter PotFile is empty"))
	}
	if len(c.LocaleDir) == 0 {
		err = errors.Join(err, errors.New("configuration parameter LocaleDir is empty"))
	}
	if len(c.MoDir) == 0 {
		err = errors.Join(err, errors.New("configuration parameter MoDir is empty"))
	}
	return err
}

// prependPaths takes any relative paths in the Configuration and makes
// them absolute under dir.
func (c *Configuration) prependPaths(dir string) {
	if !filepath.IsAbs(c.RootDir) {
		c.RootDir = filepath.Join(dir, c.RootDir)
	}

	if !filepath.IsAbs(c.PotFile) {
		c.PotFile = filepath.Join(dir, c.PotFile)
	}

	if !filepath.IsAbs(c.LocaleDir) {
		c.LocaleDir = filepath.Join(dir, c.LocaleDir)
	}

	if !filepath.IsAbs(c.MoDir) {
		c.MoDir = filepath.Join(dir, c.MoDir)
	}
}

// createPo creates new po files.
func createPo(potfile, localeDir string, locales []string) error {
	if _, err := os.Stat(potfile); err != nil {
		return fmt.Errorf("%q can't be read: %v", potfile, err)
	}

	if len(locales) == 0 {
		return errors.New("create-po: No locales provided")
	}

	for _, loc := range locales {
		pofile := filepath.Join(localeDir, loc+".po")
		if _, err := os.Stat(pofile); err == nil {
			log.Printf("Skipping %q as it already exists. Please use update-po to refresh it or delete it first.", loc)
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
func updatePo(potfile, localeDir, rootDir string) error {
	if err := os.MkdirAll(localeDir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory for %q: %v", localeDir, err)
	}

	// Create pot file
	var files []string
	var err error

	err = errors.Join(err, filepath.WalkDir(rootDir, func(p string, de fs.DirEntry, err error) error {
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

		rel, err := filepath.Rel(rootDir, p)
		if err != nil {
			return fmt.Errorf("path %q cannot be made relative to %q", p, rootDir)
		}

		files = append(files, rel)
		return nil
	}))

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
		"-D", rootDir, "--output=" + potfile}, files...)
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
