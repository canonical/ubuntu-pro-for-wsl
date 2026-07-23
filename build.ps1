<#
.Synopsis
    Build and package the Ubuntu Pro for WSL MSIX package.
.DESCRIPTION
    Two-phase build system:
    - Compile: Compiles all components (C++, Go, Flutter), patches XML manifests and places them into dist/<arch>/
    - Pack:    Creates the MSIX package via winappCLI from the provided input folders and manifest file.
.PARAMETER Action
    The action to perform: Compile or Pack.

.PARAMETER Config
    Build configuration: Release or Debug (default: Release).
.PARAMETER Platform
    Target architecture: x64 or arm64 (auto-detected from host if omitted).
.PARAMETER Mode
    Build mode: production or end_to_end_tests (default: production). end_to_end_tests enables the mock store API and the Flutter test entrypoint.
.PARAMETER FullVersion
    Full git describe version string to embed in Go and Flutter binaries.
.PARAMETER Version
    Numeric MSIX version (e.g. "1.2.3.0") for the package and appinstaller.
.PARAMETER Tag
    Git tag of the latest release construct the App installer MainBundle URIs.

.PARAMETER InputFolders
    One or more folders to package. Multiple folders produce a multi-architecture .msixbundle. Single folder produces only a .msix.
.PARAMETER Manifest
    A Package.appxmanifest file containing the packaging definitions.
.PARAMETER Cert
    Path to a PFX certificate for signing the MSIX.
.PARAMETER CertPass
    Password for the PFX certificate (as a SecureString).
.PARAMETER Output
    Optional output file path for the resulting MSIX bundle.
#>

param (
    [Parameter(Mandatory = $true, Position = 0)]
    [ValidateSet('Compile', 'Pack')]
    [string]$Action,

    [Parameter(ParameterSetName = 'Compile', Mandatory = $true)][ValidateSet('Release', 'Debug')][string]$Config = 'Release',
    [Parameter(ParameterSetName = 'Compile')][ValidateSet('x64', 'arm64')][string]$Platform,
    [Parameter(ParameterSetName = 'Compile')][ValidateSet('production', 'end_to_end_tests')][string]$Mode = 'production',
    [Parameter(ParameterSetName = 'Compile')][string]$FullVersion,
    [Parameter(ParameterSetName = 'Compile', Mandatory = $true)][string]$Version,
    [Parameter(ParameterSetName = 'Compile', Mandatory = $true)][string]$Tag,

    [Parameter(ParameterSetName = 'Pack', Mandatory = $true)][string[]]$InputFolders,
    [Parameter(ParameterSetName = 'Pack', Mandatory = $true)][string]$Manifest,
    [Parameter(ParameterSetName = 'Pack')][string]$Cert,
    [Parameter(ParameterSetName = 'Pack')][SecureString]$CertPass,
    [Parameter(ParameterSetName = 'Pack')][string]$Output
)

$ErrorActionPreference = 'Stop'
$RepoRoot = $PSScriptRoot
$InstallTree = "$RepoRoot\dist"
if ($Action -eq 'Compile' -and -not $Platform) {
    $Platform = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x64' }
}
$ArchInstallTree = "$InstallTree\$Platform"

# -------------------------------------------------------------------
# Build phase functions
# -------------------------------------------------------------------

# Builds the C++ components and places the outputs into the 'dist\<arch>' folder.
function Build-Cpp {
    $buildDir = "$RepoRoot\build\$Platform"
    if (-not (Test-Path "$buildDir\CMakeCache.txt")) {
        Write-Output "Configuring CMake for $Platform..."
        cmake -S "$RepoRoot" -B "$buildDir" -A "$Platform" "-DCMAKE_BUILD_TYPE=$Config"
        if ($LASTEXITCODE -ne 0) { throw "CMake configure failed" }
    }

    Write-Output "Building C++ components..."
    cmake --build "$buildDir" --config "$Config"
    if ($LASTEXITCODE -ne 0) { throw "CMake build failed" }

    # Places the artifacts to the 'dist\<arch>' folder
    $agentInstallDir = "$ArchInstallTree\agent"
    New-Item -ItemType Directory -Path "$agentInstallDir" -Force | Out-Null
    $buildDir = "$RepoRoot\build\$Platform"
    cmake --install $buildDir --prefix $agentInstallDir --config $Config
    if ($LASTEXITCODE -ne 0) { throw "CMake install failed" }
}

