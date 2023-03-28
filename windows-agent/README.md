# Windows agent

The Windows Agent is the component that runs on the host Windows machine.

## Usage

### User commands

#### ubuntu-pro-agent

Ubuntu Pro for Windows agent

##### Synopsis

Ubuntu Pro for Windows agent for managing your pro-enabled distro. SAMPLE TEXT TO TRIGGER THE CI.

```
ubuntu-pro-agent COMMAND [flags]
```

##### Options

```
  -h, --help              help for ubuntu-pro-agent
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent completion

Generate the autocompletion script for the specified shell

##### Synopsis

Generate the autocompletion script for ubuntu-pro-agent for the specified shell.
See each sub-command's help for details on how to use the generated script.


##### Options

```
  -h, --help   help for completion
```

##### Options inherited from parent commands

```
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent completion bash

Generate the autocompletion script for bash

##### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(ubuntu-pro-agent completion bash)

To load completions for every new session, execute once:

###### Linux:

	ubuntu-pro-agent completion bash > /etc/bash_completion.d/ubuntu-pro-agent

###### macOS:

	ubuntu-pro-agent completion bash > $(brew --prefix)/etc/bash_completion.d/ubuntu-pro-agent

You will need to start a new shell for this setup to take effect.


```
ubuntu-pro-agent completion bash
```

##### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent completion fish

Generate the autocompletion script for fish

##### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	ubuntu-pro-agent completion fish | source

To load completions for every new session, execute once:

	ubuntu-pro-agent completion fish > ~/.config/fish/completions/ubuntu-pro-agent.fish

You will need to start a new shell for this setup to take effect.


```
ubuntu-pro-agent completion fish [flags]
```

##### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent completion powershell

Generate the autocompletion script for powershell

##### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	ubuntu-pro-agent completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
ubuntu-pro-agent completion powershell [flags]
```

##### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent completion zsh

Generate the autocompletion script for zsh

##### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(ubuntu-pro-agent completion zsh); compdef _ubuntu-pro-agent ubuntu-pro-agent

To load completions for every new session, execute once:

###### Linux:

	ubuntu-pro-agent completion zsh > "${fpath[1]}/_ubuntu-pro-agent"

###### macOS:

	ubuntu-pro-agent completion zsh > $(brew --prefix)/share/zsh/site-functions/_ubuntu-pro-agent

You will need to start a new shell for this setup to take effect.


```
ubuntu-pro-agent completion zsh [flags]
```

##### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

##### Options inherited from parent commands

```
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent version

Returns version of agent and exits

```
ubuntu-pro-agent version [flags]
```

##### Options

```
  -h, --help   help for version
```

##### Options inherited from parent commands

```
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

### Hidden commands

Those commands are hidden from help and should primarily be used by the system or for debugging.

