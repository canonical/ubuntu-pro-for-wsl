// Package locales generates pot, po, and mo files to enable i18n.
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
	"strings"

	"github.com/canonical/ubuntu-pro-for-wsl/generate/internal/generators"
	"gopkg.in/yaml.v3"
)

type configuration struct {
	// Domain is the name of the TEXTDOMAIN to use
	Domain string `yaml:"domain"`

	// PotFile is the path to the Portable Object Template (POT) file.
	PotFile string `yaml:"pot-file"`

	// LocaleDir is the directory where all the Portable Object (PO) files are located.
	LocaleDir string `yaml:"locale-dir"`

	// MoDir is the directory where all the Machine Object (MO) files are located.
	// It'll be appended to the install dir.
	MoDir string `yaml:"mo-dir"`
}

// Main is the entrypoint for the locale generator.
func Main() {
	if len(os.Args) < 2 {
		log.Fatalf("Too few arguments\n"+usage, os.Args[0])
	}

	verb := os.Args[1]

	if verb == "help" {
		fmt.Printf(usage, os.Args[0])
		return
	}

	if len(os.Args) < 3 {
		log.Fatalf("Too few arguments\n"+usage, os.Args[0])
	}
	confPath := os.Args[2]
	extraArgs := os.Args[3:]

	config, projectRoot, err := parseConfiguration(confPath)
	if err != nil {
		log.Fatalf("%v", err)
	}

	generate(verb, config, projectRoot, extraArgs...)
}

const usage = `Usage:

	%s VERB [ARGS...]

VERB and ARGS:

	create-po CONFIG LOC [LOC...]
		Create new LOC.po file(s) in <CONFIG.locale-dir> from <CONFIG.pot-file>.
	update-po CONFIG
		Create/Update <CONFIG.pot-file> and refresh any existing po files in <CONFIG.locale-dir>.
	generate-mo CONFIG
		Create .mo files for any .po in <CONFIG.locale-dir> in an structured hierarchy in <CONFIG.mo-dir>.


CONFIG:

	It is the path to the configuration yaml file. It expects a yaml with the following pattern:
	---
	project-root: Root of the project. All other paths are relative to the project root.
	i18n:
	  domain:     The domain for i18n
	  pot-file:   The path to the pot file
	  locale-dir: The directory where po files are located
	  mo-dir:     The directory where mo files will be stored
	---
`

func parseConfiguration(confPath string) (c configuration, projectRoot string, err error) {
	raw, err := os.ReadFile(confPath)
	if err != nil {
		return c, projectRoot, fmt.Errorf("could not open config file: %v", err)
	}

	conf := struct {
		ProjectRoot string `yaml:"project-root"`
		I18n        configuration
	}{
		ProjectRoot: filepath.Dir(confPath),
	}

	if err := yaml.Unmarshal(raw, &conf); err != nil {
		return c, projectRoot, fmt.Errorf("could not parse config file: %v", err)
	}

	c = conf.I18n
	projectRoot = conf.ProjectRoot

	if err := c.validate(projectRoot); err != nil {
		return c, projectRoot, fmt.Errorf("invalid configuration: %v", err)
	}

	if err := c.fixPaths(confPath, &projectRoot); err != nil {
		return c, projectRoot, err
	}

	return c, projectRoot, nil
}

func generate(verb string, c configuration, projectRoot string, locales ...string) {
	switch verb {
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
		if err := updatePo(c.PotFile, c.LocaleDir, projectRoot); err != nil {
			log.Fatalf("Error when updating po files: %v", err)
		}

	case "generate-mo":
		if err := generateMo(c.Domain, c.LocaleDir, filepath.Join(generators.DestDirectory(c.MoDir), "usr", "share")); err != nil {
			log.Fatalf("Error when generating mo files: %v", err)
		}
	default:
		log.Fatalf("Invalid verb %q"+usage, verb, os.Args[0])
	}
}

// validate makes a few safety checks on the configuration object.
func (c configuration) validate(projectRoot string) (err error) {
	if len(projectRoot) == 0 {
		err = errors.Join(err, errors.New("configuration parameter project-root is empty"))
	}
	if len(c.Domain) == 0 {
		err = errors.Join(err, errors.New("configuration parameter domain is empty"))
	}
	if len(c.PotFile) == 0 {
		err = errors.Join(err, errors.New("configuration parameter pot-file is empty"))
	}
	if len(c.LocaleDir) == 0 {
		err = errors.Join(err, errors.New("configuration parameter locale-dir is empty"))
	}
	if len(c.MoDir) == 0 {
		err = errors.Join(err, errors.New("configuration parameter mo-dir is empty"))
	}
	return err
}

// fixPaths takes any relative paths in the Configuration and makes
// them absolute under dir.
func (c *configuration) fixPaths(confPath string, projectRoot *string) (err error) {
	if !filepath.IsAbs(*projectRoot) {
		// If project root is relative, make it relative to the config dir
		confDir := filepath.Dir(confPath)
		*projectRoot = filepath.Join(confDir, *projectRoot)

		if *projectRoot, err = filepath.Abs(*projectRoot); err != nil {
			return err
		}
	}

	// Make other paths relative to the project root
	for _, p := range []*string{&c.PotFile, &c.LocaleDir, &c.MoDir} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(*projectRoot, *p)
		}
		*p = filepath.Clean(*p)
	}

	return nil
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
	if err := os.MkdirAll(localeDir, 0750); err != nil {
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

// generateMo generates a locale directory structure with a mo for each po in localeDir.
func generateMo(textdomain, in, out string) error {
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
			"--output-file="+filepath.Join(outDir, textdomain+".mo"),
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
