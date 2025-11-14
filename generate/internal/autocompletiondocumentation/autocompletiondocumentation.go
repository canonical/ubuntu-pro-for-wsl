// Package autocompletiondocumentation generates a readme, man, autocompletion, cli refs
package autocompletiondocumentation

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/ubuntu-pro-for-wsl/generate/internal/generators"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"gopkg.in/yaml.v3"
)

const usage = `Usage:

   %s VERB CONFIG

VERB:

	help
		Print this message and exit.
	completion
		Create completions files in a structured hierarchy in CONFIG.completions.
	man
		Create man pages files in a structured hierarchy in CONFIG.man.
	update-readme
		Update CONFIG.readme with commands.
	update-doc-cli-ref
		Update CONFIG.docs with commands.

CONFIG:
	It is the path to the configuration yaml file. It expects a yaml with the following pattern:
	---
	project-root: Root of the project. All other paths are relative to the project root.
	docs:
	  completions: The directory where completion files will be stored
	  docs:        The path to the doc chapter to update
	  man:         The directory where man files will be stored
	  readme:      The path to the README to update
	---
`

// Configuration is a set of options for the paths where the generated documentation
// is to be stored.
type configuration struct {
	// ReadmePath is the path to the REAMDE file to be updated
	// If the path is relative, it'll be computed from the dir the caller is in.
	ReadmePath string `yaml:"readme"`

	// ReadmePath is the path to the documentation file to be updated
	// If the path is relative, it'll be computed from the dir the caller is in.
	DocsPath string `yaml:"docs"`

	// ManPath is the directory where the man page will be stored in
	// It'll be appended to $GENERATE_ONLY_INSTALL_TO_DESTDIR if available, otherwise to
	// argv[2]
	ManPath string `yaml:"man"`

	// CompletionPath is the directory where the autocompletion file will be stored in.
	// It'll be appended to $GENERATE_ONLY_INSTALL_TO_DESTDIR if available, otherwise to
	// argv[2]
	CompletionPath string `yaml:"completions"`
}

// Main is the entry point to the generator. It needs a closure to obtain the commands
// to generate documentation and autocompletion of.
func Main(getCommands func(module string) []cobra.Command) {
	if len(os.Args) < 2 {
		log.Fatalf("Too few arguments\n"+usage, os.Args[0])
	}

	verb := os.Args[1]

	if verb == "help" {
		fmt.Printf(usage, os.Args[0])
		return
	}

	if len(os.Args) != 3 {
		log.Fatalf("Wrong number of arguments\n"+usage, os.Args[0])
	}
	confPath := os.Args[2]

	config, projectRoot, err := parseConfiguration(confPath)
	if err != nil {
		log.Fatalf("%v", err)
	}

	generate(verb, config, getCommands(filepath.Base(projectRoot))...)
}

func parseConfiguration(confPath string) (c configuration, projectRoot string, err error) {
	raw, err := os.ReadFile(confPath)
	if err != nil {
		return c, projectRoot, fmt.Errorf("could not open config file: %v", err)
	}

	conf := struct {
		ProjectRoot string `yaml:"project-root"`
		Docs        configuration
	}{
		ProjectRoot: filepath.Dir(confPath), // By default, project root is where config.yaml is located
	}

	if err := yaml.Unmarshal(raw, &conf); err != nil {
		return c, projectRoot, fmt.Errorf("could not parse config file: %v", err)
	}

	projectRoot = conf.ProjectRoot
	docs := conf.Docs

	if err := docs.validate(); err != nil {
		return c, projectRoot, fmt.Errorf("invalid configuration: %v", err)
	}

	if projectRoot, err = docs.makePathsAbsolute(confPath, projectRoot); err != nil {
		return c, projectRoot, err
	}

	return docs, projectRoot, nil
}

// generate generates the autocompletion and documentation for the module.
func generate(verb string, config configuration, commands ...cobra.Command) {
	if err := config.validate(); err != nil {
		log.Fatalf("Wrong config: %v", err)
	}

	switch verb {
	case "completion":
		dir := generators.DestDirectory(config.CompletionPath)
		genCompletions(commands, dir)
	case "man":
		dir := generators.DestDirectory(config.ManPath)
		genManPages(commands, dir)
	case "update-readme":
		if generators.InstallOnlyMode() {
			return
		}
		updateFromCmd(commands, config.ReadmePath)
	case "update-doc-cli-ref":
		if generators.InstallOnlyMode() {
			return
		}
		updateFromCmd(commands, config.DocsPath)
	default:
		log.Fatalf(usage, os.Args[0])
	}
}

