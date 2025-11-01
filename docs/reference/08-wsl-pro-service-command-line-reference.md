---
myst:
  html_meta:
    "description lang=en":
      "Command line reference for Ubuntu Pro for WSL's WSL Pro Service."
---

# WSL Pro Service CLI

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

> See first: [Pro for WSL - WSL Pro Service](ref::up4w-wsl-pro-service)

## Usage

### User commands

#### wsl-pro-service

WSL Pro Service

##### Synopsis

WSL Pro Service connects Ubuntu Pro for WSL agent to your distro.

```
wsl-pro-service COMMAND [flags]
```

##### Options

```
  -c, --config string     configuration file path
  -h, --help              help for wsl-pro-service
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### wsl-pro-service completion

Generate the autocompletion script for the specified shell

##### Synopsis

Generate the autocompletion script for wsl-pro-service for the specified shell.
See each sub-command's help for details on how to use the generated script.


##### Options

```
  -h, --help   help for completion
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### wsl-pro-service completion bash

Generate the autocompletion script for bash

##### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(wsl-pro-service completion bash)

To load completions for every new session, execute once:

###### Linux:

	wsl-pro-service completion bash > /etc/bash_completion.d/wsl-pro-service

###### macOS:

	wsl-pro-service completion bash > $(brew --prefix)/etc/bash_completion.d/wsl-pro-service

You will need to start a new shell for this setup to take effect.


```
wsl-pro-service completion bash
```

##### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### wsl-pro-service completion fish

Generate the autocompletion script for fish

##### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	wsl-pro-service completion fish | source

To load completions for every new session, execute once:

	wsl-pro-service completion fish > ~/.config/fish/completions/wsl-pro-service.fish

You will need to start a new shell for this setup to take effect.


```
wsl-pro-service completion fish [flags]
```

##### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### wsl-pro-service completion powershell

Generate the autocompletion script for powershell

##### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	wsl-pro-service completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
wsl-pro-service completion powershell [flags]
```

##### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### wsl-pro-service completion zsh

Generate the autocompletion script for zsh

##### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(wsl-pro-service completion zsh)

To load completions for every new session, execute once:

###### Linux:

	wsl-pro-service completion zsh > "${fpath[1]}/_wsl-pro-service"

###### macOS:

	wsl-pro-service completion zsh > $(brew --prefix)/share/zsh/site-functions/_wsl-pro-service

You will need to start a new shell for this setup to take effect.


```
wsl-pro-service completion zsh [flags]
```

##### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### wsl-pro-service version

Returns version of wsl-pro-service and exits

```
wsl-pro-service version [flags]
```

##### Options

```
  -h, --help   help for version
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

### Hidden commands

Those commands are hidden from help and should primarily be used by the system or for debugging.

