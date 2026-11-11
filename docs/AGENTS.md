# Ubuntu Pro app documentation agent guide

This guide applies to any work under `docs/`. Repository-level, code-specific instructions can be found in `../AGENTS.md`.

## What this documentation is

The Ubuntu Pro app documentation is a Sphinx project that uses MyST Markdown, `canonical-sphinx`, and the extensions listed in `requirements.txt`. The docs are hosted using Read the Docs, which builds based on `.readthedocs.yaml`.

The documentation includes information about the Ubuntu distribution for the WSL platform, as well as the Ubuntu Pro app (formerly Ubuntu Pro for WSL), which currently supports Windows and Ubuntu on WSL.

The content follows the [Diátaxis](https://diataxis.fr/) framework:

- `tutorials/` contains learning-oriented, end-to-end lessons
- `howto/` contains task-oriented procedures
- `explanation/` contains concepts, architecture, and background
- `reference/` contains factual descriptions of commands, features, configuration, and terminology

Supporting content lives in:

- `assets/` and the per-section `assets/` folders for screenshots and other static images
- `includes/` for shared include snippets, such as `pro_content_notice.txt` and `dev_docs_notice.txt`
- `redirects.txt` for redirects of moved or renamed pages
- `internal/` for development documentation that is not published on the website, such as architectural decision records and coding standards
- `_dev/`, `_static/`, and `_templates/` for build tooling and templates; these are not user-facing content and should not be edited as page content

Key files include:

- `index.md` for the landing page and top-level navigation
- `howto/index.md`, `explanation/index.md`, `reference/index.md`, and `tutorials/index.md` landing pages for section navigation
- `conf.py` for Sphinx extensions, redirects, and build hooks
- `Makefile` and `requirements.txt` for local builds and checks
- `.custom_wordlist.txt` and `.wordlist.txt` for accepted technical terms

## Generated content

Do not directly edit generated reference pages:

- `reference/07-windows-agent-command-line-reference.md` is generated from the `agent`'s Cobra command definitions by `go generate -tags tools ./windows-agent/generate`
- `reference/08-wsl-pro-service-command-line-reference.md` is generated from `wsl-pro-service`'s Cobra command definitions by `go generate -tags tools ./wsl-pro-service/generate`

## Build and check

Run documentation commands from the repository root:

```text
make -C docs install
make -C docs run
make -C docs html
make -C docs lint-md
make -C docs vale
make -C docs spelling
make -C docs linkcheck
make -C docs pa11y
make -C docs pdf
```

`install` creates `docs/.venv` and requires the system `python3-venv` package. Use `run` for an interactive preview at `http://127.0.0.1:8000`.

Use `CHECK_PATH` to restrict Vale-based checks while iterating:

```text
make -C docs vale CHECK_PATH=howto/set-up-up4w.md
make -C docs spelling CHECK_PATH=howto/set-up-up4w.md
```

Run `html` after navigation, configuration, MyST syntax, or cross-reference changes. Run `linkcheck` when changing links or moving pages. Run `pa11y` for changes to templates, layout, or other accessibility-sensitive content.

The `pdf` target builds a PDF export; it requires additional system packages that you can install with `make -C docs pdf-prep`.

## Writing conventions

### Language

- Use US English, active voice, and second person where practical
- Expand uncommon acronyms on first use
- Use descriptive link text instead of phrases such as "click here"
- Avoid filler such as "simply", "just", "easy", "obviously", and "basically"
- Do not use emojis

Do not make cosmetic rewrites or synonym substitutions unless the task identifies a concrete accuracy, consistency, or clarity problem.

### Application name substitution

The Ubuntu Pro application's name is defined once in `conf.py` (`APPLICATION_NAME`) and exposed to
all pages as the MyST substitution `application_name`. Renaming the application is a one-line change
there; do not hard-code the name in page content.

- Write `{{ application_name }}` whenever the text refers to the application. The variable holds
only the product name, so follow it freely with "app" or "application" to disambiguate from the
Ubuntu Pro subscription service, given they are currently homonymous. For example:
`the {{ application_name }} app` or `the {{ application_name }} application`, but never just 
`{{ application_name }}`, due the inherent ambiguity.
- Do not use the substitution for the Ubuntu Pro subscription service itself (Pro tokens, ESM,
attachment). Write that literally as "Ubuntu Pro". The variable is a semantic marker for the
application, not a mechanical replacement of the string.
- When a sentence could be read as referring to the Ubuntu Pro subscription (for example near "Pro
token" or "attach"), prefer the disambiguated form "the Ubuntu Pro app" or `` {term}`Ubuntu Pro app`
``.

Limitations to consider:

- Substitutions are not expanded inside inline roles, such as `` {guilabel}`...` ``, nor inside
other substitution values. For user-interface strings, define a substitution whose value contains
the markup with the name spelled out, for example `"download_button": "{guilabel}`Download Ubuntu
Pro`"`, and reference it as `{{ download_button }}`.
- Substitutions are not expanded inside fenced code blocks or `{terminal}` blocks. This is
intentional: commands, file contents, and program output must keep their real strings (registry
values, installed app names, Start Menu entries).
- Substitutions are not expanded in front matter, so `myst.html_meta` descriptions cannot use them.
New pages must still add the description front matter by hand.

Where the substitution is intentionally not used:

- Historical mentions: "formerly Ubuntu Pro for WSL" introductions, version-conditional notes, and
the glossary entries that define the former name.
- References to the Ubuntu Pro subscription service (literal "Ubuntu Pro").
- Generated reference pages (`reference/07-*` and `reference/08-*`); if their wording must change,
update the generator templates in `windows-agent/generate` and `wsl-pro-service/generate`, not the
generated files.
- Code fences and `{terminal}` blocks, as above.

### Page structure

- Use kebab-case filenames for new hand-written pages
- Add `myst.html_meta` description front matter to new user-facing pages
- Use one level-one heading per page and sentence case for headings
- Do not skip heading levels, and introduce a section before adding subsections
- Add a unique MyST target before the level-one heading when a page will be cross-referenced; prefix the label with the section name: `howto::` for how-to guides, `tut::` for tutorials, `ref::` for references, `exp::` for explanations, and `dev::` for developer documentation
- Keep tutorials, how-to guides, explanations, and references focused on their respective Diátaxis purpose

Use the same opening structure as current user-facing pages:

```text
---
myst:
  html_meta:
    "description lang=en":
      "Describe the page for search results."
---

(howto::page-label)=
# Page title
```

Optionally add a `relatedlinks` front matter entry to surface relevant external resources next to the page, using the same URL notation as existing pages:

```text
---
relatedlinks: "[Download&#32;Pro&#32;for&#32;WSL](https://www.ubuntu.com/desktop/wsl)"
---
```

When adding a page, include it in the nearest `index.md` `toctree`. Update the top-level `index.md` only when the page belongs in the landing-page navigation. Section indexes use `:titlesonly:` `toctrees`.

### MyST and Markdown

#### Roles and terms

- Use existing MyST roles such as `{term}`, `{ref}`, `{guilabel}`, `{kbd}`, and `{menuselection}` when they fit
- Use `{term}` only for terms defined in `reference/glossary.md`

#### Links

- Prefer `{ref}` for an existing labeled internal target
- In content pages, use relative Markdown links when no labeled target exists
- Do not hard-code the published documentation URL for an internal link

#### Code and formatting

- Use triple-backtick MyST directive fences, matching the syntax already used in this project
- Use `{terminal}` for any block that represents a command typed at a prompt, whether or not it shows output; use `text`, `ini`, `yaml`, or another appropriate language fence only for file content, configuration, logs, and scripts meant to be saved rather than typed interactively
- In `{terminal}` blocks, set `:prompt: PS>` for PowerShell and `:prompt: $` for Ubuntu/bash (use a directory-aware prompt such as `:prompt: ~/project$` when the working directory is meaningful to the example); add `:copy:` so only the command is copyable; use `:output-only:` for blocks that show command output with no prompt; split a block containing multiple commands into consecutive `{terminal}` blocks, which merge visually
- Put inline commands, file paths, option names, and literal values in backticks
- Use admonitions sparingly
- Include `includes/pro_content_notice.txt` with the existing `{include}` pattern on pages that document Ubuntu Pro features, and `includes/dev_docs_notice.txt` on developer doc pages

#### Assets

- Give every image meaningful alternative text
- Keep image paths under the relevant section's `assets/` folder

Add valid technical terms to `.custom_wordlist.txt` as plain entries instead of hiding spelling errors with inline markup. Keep case variants together and preserve the file's existing organization.

## Moving or deleting pages

Preserve inbound links whenever a published page moves or is removed. Add an entry to `redirects.txt`, update affected `toctrees` and cross-references, and check the resulting links with `make -C docs linkcheck`.

## Scope discipline

Keep documentation changes tied to the requested behavior or identified problem. Prefer official upstream documentation and man pages as sources, and do not reproduce large sections of external content.
