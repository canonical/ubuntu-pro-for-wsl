<#
.Synopsis
    Build the Ubuntu Pro For Windows Appx package for local use.
#>

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

        foreach ( $release in "Enterprise","Professional","Community") {
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
        Write-Error "You need a certificate to build and install the Appx. `
        Create and install a certificate, and write its thumbprint in ${certificate_path}.`
        See https://learn.microsoft.com/en-us/windows/win32/appxpkg/how-to-create-a-package-signing-certificate for more details"
        exit 1
    }

    $certificate_thumbprint = Get-Content ${certificate_path}

    # Replacing with local certificate
    $wapproj = ".\msix\UbuntuProForWindows\UbuntuProForWindows.wapproj"
    (Get-Content -Path "${wapproj}")                                                                   `
        -replace                                                                                       `
            "<PackageCertificateThumbprint>.*</PackageCertificateThumbprint>",                         `
            "<PackageCertificateThumbprint>${certificate_thumbprint}</PackageCertificateThumbprint>"   `
        | Set-Content -Path "${wapproj}"
}

function Install-Appx {
    $artifacts = (
        Get-ChildItem ".\msix\UbuntuProForWindows\AppPackages\UbuntuProForWindows_*"    `
        | Sort-Object LastWriteTime                                                     `
        | Select-Object -last 1                                                         `
    )
    
    & "${artifacts}\Install.ps1" -Force

    if ( "${LastExitCode}" -ne "0" ) {
        Write-Output "could not install Appx"
        exit 1
    }
}

# Uninstall currently installed version
Get-AppxPackage "CanonicalGroupLimited.UbuntuProForWindows" | Remove-AppxPackage

# Going to project root
Push-Location "${PSScriptRoot}\..\.."

Update-Certificate

Start-VsDevShell

msbuild.exe                                          `
    .\msix\msix.sln                                  `
    -target:Build                                    `
    -maxCpuCount                                     `
    -property:Configuration=Release                  `
    -property:AppxBundle=Always                      `
    -property:AppxBundlePlatforms=x64                `
    -property:ProcessorArchitecture=x64              `
    -property:UapAppxPackageBuildMode=SideloadOnly   `
    -nologo                                          `
    -verbosity:normal

if (! $?) { exit 1 }

Install-Appx

Pop-Location