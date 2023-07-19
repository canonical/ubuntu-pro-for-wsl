$appx = Get-AppxPackage -Name "CanonicalGroupLimited.UbuntuPreview"
if ( $appx -eq "" ) {
    Write-Error "Ubuntu Preview is not installed"
}

$env:WSL_UTF8=1

if ( "$(wsl --list --verbose | Select-String Ubuntu-Preview)" -eq "" ) {
    ubuntupreview.exe install --root --ui=none
    if ( "${LastExitCode}" -ne "0" ) {
        Write-Error "could not install Ubuntu-Preview"
        exit 1
    }
}

$scriptWindows=New-TemporaryFile

# Using WriteAllText to avoid CRLF
[IO.File]::WriteAllText($scriptWindows, @'
#!/bin/bash
set -eu

# Set up directory
build_dir="${HOME}/wsl-pro-service-build"

rsync --recursive --quiet --exclude=".git" "." "${build_dir}"

# Build
bash -e "${build_dir}/tools/build/build-deb.sh"

# Export artifacts
cp -f ${build_dir}/wsl-pro-service_* .

'@)

$scriptLinux=( wsl.exe -d Ubuntu-Preview -- wslpath -ua `'${scriptWindows}`' )
if ( "${LastExitCode}" -ne "0" ) {
    Write-Error "could not get build script's linux path"
    exit 1
}

wsl.exe -d Ubuntu-Preview -u root -- bash -ec "chmod +x ${scriptLinux} 2>&1"
if ( "${LastExitCode}" -ne "0" ) {
    Write-Error "could not make build script executable"
    exit 1
}

Push-Location "${PSScriptRoot}\..\.."

wsl.exe -d Ubuntu-Preview -u root -- bash -ec "${scriptLinux} 2>&1"
if ( "${LastExitCode}" -ne "0" ) {
    Write-Error "could not build deb"
    exit 1
}

Pop-Location

Remove-Item -Path "${scriptWindows}"

exit 0