# Builds the Go component and places the outputs into the 'dist\<arch>' folder.
function Build-Go {
    $agentInstallDir = "$ArchInstallTree\agent"
    New-Item -ItemType Directory -Path "$agentInstallDir" -Force | Out-Null

    $args = @('build')
    if ($Mode -eq 'end_to_end_tests') {
        $args += '-tags=server_mocks'
    }
    if ($FullVersion) {
        $args += "-ldflags=-X=github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts.Version=$FullVersion"
    }
    $args += @('-o', "$agentInstallDir\ubuntu-pro-agent.exe", '.')

    Write-Output "Building Go agent..."
    Push-Location "$RepoRoot\windows-agent\cmd\ubuntu-pro-agent"
    try {
        & go @args
        if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
    } finally {
        Pop-Location
    }
}

# Builds the Flutter component and places the outputs into the 'dist\<arch>' folder.
function Build-Flutter {
    $flutterDir = "$RepoRoot\gui\packages\ubuntupro"
    $iconSrc = "$RepoRoot\msix\Images\icon.ico"
    $iconDst = "$flutterDir\windows\runner\resources\app_icon.ico"

    Write-Output "Copying MSIX icon to Flutter app..."
    Copy-Item -LiteralPath "$iconSrc" -Destination "$iconDst" -Force

    $flutterArgs=@()
    $configFlag = if ($Config -eq 'Release') { '--release' } else { '--debug' }
    $flutterArgs += $configFlag
    if ($Mode -eq 'end_to_end_tests') {
        $flutterArgs += '-t','end_to_end/end_to_end_test.dart'
    }
    if ($FullVersion) {
        $flutterArgs += "--dart-define=""UP4W_FULL_VERSION=$FullVersion"""
    }

    Write-Output "Building Flutter app..."
    Push-Location "$flutterDir"
    try {
        & flutter build windows @flutterArgs
        if ($LASTEXITCODE -ne 0) { throw "Flutter build failed" }
    } finally {
        Pop-Location
    }

    # Places the artifacts to the 'dist\<arch>' folder
    $flutterBuildDir = "$RepoRoot\gui\packages\ubuntupro\build\windows\$Platform\runner\$Config"
    $guiInstallDir = "$ArchInstallTree\gui"
    New-Item -ItemType Directory -Path "$guiInstallDir" -Force | Out-Null
    Copy-Item -Recurse "$flutterBuildDir\*" "$guiInstallDir\" -Force
}

# Updates the App installer file Version and the MSIX version it refers to (they must remain in sync)
# and saves the result to the Destination folder.
function Edit-AppinstallerFile {
    param([string]$Version, [string]$Destination)

    if ($Mode -eq "end_to_end_tests") {
        Write-Host "Skipping app installer file generation to avoid undesired updates in CI..." -ForegroundColor DarkYellow
        return
    }

    $appinstallerFile = "UbuntuProForWSL.appinstaller"
    Write-Output "Patching appinstaller version to $Version..."

    [Reflection.Assembly]::LoadWithPartialName("System.Xml.Linq") | Out-Null
    $doc = [System.Xml.Linq.XDocument]::Load("$RepoRoot\msix\$appinstallerFile")
    $doc.Root.SetAttributeValue("Version", $Version)
    $ns = $doc.Root.Name.Namespace
    $mainBundle = $doc.Root.Element($ns + "MainBundle")
    if ($mainBundle) {
        $mainBundle.SetAttributeValue("Version", $Version)
        $mainBundle.SetAttributeValue("Uri", "https://github.com/canonical/ubuntu-pro-for-wsl/releases/download/${Tag}/UbuntuProForWSL.msixbundle")
    }
    
    # Save changes to the destination folder rather than changing source files.
    $doc.Save("$Destination\$appinstallerFile")
}

