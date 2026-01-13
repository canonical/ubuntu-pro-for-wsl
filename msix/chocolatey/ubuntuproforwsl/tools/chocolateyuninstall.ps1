$ErrorActionPreference = 'Stop'

# This is a constant string defined within the MSIX manifest, NOT the Chocolatey package ID.
$msixPackageName = "CanonicalGroupLimited.UbuntuPro"
# The app specific registry key not virtualized.
$registryKeyPath = "Software\Canonical\UbuntuPro"

Write-Host "Starting uninstallation for ${env:ChocolateyPackageName}..."

Write-Host "   Starting registry cleanup for all user profiles..."

try {
    # Create a PSDrive for HKEY_USERS if it isn't available for PowerShell.
    if (-not (Test-Path HKU:)) {
        New-PSDrive -Name HKU -PSProvider Registry -Root HKEY_USERS
    }
    # SIDs starting with S-1-5-21 typically denote actual interactive users.
    $userProfiles = Get-ChildItem 'HKU:\' | Where-Object { $_.PSChildName -match '^S-1-5-21-\d+-\d+-\d+-\d+$' }

    if ($userProfiles.Count -eq 0) {
        Write-Warning "   No interactive user profiles found in HKU. Skipping user registry cleanup."
    }
    else {
        foreach ($prof in $userProfiles) {
            $sid = $prof.PSChildName
            $fullKeyPath = "HKU:\$sid\$registryKeyPath"

            Remove-Item -Path $fullKeyPath -Recurse -Force -ErrorAction SilentlyContinue
            Write-Host "   Successfully removed registry key $fullKeyPath."
        }
    }
}
catch {
    Write-Warning "   Registry cleanup: $($_.Exception.Message)"
    # Continue to the package removal, as registry cleanup failure shouldn't block uninstallation.
}


Write-Host "Checking for and removing provisioned package..."
# Clean up any remaining data before unprovisioning.
ubuntu-pro-agent.exe clean | Out-Null

try {
    # Get the provisioned package object, which requires the exact Package Name string.
    $provisionedPackage = Get-AppxProvisionedPackage -Online | Where-Object { $_.PackageName -eq $msixPackageName }

    if ($provisionedPackage) {
        # Remove the package from the system image (unprovisioning it).
        Remove-AppxProvisionedPackage -Online -PackageName $provisionedPackage.PackageName -ErrorAction Stop
        Write-Host "Successfully unprovisioned $msixPackageName."
    } else {
        Write-Host "$msixPackageName was not found as a provisioned package. Skipping unprovisioning."
    }
}
catch {
    Write-Error "Failed to unprovision ${msixPackageName}: $($_.Exception.Message)"
    # Note: We won't throw here, as we still want to try to remove it for existing users.
}


Write-Host "Checking for and removing package for all existing users..."

try {
    # Get all installed packages that match the base name across all user accounts.
    # We use a wildcard to match any version or architecture (the Package Family Name part).
    $installedPackages = Get-AppxPackage -AllUsers -Name "*$msixPackageName*"

    if ($installedPackages) {
        # Iterate over all found packages (in case different versions/architectures exist)
        foreach ($package in $installedPackages) {
            Write-Host "Attempting removal of $($package.PackageFullName) for all users."

            # Remove-AppxPackage with -AllUsers removes the package from all profiles.
            Remove-AppxPackage -Package $package.PackageFullName -AllUsers -ErrorAction Stop
            Write-Host "Removed package $($package.PackageFullName) successfully."
        }
    } else {
        Write-Host "$msixPackageName was not found installed for any user. Skipping user removal."
    }
}
catch {
    Write-Error "Failed to remove $msixPackageName for users: $($_.Exception.Message)"
    # Re-throw the error to indicate a critical uninstall failure.
    throw
}

Write-Host "Uninstallation of ${env:ChocolateyPackageName} complete."