# Ubuntu Pro for WSL (UP4W)
```{warning}
This documentation describes a future release of Ubuntu Pro for WSL. Ubuntu Pro for WSL is not yet generally available in the Microsoft Store.
```



Ubuntu Pro for WSL (UP4W) is a desktop application that facilitates access to your [Ubuntu Pro]( https://ubuntu.com/pro) subscription benefits in the context of [Ubuntu WSL](https://canonical-ubuntu-wsl.readthedocs-hosted.com/en/latest/).

When you install UP4W on your Windows host:

- If you attach your Ubuntu Pro token to UP4W on the host, UP4W will automatically add it to the Ubuntu Pro client on any Ubuntu WSL instance on the host as well, so that all of your instances are added to your Ubuntu Pro subscription.
- If you configure the Landscape client component of UP4W on the host, UP4W will automatically configure the Landscape client on any Ubuntu WSL instance on the host as well, so that both your host and the instances on the host are registered with Landscape. You can then use Landscape not just to manage the instances but also to create and provision new instances on the host.

Pro-attaching and Landscape-enrolling an Ubuntu instance is not difficult, but when your Ubuntu instance fleet is large it can get tedious.  UP4W takes advantage of the WSL setting to make it a breeze -- regardless of the size of your fleet.

UP4W is of value to system administrators, corporate security teams, and desktop users.

## Project and community

UP4W is a member of the Ubuntu family. It’s an open-source project that warmly welcomes community contributions, suggestions, fixes and constructive feedback. Check out our [contribution page](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/CONTRIBUTING.md) on GitHub in order to bring ideas, report bugs, participate in discussions, and much more!

Thinking about using UP4W for your next project? Get in touch!

```{toctree}
:hidden:
:titlesonly:

UP4W <self>
Tutorial </tutorial/index>
How-to guides </howto/index>
Reference </reference/index>
UP4W Dev </dev/index>
```
