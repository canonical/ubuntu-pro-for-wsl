# Windows agent

The Windows Agent is the component that runs on the host Windows machine.

## Usage

### User commands

#### ubuntu-pro-agent.exe

Ubuntu Pro Windows agent

##### Synopsis

Ubuntu Pro Windows agent manages Ubuntu instances on WSL.

```
ubuntu-pro-agent.exe COMMAND [flags]
```

##### Options

```
  -c, --config string     configuration file path
  -h, --help              help for ubuntu-pro-agent.exe
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent.exe clean

Removes all the agent's data and exits

```
ubuntu-pro-agent.exe clean [flags]
```

##### Options

```
  -h, --help   help for clean
```

##### Options inherited from parent commands

```
  -c, --config string     configuration file path
  -v, --verbosity count   issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output
```

#### ubuntu-pro-agent.exe completion

Generate the autocompletion script for the specified shell

##### Synopsis

Generate the autocompletion script for ubuntu-pro-agent.exe for the specified shell.
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

#### ubuntu-pro-agent.exe completion bash

Generate the autocompletion script for bash

##### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(ubuntu-pro-agent.exe completion bash)

To load completions for every new session, execute once:

###### Linux:

	ubuntu-pro-agent.exe completion bash > /etc/bash_completion.d/ubuntu-pro-agent.exe

###### macOS:

	ubuntu-pro-agent.exe completion bash > $(brew --prefix)/etc/bash_completion.d/ubuntu-pro-agent.exe

You will need to start a new shell for this setup to take effect.


```
ubuntu-pro-agent.exe completion bash
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

#### ubuntu-pro-agent.exe completion fish

Generate the autocompletion script for fish

##### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	ubuntu-pro-agent.exe completion fish | source

To load completions for every new session, execute once:

	ubuntu-pro-agent.exe completion fish > ~/.config/fish/completions/ubuntu-pro-agent.exe.fish

You will need to start a new shell for this setup to take effect.


```
ubuntu-pro-agent.exe completion fish [flags]
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

#### ubuntu-pro-agent.exe completion powershell

Generate the autocompletion script for powershell

##### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	ubuntu-pro-agent.exe completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
ubuntu-pro-agent.exe completion powershell [flags]
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

#### ubuntu-pro-agent.exe completion zsh

Generate the autocompletion script for zsh

##### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(ubuntu-pro-agent.exe completion zsh)

To load completions for every new session, execute once:

###### Linux:

	ubuntu-pro-agent.exe completion zsh > "${fpath[1]}/_ubuntu-pro-agent.exe"

###### macOS:

	ubuntu-pro-agent.exe completion zsh > $(brew --prefix)/share/zsh/site-functions/_ubuntu-pro-agent.exe

You will need to start a new shell for this setup to take effect.


```
ubuntu-pro-agent.exe completion zsh [flags]
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

#### ubuntu-pro-agent.exe version

Returns version of agent and exits

```
ubuntu-pro-agent.exe version [flags]
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

