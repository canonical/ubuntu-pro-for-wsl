# How to access the logs

At some point you may want to read the logs of Ubuntu Pro for WSL, most likely for debugging purposes. The agent and the service store their logs separately. This guide shows you where to find each of the logs.

## WSL Pro service

To access the logs of a specific distribution's WSL-Pro-Service, you must first launch the distribution and then query the journal:

```bash
journalctl -u wsl-pro.service
```

For more information on using the journal, you can check out its man page with `man journalctl` or [online](https://man7.org/linux/man-pages/man1/journalctl.1.html).

These logs may be insufficient for proper debugging, so you may be interested in looking at the agent's logs as well.

## Windows agent

Follow these steps to access the logs for the Windows agent.

1. Go to your home directory
   - Open the file explorer
   - Write `%USERPROFILE%` at the address
2. In the home directory, find the `.ubuntupro` directory and double-click on it.
2. In the `.ubuntupto` folder, find file `log` and open it with any text editor.
   - This file contains the logs sorted with the oldest entries at the top and the newest at the bottom.
