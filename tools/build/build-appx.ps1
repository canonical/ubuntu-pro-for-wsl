<#
.Synopsis
    Build the Ubuntu Pro for WSL Appx package for local use.
#>

param (
    [Parameter(Mandatory = $true, HelpMessage = "production, end_to_end_tests.")]
    [string]$mode,

    [Parameter(Mandatory = $false, HelpMessage = "A directory were the MSIX and the certificate will be copied to")]
    [string]$OutputDir
)

function Start-VsDevShell {
    # Looking for a path like
    # ${env:ProgramFiles}\Microsoft Visual Studio\$VERSION\$RELEASE\Common7\Tools\Launch-VsDevShell.ps1
    # where VERSION is a four-digit number and RELEASE is one of Enterprise, Professional, or Community.
 
    $vsRoot = "${env:ProgramFiles}\Microsoft Visual Studio"
    if (! (Test-Path "${vsRoot}")) {
        Write-Error "Visual Studio could not be found in ${vsRoot}"
        exit 1 
    }

    $versions = Get-ChildItem "${vsRoot}" | ForEach-Object { $_.Name } | Sort-Object
    if ( "$versions" -eq "" ) {
        Write-Error "No version of Visual Studio found" 
        exit 1
    }

    foreach ( $version in $versions ) {
        if (! (Test-Path "${vsRoot}\${version}")) {
            continue
        }

        foreach ( $release in "Enterprise", "Professional", "Community") {
            $devShell = "${vsRoot}\${version}\${release}\Common7\Tools\Launch-VsDevShell.ps1"
            if (! (Test-Path "${devShell}") ) {
                continue
            }

            & "${devShell}" -SkipAutomaticLocation
            return
        }
    }

    Write-Error "Visual Studio developer powershell could not be found"
    exit 1
}

function Update-Certificate {
    # Finding local certificate
    $certificate_path = "${PSScriptRoot}\.certificate_thumbprint"
    if (! (Test-Path "${certificate_path}") ) {
        Write-Warning "You need a certificate to build and install the Appx. `
        Create and install a certificate, and write its thumbprint in ${certificate_path}.`
        See https://learn.microsoft.com/en-us/windows/win32/appxpkg/how-to-create-a-package-signing-certificate for more details"
        
        Write-Output "Continuing with default certificate"        
        return
    }

    $certificate_thumbprint = Get-Content ${certificate_path}

    # Replacing with local certificate
    $wapproj = ".\msix\UbuntuProForWSL\UbuntuProForWSL.wapproj"
    (Get-Content -Path "${wapproj}")                                                                   `
        -replace `
        "<PackageCertificateThumbprint>.*</PackageCertificateThumbprint>", `
        "<PackageCertificateThumbprint>${certificate_thumbprint}</PackageCertificateThumbprint>"   `
    | Set-Content -Path "${wapproj}"
}

function Set-Version {
    $versionInfo=ConvertFrom-Json $(go run .\tools\build\compute_version.go --json)
    $env:UP4W_FULL_VERSION=$($versionInfo.full_version)
    $UP4W_VERSION=$($versionInfo.numeric_version)
    # Update the AppxManifest version
    [Reflection.Assembly]::LoadWithPartialName("System.Xml.Linq")
    $path = "$PWD/Msix/UbuntuProForWSL/Package.appxmanifest"

    Write-Output "Setting version to $UP4W_VERSION in file $path"
    Write-Output "Building with full-version $env:UP4W_FULL_VERSION"

    $doc = [System.Xml.Linq.XDocument]::Load($path)
    $xName = [System.Xml.Linq.XName]::Get("{http://schemas.microsoft.com/appx/manifest/foundation/windows10}Identity")
    $doc.Root.Element($xName).Attribute("Version").Value = "$UP4W_VERSION.0";
    $doc.Save($path)
}

function Install-Appx {
    Get-AppxPackage -Name "CanonicalGroupLimited.UbuntuPro" | Remove-AppxPackage

    $artifacts = (
        Get-ChildItem ".\msix\UbuntuProForWSL\AppPackages\UbuntuProForWSL_*"    `
        | Sort-Object LastWriteTime                                                     `
        | Select-Object -last 1                                                         `
    )

    if ( "${OutputDir}" -ne "" ) {
        Copy-Item -Path "${artifacts}/*.cer" -Destination "${OutputDir}"
        Copy-Item -Path "${artifacts}/*.msixbundle" -Destination "${OutputDir}"
    }

    If ($mode -ne 'production') {
        Add-AppxPackage "${env:ProgramFiles(x86)}\Microsoft SDKs\Windows Kits\10\ExtensionSDKs\Microsoft.VCLibs.Desktop\14.0\Appx\Debug\x64\Microsoft.VCLibs.x64.Debug.14.00.Desktop.appx"
    }
    & "${artifacts}\Install.ps1" -Force

    if ( "${LastExitCode}" -ne "0" ) {
        Write-Output "could not install Appx"
        exit 1
    }
}

# Uninstall currently installed version
Get-AppxPackage "CanonicalGroupLimited.UbuntuPro" | Remove-AppxPackage

# Going to project root
Push-Location "${PSScriptRoot}\..\.."

# Must be the first thing otherwise it may append `-dirty` in the full version string.
Set-Version
Update-Certificate

try {
    msbuild.exe --version
}
catch {
    Start-VsDevShell
}

If ($mode -eq 'end_to_end_tests') {
    $env:UP4W_TEST_WITH_MS_STORE_MOCK = 1
}

msbuild.exe                                                                              `
    .\msix\msix.sln                                                                      `
    -target:Build                                                                        `
    -maxCpuCount                                                                         `
    -property:Configuration=$(If($mode -eq 'production'){"Release"} Else {"Debug"})      `
    -property:AppxBundle=Always                                                          `
    -property:AppxBundlePlatforms=x64                                                    `
    -property:ProcessorArchitecture=x64                                                  `
    -property:UapAppxPackageBuildMode=SideloadOnly                                       `
    -nologo                                                                              `
    -property:UP4W_END_TO_END=$(If($mode -eq 'end_to_end_tests'){"true"} Else {"false"}) `
    -verbosity:normal

if (! $?) { exit 1 }

Install-Appx

Pop-Location