# Updates the MSIX Version in the Package.appxmanifest file and saves the result to the Destination folder.
function Edit-AppxManifest {
    param([string]$Version, [string]$Config, [string]$Destination)
    $manifest = "Package.appxmanifest"
    Write-Output "Patching $manifest version to $Version..."

    [Reflection.Assembly]::LoadWithPartialName("System.Xml.Linq") | Out-Null
    $doc = [System.Xml.Linq.XDocument]::Load("$RepoRoot\msix\$manifest")
    $ns = $doc.Root.Name.Namespace
    $identity = $doc.Root.Element($ns + "Identity")
    if ($identity) {
        $identity.SetAttributeValue("Version", $Version)
    }

    # Mimics the MSBuild magic behaviour.
    if ($Config -eq "Debug") {
        Write-Output "Replacing the Microsoft.VCLibs.140.00.UWPDesktop PackageDependency  in the $manifest with its debug version..."
        $dep = $doc.Root.Element($ns + "Dependencies").Elements() | Where-Object { $_.Attribute("Name").Value -eq "Microsoft.VCLibs.140.00.UWPDesktop"}
        if ($null -ne $dep) {
            $dep.SetAttributeValue("Name", "Microsoft.VCLibs.140.00.Debug.UWPDesktop")
        }
    }

    if ($Mode -eq "end_to_end_tests") {
        Write-Host "Removing references to the app installer file to avoid undesired updates in CI..." -ForegroundColor DarkYellow
        $autoupdate = $doc.Root.Element($ns + "Properties").Element("{http://schemas.microsoft.com/appx/manifest/uap/windows10/13}AutoUpdate")
        if ($null -ne $autoupdate){
            $autoupdate.Remove()
        }
    }

    # Save changes to the destination folder rather than changing source files.
    $doc.Save("$Destination\$manifest")
}

# Copies the non-compiled components (image assets and other static files) to the install tree (dist\<arch>).
function Copy-Assets {
    Write-Output "Placing asets to dist\$Platform\..."

    # Images
    Copy-Item -Recurse "$RepoRoot\msix\Images" "$ArchInstallTree\Images" -Force

    # Appinstaller file needs to be embedded into the inner MSIX packages
    Edit-AppinstallerFile -Version $Version -Destination $ArchInstallTree

    # Package.appxmanifest doesn't go embedded into the inner MSIX packages, thus placed one level above.
    Edit-AppxManifest -Version $Version -Config $Config -Destination "$InstallTree"
}

# -------------------------------------------------------------------
# Pack phase functions
# -------------------------------------------------------------------

# Runs winappCLI pack pointing to the appx manifest file provided.
function Invoke-WinappPack {
    $certArgs = @()

    if ($Cert) {
        $certArgs += '--cert', $Cert
        $exportArgs = @{FilePath=$Cert}
        if ($CertPass) {
            # Exporting the .cer file from a password protected PFX requires PowerShell 7 (pwsh.exe on GitHub)
            $exportArgs["Password"]=$CertPass
            $ptr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($CertPass)
            try {
                $plainPass = [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
                $certArgs += '--cert-password', $plainPass
            } finally {
                [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
            }
        }
        $basename = $(Get-Item $Cert).BaseName
        Get-PfxCertificate @exportArgs | Export-Certificate -FilePath "$basename.cer" -Type CERT
    }

    Write-Output "Packing MSIX from: $($InputFolders -join ', ')..."
    $winappArgs = @('pack', '--verbose') + $InputFolders + @('--manifest', $Manifest) + $certArgs
    if ($Output) { $winappArgs += @('--output', $Output) }

    & winapp @winappArgs
    if ($LASTEXITCODE -ne 0) { throw "winapp pack failed" }
}

# -------------------------------------------------------------------
# Main dispatch
# -------------------------------------------------------------------

switch ($Action) {
    'Compile' {
        if ($Mode -eq "end_to_end_tests") {
            $env:UP4W_TEST_WITH_MS_STORE_MOCK=1
        }
        Build-Cpp
        Build-Go
        Build-Flutter
        Write-Output "Compile complete."
        Copy-Assets
        Write-Output "dist/$Platform/ layout complete."
    }
    'Pack' {
        foreach ($folder in $InputFolders) {
            if (-not (Test-Path "$folder")) {
                throw "Input folder not found: $folder"
            }
        }
        Invoke-WinappPack
        Write-Output "Pack complete."
    }
}
