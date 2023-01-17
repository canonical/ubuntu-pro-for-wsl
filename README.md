# ubuntu-pro-for-windows

Ubuntu Pro for Windows is a set of applications to manage Ubuntu WSL instances to allow to:

* grants ‘pro-enabled’ status to any Ubuntu instance on the device,
* orchestrates instances for Landscape,
* manages instance states (spin up/down to apply policies/patches).

It bridges Ubuntu WSL instances to Ubuntu Pro.

## System Components

The system consists of the following components:

* A MS Windows application made of an agent with its user interface.
* An interface between the agent and Ubuntu Pro to handle the transactions with the contract server.
* An interface between the agent and Landscape to manage the WSL instances from Landscape.
* A WSL management API. This interface controls the lifecycle of the WSL instances, like provisioning, updates, and starting or stopping the WSL instances.
* A ubuntu-wsl service and the associated API. This interface controls the Pro and Landscape status between the agent running on Windows and the WSL instance.
* cloud-init is used to customise the images on first boot or to reconfigure an image.
