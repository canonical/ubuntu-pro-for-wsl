$appx = Get-AppxPackage -Name "CanonicalGroupLimited.UbuntuPreview"
if ( $appx -eq "" ) {
    Write-Error "Ubuntu Preview is not installed"
}

ubuntupreview.exe install --root --ui=none

Push-Location "${PSScriptRoot}\.."

$script=New-TemporaryFile

# Using WriteAllText to avoid CRLF
[IO.File]::WriteAllText($script, @'
set -eu

# Install dependencies
apt update
apt install -y devscripts equivs

# Set up directory
build_dir="${HOME}/wsl-pro-service-build"
mkdir -p "${build_dir}"
rsync --recursive --quiet wsl-pro-service "${build_dir}"

# Build
cd "${build_dir}/wsl-pro-service"
mk-build-deps --install --tool="apt -y" --remove
DEB_BUILD_OPTIONS=nocheck UP4W_SKIP_INTERNAL_DEPENDENCY_UPDATE=1 debuild
cd -

# Export artifacts
rsync --recursive --quiet --exclude="wsl-pro-service/" "${build_dir}/" "."
'@)

wsl.exe -d Ubuntu-Preview -u root -- bash "`$(wslpath -ua `'${script}`')"

Remove-Item -Path "${script}"

Pop-Location