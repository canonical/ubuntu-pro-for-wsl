# Coding Standards for Go

## Style guide

- Prefer small, explicit functions over clever abstractions.
- Document exported symbols with complete sentences, starting with its name.
- Validate required inputs and dependencies early, then return immediately on invalid state.
- Strive for making invalid states non-representable to avoid spreading validation everywhere.
- Keep control flow flat: prefer guard clauses and early returns over nested `else` blocks.
- Keep exported APIs narrow, beware of Hyrum's Law.
- Do not widen visibility just to satisfy tests; use the existing `export_test.go` pattern instead.
- Prefer descriptive names over abbreviations, except for well-known abbreviations (`URL`, `JSON`, `HTTP`) and conventional short receiver names.
- Short variable names are preferred when their scope is small and obvious.
- Keep constructors and options pragmatic:
  - Use a constructor when required dependencies or invariants must be enforced.
  - Do not introduce option plumbing for a type that can be configured more clearly with direct fields or simple arguments.
- Prefer package-level sentinel errors only for conditions callers need to branch on.
- Keep comments for non-obvious invariants, edge cases, or intent; avoid comments that restate the code.
- Avoid unnecessary helper extraction. A short local block is usually better than a helper that obscures the main path.
- At a given layer, either log an error or return it with context. Avoid duplicating the same failure message in both places unless each layer adds distinct operational value.
- Keep empty lines separating logical blocks, making lines closely related standing out as a group.

### Example of good code style

```go
// normalizeLandscapeConfig ensures that the landscape config has the expected computer_title and SSL certificate path
// transformed in a Linux path.
func normalizeLandscapeConfig(ctx context.Context, s *System, iniFile *ini.File) (modifiedLandscapeConfig string, didChange bool, err error) {
	clientSection, err := iniFile.GetSection("client")
	if err != nil {
		return "", false, err
	}

	// Add or refresh computer title
	distroName, err := s.WslDistroName(ctx)
	if err != nil {
		return "", false, err
	}
	titleChanged := false
	oldComputerTitle, err := clientSection.GetKey("computer_title")
	if err != nil {
		if _, err = clientSection.NewKey("computer_title", distroName); err != nil {
			return "", false, err
		}
		titleChanged = true
	} else if oldComputerTitle.String() != distroName {
		oldComputerTitle.SetValue(distroName)
		titleChanged = true
	}

	// Refresh SSL certificate path if any
	certChanged, err := overrideSSLCertificate(ctx, s, clientSection)
	if err != nil {
		return "", false, fmt.Errorf("could not override SSL certificate path: %v", err)
	}

	// Return the modified config as a string.
	w := &bytes.Buffer{}
	if _, err := iniFile.WriteTo(w); err != nil {
		return "", false, fmt.Errorf("could not regenerate modified config: %v", err)
	}

	didChange = titleChanged || certChanged
	return w.String(), didChange, nil
}
```

## Error handling

- Use `decoreate.OnError` to add context to errors returned from functions at a single location.
- Prefer `errors.New` for static sentinel errors and declare them as `var ErrSomething = errors.New("...")`.
- Return errors wrapped with `%v`; only use `%w` only when callers must match it with `errors.Is`/`errors.As`.
- If the underlying error is only being included for human consumption, use `%v` instead of `%w`.
- Prefer one meaningful layer of context at the abstraction boundary that changes what the operation means to the caller.
- When a caller needs to match a domain-specific condition and still retain extra detail, prefer `errors.Join` with a sentinel error.
- Use lowercase error messages without trailing punctuation.
- Avoid exporting foreign implementation details by default. Expose them only when callers have a concrete need to branch on them.
- Do not automatically return sentinel errors from underlying libraries, unless the caller is expected to match them. Use `fmt.Errorf("<message>: %v", err)` instead.
 
 ### Example of good error handling
 
```go
// ProStatus returns whether this distro is pro-attached.
func (s System) ProStatus(ctx context.Context) (attached bool, err error) {
	defer decorate.OnError(&err, "pro status")

	cmd := s.backend.ProExecutable(ctx, "status", "--format=json")
	out, err := runCommand(cmd)
	if err != nil {
		return false, err
	}

	var attachedStatus struct {
		Attached bool
	}
	if err = json.Unmarshal(out, &attachedStatus); err != nil {
		return false, fmt.Errorf("could not parse output: %v. Output: %s", err, string(out))
	}

	return attachedStatus.Attached, nil
}
```

## Testing

- Prefer table-driven tests keyed by name in a map (`testcases := map[string]struct{...} { ...}`),
  where each element holds a particular test case arguments ordered to facilitate grasping the
  differences between sub-tests, preserving the test body similar in implementation. Iterate over
  that map as `for name, tc := range testscases { ... }` and define sub-tests for each case:
  `t.Run(name, func(t *testing.T) { ... })`.
- Table-driven testing exemption is allowed when no more than one case exists or is foreseeable or
  sub-test candidates are drastically different in implementation. When the behaviour under test has
  no injectable failure mode — nothing can break, no invalid input reachable, no error the code can
  return — or when only a single code path can be exercised, write the single success case as a
  plain test. Do not manufacture a one-entry table, and do not widen visibility or add seams that
  exist only to fabricate a failure. Delete cases whose failure mode a redesign has removed, rather
  than keeping them alive against a contrived error.
- If a test function cannot have sub-tests, it must have a comment explaining what it does and why
  it's useful to prevent regression.
- Strive to call `t.Parallel()` inside the sub-tests, comment in the test if parallelization is not possible.
- Keep test cases deterministic and self-contained; avoid hidden shared mutable state between cases.
- Prefix assertion messages with "Setup: " when a setup step fails before the actual test assertion.
- Use golden files when expected output is large, structured, or hard to maintain inline.
- Use shared helpers from `common/testutils/golden.go`:
  - `LoadWithUpdateFromGolden` for plain text expectations.
  - `LoadWithUpdateFromGoldenYAML` for structured/YAML expectations.
- Update golden files intentionally with `TESTS_UPDATE_GOLDEN=yes`, then commit the updated golden artifacts in the same PR.
- Prefer explicit, stable assertions over ad hoc string-contains checks.

### Example of good test

```go
tests := map[string]struct {
    input string
    want  string
}{
    "simple case": {input: "x", want: "y"},
}

for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        got := run(tc.input)
        want := testutils.LoadWithUpdateFromGolden(t, got)
        require.Equal(t, want, got)
    })
}
```

## Linting

The project uses `golangci-lint` with config at `.golangci.yaml` in the repo root. Run it as:

```
golangci-lint run <module>/... --fix
```

Where `<module>` is the Go module directory being changed (e.g. `windows-agent`, `common`,
`wsl-pro-service`). All findings must be resolved before committing — a lint error is as blocking
as a failing test.

Key rules to know before writing code:

- `ioutil.*` is forbidden — use `os` or `io` equivalents (`forbidigo`).
- Bare `print`/`println` are forbidden — use the project logger (`forbidigo`).
- All exported symbols must have doc comments ending with a period (`godot`).
- Error type names must end in `Error` or implement `error` as `*T` (`errname`).
- Test helpers must call `t.Helper()` (`thelper`).
- Parallel sub-tests must call `t.Parallel()` (`tparallel`).
- Use the correct `testify` assertion variant (`testifylint`).
- Never use naked type assertions — always check the `ok` form or use a type switch (`forcetypeassert`).

