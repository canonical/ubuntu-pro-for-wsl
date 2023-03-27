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
	"runtime"
	"strings"

	"github.com/canonical/ubuntu-pro-for-windows/common/autocompletiondocumentation/internal/generators"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const usage = `Usage of %s:
   help
     Print this message and exit   
   completion DIRECTORY
     Create completions files in a structured hierarchy in DIRECTORY.
   man DIRECTORY
     Create man pages files in a structured hierarchy in DIRECTORY.
   update-readme
     Update repository README with commands.
   update-doc-cli-ref
	Update repository doc with commands.
`

// App encapsulate commands and options of a CLI.
type App interface {
	RootCmd() cobra.Command
}

// Configuration is a set of options for the paths where the generated documentation
// is to be stored.
type Configuration struct {
	// ReadmePath is the path to the REAMDE file to be updated
	// If the path is relative, it'll be computed from the dir the caller is in.
	ReadmePath string

	// ReadmePath is the path to the documentation file to be updated
	// If the path is relative, it'll be computed from the dir the caller is in.
	DocsPath string

	// ManPath is the directory where the man page will be stored in
	// It'll be appended to $GENERATE_ONLY_INSTALL_TO_DESTDIR if available, otherwise to
	// argv[2]
	ManPath string

	// CompletionPath is the directory where the autocompletion file will be stored in.
	// It'll be appended to $GENERATE_ONLY_INSTALL_TO_DESTDIR if available, otherwise to
	// argv[2]
	CompletionPath string
}

// Generate generates the autocompletion and documentation for the module.
func Generate(argv []string, config Configuration, apps ...App) {
	if len(argv) < 2 {
		log.Fatalf(usage, argv[0])
	}

	if err := config.validate(); err != nil {
		log.Fatalf("Wrong config: %v", err)
	}

	var commands []cobra.Command
	for _, a := range apps {
		commands = append(commands, a.RootCmd())
	}

	switch argv[1] {
	case "completion":
		if len(argv) < 3 {
			log.Fatalf(usage, argv[0])
		}
		dir := filepath.Join(generators.DestDirectory(argv[2]), config.CompletionPath)
		genCompletions(commands, dir)
	case "man":
		if len(argv) < 3 {
			log.Fatalf(usage, argv[0])
		}
		dir := filepath.Join(generators.DestDirectory(argv[2]), config.ManPath)
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
		updateFromCmd(commands, filepath.Join("doc", config.DocsPath))
	case "help":
		log.Printf(usage, argv[0])
		return
	default:
		log.Fatalf(usage, argv[0])
	}
}

// validate makes a few safety checks on the configuration object.
func (c Configuration) validate() (err error) {
	if len(c.ReadmePath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter ReadmePath is empty"))
	}
	if len(c.DocsPath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter DocsPath is empty"))
	}
	if len(c.ManPath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter ManPath is empty"))
	}
	if len(c.CompletionPath) == 0 {
		err = errors.Join(err, errors.New("configuration parameter CompletionPath is empty"))
	}
	return err
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
		cmd := cmd
		// Run ExecuteC to install completion and help commands
		_, _ = cmd.ExecuteC()
		opts := doc.GenManTreeOptions{
			Header: &doc.GenManHeader{
				Title: fmt.Sprintf("ADSys: %s", cmd.Name()),
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
	if !filepath.IsAbs(targetFile) {
		_, current, _, ok := runtime.Caller(2)
		if !ok {
			log.Fatal("Couldn't find current file name")
		}

		targetFile = filepath.Join(filepath.Dir(current), targetFile)
	}

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
		cmd := cmd
		// Run ExecuteC to install completion and help commands
		_, _ = cmd.ExecuteC()
		user = append(user, cmd)
		user = append(user, collectSubCmds(cmd, false /* selectHidden */, false /* parentWasHidden */)...)
	}

	for _, cmd := range cmds {
		cmd := cmd
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
			cmd := cmd
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