// validate makes a few safety checks on the configuration object.
func (c configuration) validate() (err error) {
	if len(c.ReadmePath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter readme is empty"))
	}
	if len(c.DocsPath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter docs is empty"))
	}
	if len(c.ManPath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter man is empty"))
	}
	if len(c.CompletionPath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter completion is empty"))
	}
	return err
}

// makePathsAbsolute takes any relative paths in the Configuration and makes
// them absolute. It also returns the project root as an absolute path.
func (c *configuration) makePathsAbsolute(confPath, projectRoot string) (absProjectRoot string, err error) {
	absProjectRoot = projectRoot
	if !filepath.IsAbs(absProjectRoot) {
		// If project root is relative, make it relative to the config dir
		confDir := filepath.Dir(confPath)
		absProjectRoot = filepath.Join(confDir, absProjectRoot)

		if absProjectRoot, err = filepath.Abs(absProjectRoot); err != nil {
			return absProjectRoot, err
		}
	}

	// Make other paths relative to the project root
	for _, p := range []*string{&c.CompletionPath, &c.DocsPath, &c.ManPath, &c.ReadmePath} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(absProjectRoot, *p)
		}
		*p = filepath.Clean(*p)
	}

	return absProjectRoot, nil
}

// genCompletions for bash and zsh directories.
func genCompletions(cmds []cobra.Command, dir string) {
	bashCompDir := filepath.Join(dir, "bash-completion", "completions")
	zshCompDir := filepath.Join(dir, "zsh", "site-functions")
	for _, d := range []string{bashCompDir, zshCompDir} {
		if err := generators.CleanDirectory(filepath.Dir(d)); err != nil {
			log.Fatalln(err)
		}
		if err := generators.CreateDirectory(d, 0755); err != nil {
			log.Fatalf("Couldn't create bash completion directory: %v", err)
		}
	}

	for _, cmd := range cmds {
		if err := cmd.GenBashCompletionFileV2(filepath.Join(bashCompDir, cmd.Name()), true); err != nil {
			log.Fatalf("Couldn't create bash completion for %s: %v", cmd.Name(), err)
		}
		if err := cmd.GenZshCompletionFile(filepath.Join(zshCompDir, cmd.Name())); err != nil {
			log.Fatalf("Couldn't create zsh completion for %s: %v", cmd.Name(), err)
		}
	}
}

func genManPages(cmds []cobra.Command, dir string) {
	manBaseDir := filepath.Join(dir, "man")
	if err := generators.CleanDirectory(manBaseDir); err != nil {
		log.Fatalln(err)
	}

	out := filepath.Join(manBaseDir, "man1")
	if err := generators.CreateDirectory(out, 0755); err != nil {
		log.Fatalf("Couldn't create man pages directory: %v", err)
	}

	for _, cmd := range cmds {
		// Run ExecuteC to install completion and help commands
		_, _ = cmd.ExecuteC()
		opts := doc.GenManTreeOptions{
			Header: &doc.GenManHeader{
				Title: fmt.Sprintf("Ubuntu Pro for WSL: %s", cmd.Name()),
			},
			Path: out,
		}
		if err := genManTreeFromOpts(&cmd, opts); err != nil {
			log.Fatalf("Couldn't generate man pages for %s: %v", cmd.Name(), err)
		}
	}
}

