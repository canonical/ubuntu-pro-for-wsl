---
myst:
  html_meta:
    "description lang=en":
      "Scale up your deployment of a custom rootfs using Ubuntu Pro for WSL's Landscape integration."
---

# Deploy a custom rootfs to multiple Windows machines with Ubuntu Pro for WSL and the Landscape API

```{include} ../includes/pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

This guide shows how to use the Landscape API to automate the deployment of a custom rootfs across multiple Windows machines.
Scaled deployment is enabled by Ubuntu Pro for WSL, which ensures that Ubuntu WSL instances on Windows machines are automatically registered with Landscape.
Cloud-init is used for initialisation and final configuration of the instances.
To follow the steps outlined in this guide you can use either:
- Bash scripting on Linux, or
- PowerShell scripting on Windows

## Prerequisites

- A running self-hosted Landscape server version `24.10~beta.5` or later.
- Multiple Windows machines [already registered with Landscape](howto::config-landscape) using Pro for WSL.
- Make sure you have installed `curl` and `jq`, if you're following this guide using Bash.
- Familiarity with Bash and/or PowerShell.

```{note}
You need PowerShell 7.0 or later to follow this guide. Some commands will fail on PowerShell 5.
You can check your PowerShell version by running the command `$PSVersionTable`.
```

## Prepare the environment

Export the following environment variables, modifying the values that are assigned as needed.
They will be used in subsequent commands.

`````{tabs}

````{group-tab} Bash
```bash
# Credentials to authenticate the API requests
export LANDSCAPE_USER_EMAIL=admin@mib.com
export LANDSCAPE_USER_PASSWORD=mib
export LANDSCAPE_URL=https://landscape.mib.com

# The URL of the custom rootfs to be deployed
export ROOTFS_URL="http://landscape.mib.com:9009/ubuntu-24.04-custom.tar.gz"

# The list of IDs of the different Windows machines on which we are going to deploy WSL instances
export PARENT_COMPUTER_IDS=(26 30 31)

# The name of the WSL instance to be created
export COMPUTER_NAME=Carbonizer

# Path to the cloud-config file whose contents will be used to initialize the WSL instances
export CLOUD_INIT_FILE="~/Downloads/init.yaml"
```

````

````{group-tab} PowerShell
```powershell
# Credentials to authenticate the API requests
$LANDSCAPE_USER_EMAIL="admin@mib.com"
$LANDSCAPE_USER_PASSWORD="mib"
$LANDSCAPE_URL="https://landscape.mib.com"

# The URL of the custom rootfs to be deployed
$ROOTFS_URL="http://landscape.mib.com:9009/ubuntu-24.04-custom.tar.gz"

# The list of IDs of the different Windows machines on which we are going to deploy WSL instances
$PARENT_COMPUTER_IDS=@(26, 30, 31)

# The name of the WSL instance to be created
$COMPUTER_NAME="Carbonizer"

# Path to the cloud-config file whose contents will be used to initialize the WSL instances
$CLOUD_INIT_FILE="~\Downloads\init.yaml"
```

````

`````

```{admonition} Computer IDs
:class: note

The `PARENT_COMPUTER_IDS` environment variable contains a list of IDs internally assigned to
Windows machines already registered to Landscape. The values used in this guide are examples,
and you can get IDs for your machines in the Landscape dashboard or through the
Landscape REST API.
```

```{admonition} Image server
:class: tip

In our example, a [custom image](howto::custom-distro) `ubuntu-24.04-custom.tar.gz` is served from
the same address as the Landscape server at port 9009. In practice, that URL could point to any
address in an intranet or the internet that's accessible from the client computers.


