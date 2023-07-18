$appx = Get-AppxPackage -Name "CanonicalGroupLimited.UbuntuPreview"
if ( $appx -eq "" ) {
    Write-Error "Ubuntu Preview is not installed"
}

ubuntupreview.exe install --root --ui=none

Push-Location "${PSScriptRoot}\..\.."

$script=New-TemporaryFile

# Using WriteAllText to avoid CRLF
[IO.File]::WriteAllText($script, @'
set -eu

# Set up directory
build_dir="${HOME}/wsl-pro-service-build"

rsync --recursive --quiet --exclude=".git" "." "${build_dir}"

# Build
bash -e "${build_dir}/tools/build/build-deb.sh"

# Export artifacts
cp -f ${build_dir}/wsl-pro-service_* .
'@)

wsl.exe -d Ubuntu-Preview -u root -- bash "`$(wslpath -ua `'${script}`')"

Remove-Item -Path "${script}"

Pop-Location