# build-msi.ps1 — produce goat-client-${VERSION}-${ARCH}.msi from a
# WiX v4 toolchain. Driven from CI (Track E's Windows runner).
#
# Inputs:
#   dist\windows_${ARCH}\goat-clientd.exe
#   dist\windows_${ARCH}\goat-client.exe
#   dist\windows_${ARCH}\wintun.dll       (vendored separately)
#   internal\ui\assets\goat-client.ico    (Track B asset)
#   internal\ui\assets\goat-client.png    (Track B asset)
#
# Output:
#   dist\goat-client-${VERSION}-${ARCH}.msi

param(
    [Parameter(Mandatory = $true)] [string]$Version,
    [ValidateSet('amd64', 'arm64')] [string]$Arch = 'amd64'
)

$ErrorActionPreference = 'Stop'

# Map our GOARCH naming to WiX's ProcessorArchitecture vocabulary.
$processorArch = if ($Arch -eq 'arm64') { 'arm64' } else { 'x64' }

$env:GOAT_VERSION = $Version

$out = "dist\goat-client-${Version}-${Arch}.msi"
New-Item -ItemType Directory -Force -Path 'dist' | Out-Null

wix build packaging\msi\goat-client.wxs `
    -ext WixToolset.Util.wixext `
    -d ProcessorArchitecture=$processorArch `
    -d ArchSuffix=$Arch `
    -arch $processorArch `
    -o $out

Write-Host "built $out"

# Authenticode signing (gated on operator-fired procurement).
if ($env:WINDOWS_SIGNING_CERT_PATH -and $env:WINDOWS_SIGNING_CERT_PASSWORD) {
    & signtool sign `
        /f $env:WINDOWS_SIGNING_CERT_PATH `
        /p $env:WINDOWS_SIGNING_CERT_PASSWORD `
        /fd SHA256 `
        /tr http://timestamp.digicert.com `
        /td SHA256 `
        /d "goat-client" `
        /du "https://github.com/dlf-dds/goat-client" `
        $out
    Write-Host "signed $out"
} else {
    Write-Warning "shipping unsigned .msi — Authenticode signing skipped (no WINDOWS_SIGNING_CERT_PATH)"
}