// updateFromCmd creates a file containing the detail of the commands
// the target filePath is relative to the root of the project.
func updateFromCmd(cmds []cobra.Command, targetFile string) {
	in, err := os.Open(targetFile)
	if err != nil {
		log.Fatalf("Couldn't open source readme file: %v", err)
	}
	defer in.Close()

	tmp, err := os.Create(targetFile + ".new")
	if err != nil {
		log.Fatalf("Couldn't create temporary readme file: %v", err)
	}
	defer tmp.Close()

	src := bufio.NewScanner(in)
	// Write header
	var usageFound bool
	const usageTarget = "## Usage"
	for src.Scan() {
		mustWriteLine(tmp, src.Text())
		if src.Text() == usageTarget {
			mustWriteLine(tmp, "")
			usageFound = true
			break
		}
	}
	if err := src.Err(); err != nil {
		log.Fatalf("Error when scanning source readme file: %v", err)
	}

	if !usageFound {
		tmp.Close()
		os.Remove(tmp.Name())
		log.Fatalf("Error when scanning source readme file: did not find usage header. To use this generator, create file %q with an introduction of your liking. Write an empty section titled %q so that the generator knows where to insert the command line usage", targetFile, usageTarget)
	}

	// Write markdown
	user, hidden := getCmdsAndHiddens(cmds)
	mustWriteLine(tmp, "### User commands\n")
	filterCommandMarkdown(user, tmp)
	mustWriteLine(tmp, "### Hidden commands\n")
	mustWriteLine(tmp, "Those commands are hidden from help and should primarily be used by the system or for debugging.\n")
	filterCommandMarkdown(hidden, tmp)

	// Write footer (skip previously generated content)
	skip := true
	for src.Scan() {
		if strings.HasPrefix(src.Text(), "## ") {
			skip = false
		}
		if skip {
			continue
		}

		mustWriteLine(tmp, src.Text())
	}
	if err := src.Err(); err != nil {
		log.Fatalf("Error when scanning source readme file: %v", err)
	}

	if err := in.Close(); err != nil {
		log.Fatalf("Couldn't close source Rreadme file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		log.Fatalf("Couldn't close temporary readme file: %v", err)
	}
	if err := os.Rename(targetFile+".new", targetFile); err != nil {
		log.Fatalf("Couldn't rename to destination readme file: %v", err)
	}
}

func mustWriteLine(w io.Writer, msg string) {
	if _, err := w.Write([]byte(msg + "\n")); err != nil {
		log.Fatalf("Couldn't write %s: %v", msg, err)
	}
}

// genManTreeFromOpts generates a man page for the command and all descendants.
// The pages are written to the opts.Path directory.
// This is a copy from cobra, but it will include Hidden commands.
func genManTreeFromOpts(cmd *cobra.Command, opts doc.GenManTreeOptions) error {
	header := opts.Header
	if header == nil {
		header = &doc.GenManHeader{}
	}
	for _, c := range cmd.Commands() {
		if (!c.IsAvailableCommand() && !c.Hidden) || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := genManTreeFromOpts(c, opts); err != nil {
			return err
		}
	}
	section := "1"
	if header.Section != "" {
		section = header.Section
	}

	separator := "_"
	if opts.CommandSeparator != "" {
		separator = opts.CommandSeparator
	}
	basename := strings.Replace(cmd.CommandPath(), " ", separator, -1)
	filename := filepath.Join(opts.Path, basename+"."+section)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	headerCopy := *header
	return doc.GenMan(cmd, &headerCopy, f)
}

func getCmdsAndHiddens(cmds []cobra.Command) (user []cobra.Command, hidden []cobra.Command) {
	for _, cmd := range cmds {
		// Run ExecuteC to install completion and help commands
		_, _ = cmd.ExecuteC()
		user = append(user, cmd)
		user = append(user, collectSubCmds(cmd, false /* selectHidden */, false /* parentWasHidden */)...)
	}

	for _, cmd := range cmds {
		// Run ExecuteC to install completion and help commands
		_, _ = cmd.ExecuteC()
		hidden = append(hidden, collectSubCmds(cmd, true /* selectHidden */, false /* parentWasHidden */)...)
	}

	return user, hidden
}

// collectSubCmds get recursiverly commands from a root one.
// It will filter hidden commands if selected, but will present children if needed.
func collectSubCmds(cmd cobra.Command, selectHidden, parentWasHidden bool) (cmds []cobra.Command) {
	for _, c := range cmd.Commands() {
		// Don’t collect command or children (hidden or not) of a hidden command
		if c.Name() == "help" || c.Hidden && !selectHidden {
			continue
		}
		// Add this command if matching request (hidden or non hidden collect).
		// Special case: if a parent is hidden, any children commands (hidden or not) will be selected.
		if (c.Hidden == selectHidden) || (selectHidden && parentWasHidden) {
			cmds = append(cmds, *c)
		}
		// Flip that we have a hidden parent
		currentOrParentHidden := parentWasHidden
		if c.Hidden {
			currentOrParentHidden = true
		}

		cmds = append(cmds, collectSubCmds(*c, selectHidden, currentOrParentHidden)...)
	}
	return cmds
}

// filterCommandMarkdown filters SEE ALSO and add subindentation for commands
// before writing to the writer.
func filterCommandMarkdown(cmds []cobra.Command, w io.Writer) {
	pr, pw := io.Pipe()

	go func() {
		for _, cmd := range cmds {
			if err := doc.GenMarkdown(&cmd, pw); err != nil {
				pw.CloseWithError(fmt.Errorf("couldn't generate markdown for %s: %v", cmd.Name(), err))
				return
			}
		}
		pw.Close()
	}()
	scanner := bufio.NewScanner(pr)
	var skip bool
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "### SEE ALSO") || strings.Contains(l, "Auto generated by") {
			skip = true
		}
		if strings.HasPrefix(l, "## ") {
			skip = false
		}
		if skip {
			continue
		}

		// Add 2 levels of subindentation
		if strings.HasPrefix(l, "##") {
			l = "##" + l
		}

		// Special case # Linux an # macOS in shell completion:
		if strings.HasPrefix(l, "# Linux") {
			continue
		} else if strings.HasPrefix(l, "# macOS") {
			l = " or:"
		}

		mustWriteLine(w, l)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Couldn't write generated markdown: %v", err)
	}
}
