---
myst:
  html_meta:
    "description lang=en":
      "For developers who are testing, debugging or developing the application."
---

# Access Ubuntu Pro for WSL logs for debugging

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

At some point you may want to read the UP4W logs, most likely for debugging purposes. The agent and the service store their logs separately. This guide shows you where to find each of the logs.

## Access the logs for the WSL Pro service

To access the logs of a specific distribution's WSL-Pro-Service, you must first launch the distribution and then query the journal:

```bash
journalctl -u wsl-pro.service
```

For more information on using the journal, you can check out its man page with `man journalctl` or [online](https://man7.org/linux/man-pages/man1/journalctl.1.html).

These logs may be insufficient for proper debugging, so you may be interested in looking at the agent's logs as well.

## Access the logs for the Windows Agent

To access the logs for the Windows Agent:

1. Go to your home directory
   - Open the file explorer
   - Write `%USERPROFILE%` at the address
2. In the home directory, find the `.ubuntupro` directory and double-click on it.
2. In the `.ubuntupro` folder, find file `log` and open it with any text editor.
   - This file contains the logs sorted with the oldest entries at the top and the newest at the bottom.