The image server can also provide an SHA256SUMS file, as done by
[cloud-images.ubuntu.com](https://cloud-images.ubuntu.com) and
[cdimage.ubuntu.com](https://cdimage.ubuntu.com). If that file is available, the Windows agent
validates the images against the SHA256SUMS file before installation.


While the example uses the `.tar.gz` extension, the most recent `.wsl` format can also be used.
Refer to [the image customisation for Ubuntu on WSL guide](howto::custom-distro) for more
information.
```


Generate a Base64-encoded string with the cloud-config data:

`````{tabs}

````{group-tab} Bash

```bash
BASE64_ENCODED_CLOUD_INIT=$(cat $CLOUD_INIT_FILE | base64 --wrap=0)
```

````

````{group-tab} PowerShell
```powershell
$content = Get-Content -Path $CLOUD_INIT_FILE -Raw
$bytes = [System.Text.Encoding]::UTF8.GetBytes($content)
$BASE64_ENCODED_CLOUD_INIT = [System.Convert]::ToBase64String($bytes)
```

````

`````

## Authenticate against the Landscape API

Build the authentication payload of the form: `{"email": "admin@mib.com", "password": "mib"}` using the values exported in prior steps:

`````{tabs}

````{group-tab} Bash
```bash
LOGIN_JSON=$( jq -n \
    --arg em "$LANDSCAPE_USER_EMAIL" \
    --arg pwd "$LANDSCAPE_USER_PASSWORD" \
    '{email: $em, password: $pwd}' )
```
````

````{group-tab} PowerShell
```powershell
$LOGIN_JSON = @{
 email = "$LANDSCAPE_USER_EMAIL"
 password = "$LANDSCAPE_USER_PASSWORD"
} | ConvertTo-Json
```
````
`````

Issue an authenticate request and retrieve the JSON web token (JWT) to be used in the subsequent API requests.

`````{tabs}

````{group-tab} Bash
```bash
LOGIN_RESPONSE=$( curl -s -X POST "$LANDSCAPE_URL/api/v2/login" \
    --data "$LOGIN_JSON"                                        \
    --header "Content-Type: application/json"                   \
    --header "Accept: application/json" )

JWT=$( echo $LOGIN_RESPONSE | jq .token | tr -d '"')
```
````

````{group-tab} PowerShell
```powershell
$LOGIN_RESPONSE = Invoke-WebRequest -Method POST `
    -URI "$LANDSCAPE_URL/api/v2/login"           `
    -Body "$LOGIN_JSON" -ContentType "application/json"

$JWT = ConvertTo-SecureString -AsPlainText -Force $( $LOGIN_RESPONSE.Content | ConvertFrom-Json).token
```
````

`````

## Send the Install request

Build the payload with information about the WSL instance to be deployed. In this case it would look like:

```json
{"rootfs_url": "http://landscape.mib.com:9009/ubuntu-24.04-custom.tar.gz", "computer_name": "Carbonizer", "cloud_init": "<base64 encoded material>"}
```

`````{tabs}

````{group-tab} Bash
```bash
WSL_JSON=$( jq -n                           \
    --arg rf "$ROOTFS_URL"                  \
    --arg cn "$COMPUTER_NAME"               \
    --arg b64 "$BASE64_ENCODED_CLOUD_INIT"  \
    '{rootfs_url: $rf, computer_name: $cn, cloud_init: $b64}' )
```
````

````{group-tab} PowerShell
```powershell
$WSL_JSON = @{
 rootfs_url = "$ROOTFS_URL"
 computer_name = "$COMPUTER_NAME"
 cloud_init = "$BASE64_ENCODED_CLOUD_INIT"
} | ConvertTo-Json

```
````
`````

At the moment of this writing there is no specific API endpoint to trigger
installation of WSL instances on multiple Windows machines at once.
Instead we send one request per target machine.

`````{tabs}

````{group-tab} Bash
```bash
for COMPUTER_ID in "${PARENT_COMPUTER_IDS[@]}"; do
    API_RESPONSE=$( curl -s -X POST                             \
        "$LANDSCAPE_URL/api/v2/computers/$COMPUTER_ID/children" \
        --data "$WSL_JSON"                                      \
        --header "Authorization:Bearer $JWT"                    \
        --header "Content-Type: application/json"               \
        --header "Accept: application/json" )

    # show the response
    echo $API_RESPONSE
    echo
done
```
````

````{group-tab} PowerShell

```powershell
foreach ($COMPUTER_ID in $PARENT_COMPUTER_IDS) {
    $API_RESPONSE = Invoke-WebRequest -Method POST -Body "$WSL_JSON" `
        -Uri "$LANDSCAPE_URL/api/v2/computers/$COMPUTER_ID/children" `
        -Authentication Bearer -Token $JWT -ContentType "application/json"

    # show the response
    Write-Output $API_RESPONSE
}
```
````
`````

When that completes, you'll be able to find activities in the Landscape
dashboard about the installation of a new WSL instance for each of the Windows
machines listed.

## Summarising the steps in a single script

The steps above can be made into a single script:

`````{tabs}


````{group-tab} Bash
```bash
#!/usr/bin/env bash

# Base64-encoding the cloud-config file contents
BASE64_ENCODED_CLOUD_INIT=$(cat $CLOUD_INIT_FILE | base64 --wrap=0)

# Build the auth payload
LOGIN_JSON=$( jq -n                      \
    --arg em "$LANDSCAPE_USER_EMAIL"     \
    --arg pwd "$LANDSCAPE_USER_PASSWORD" \
    '{email: $em, password: $pwd}' )

# Issue an auth request and retrieve the JWT
LOGIN_RESPONSE=$( curl -s -X POST "$LANDSCAPE_URL/api/v2/login" \
    --data "$LOGIN_JSON"                                        \
    --header "Content-Type: application/json"                   \
    --header "Accept: application/json" )

JWT=$( echo $LOGIN_RESPONSE | jq .token | tr -d '"')

# Build the installation payload
WSL_JSON=$( jq -n                           \
    --arg rf "$ROOTFS_URL"                  \
    --arg cn "$COMPUTER_NAME"               \
    --arg b64 "$BASE64_ENCODED_CLOUD_INIT"  \
    '{rootfs_url: $rf, computer_name: $cn, cloud_init: $b64}' )

# Issue the command for each Windows machine
for COMPUTER_ID in "${PARENT_COMPUTER_IDS[@]}"; do
    API_RESPONSE=$( curl -s -X POST                             \
        "$LANDSCAPE_URL/api/v2/computers/$COMPUTER_ID/children" \
        --data "$WSL_JSON"                                      \
        --header "Authorization:Bearer $JWT"                    \
        --header "Content-Type: application/json"               \
        --header "Accept: application/json" )

    # show the response
    echo $API_RESPONSE
    echo
done
```
````


````{group-tab} PowerShell
```powershell
# Base64-encoding the cloud-config file contents
$content = Get-Content -Path $CLOUD_INIT_FILE -Raw
$bytes = [System.Text.Encoding]::UTF8.GetBytes($content)
$BASE64_ENCODED_CLOUD_INIT = [System.Convert]::ToBase64String($bytes)

# Build the auth payload
$LOGIN_JSON = @{
 email = "$LANDSCAPE_USER_EMAIL"
 password = "$LANDSCAPE_USER_PASSWORD"
} | ConvertTo-Json

# Issue an auth request and retrieve the JWT
$LOGIN_RESPONSE = Invoke-WebRequest -Method POST `
    -URI "$LANDSCAPE_URL/api/v2/login"           `
    -Body "$LOGIN_JSON" -ContentType "application/json"

$JWT = ConvertTo-SecureString -AsPlainText -Force $( $LOGIN_RESPONSE.Content | ConvertFrom-Json).token

# Build the installation payload
$WSL_JSON = @{
 rootfs_url = "$ROOTFS_URL"
 computer_name = "$COMPUTER_NAME"
 cloud_init = "$BASE64_ENCODED_CLOUD_INIT"
} | ConvertTo-Json

# Issue the command for each Windows machine
foreach ($COMPUTER_ID in $PARENT_COMPUTER_IDS) {
    $API_RESPONSE = Invoke-WebRequest -Method POST -Body "$WSL_JSON" `
        -Uri "$LANDSCAPE_URL/api/v2/computers/$COMPUTER_ID/children" `
        -Authentication Bearer -Token $JWT -ContentType "application/json"

    # show the response
    Write-Output $API_RESPONSE
}

```
````

`````

## Further reading

- Visit [the Landscape API documentation](https://ubuntu.com/landscape/docs/make-rest-api-requests) to learn more about it.
- [Landscape documentation about WSL integration](https://ubuntu.com/landscape/docs/use-a-specific-ubuntu-image-source-for-wsl)
contains more information about this and other methods of creating WSL
instances on Windows machines registered with Landscape via its REST API.
