param(
    [switch]$SelfTest
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Repo = "php-workx/epos"

function Get-EposArchiveName {
    param(
        [Parameter(Mandatory)][string]$Tag,
        [Parameter(Mandatory)][string]$OS,
        [Parameter(Mandatory)][string]$Arch
    )

    $version = $Tag.TrimStart("v")
    if ($OS -eq "windows") {
        return "epos_${version}_${OS}_${Arch}.zip"
    }
    return "epos_${version}_${OS}_${Arch}.tar.gz"
}

function Get-EposArch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString()
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "unsupported architecture: $arch" }
    }
}

function Get-ChecksumForAsset {
    param(
        [Parameter(Mandatory)][string]$ChecksumsPath,
        [Parameter(Mandatory)][string]$AssetName
    )

    foreach ($line in Get-Content -Path $ChecksumsPath) {
        $parts = $line.Trim() -split "\s+"
        if ($parts.Count -ge 2 -and $parts[1].TrimStart("*") -eq $AssetName) {
            return $parts[0].ToLowerInvariant()
        }
    }
    return $null
}

if ($SelfTest) {
    $name = Get-EposArchiveName -Tag "v1.2.3" -OS "windows" -Arch "amd64"
    if ($name -ne "epos_1.2.3_windows_amd64.zip") {
        throw "windows amd64 archive: got '$name'"
    }
    Write-Host "install.ps1 self-test: ok"
    exit 0
}

$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "epos-install" }
$tag = $release.tag_name
if (-not $tag) {
    throw "could not determine latest release"
}

$arch = Get-EposArch
$assetName = Get-EposArchiveName -Tag $tag -OS "windows" -Arch $arch
$asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) {
    throw "release $tag does not contain $assetName"
}

$checksums = $release.assets | Where-Object { $_.name -eq "checksums.txt" } | Select-Object -First 1
if (-not $checksums) {
    throw "release $tag does not contain checksums.txt"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("epos-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    $archivePath = Join-Path $tmp $assetName
    $checksumsPath = Join-Path $tmp "checksums.txt"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archivePath
    Invoke-WebRequest -Uri $checksums.browser_download_url -OutFile $checksumsPath

    $expected = Get-ChecksumForAsset -ChecksumsPath $checksumsPath -AssetName $assetName
    if (-not $expected) {
        throw "no checksum found for $assetName"
    }
    $actual = (Get-FileHash -Algorithm SHA256 -Path $archivePath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "checksum mismatch for $assetName"
    }

    $extractDir = Join-Path $tmp "extract"
    Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
    $binary = Join-Path $extractDir "epos.exe"
    if (-not (Test-Path $binary)) {
        throw "archive did not contain epos.exe"
    }

    $installDir = if ($env:EPOS_INSTALL_DIR) {
        $env:EPOS_INSTALL_DIR
    } else {
        Join-Path $env:LOCALAPPDATA "Programs\epos\bin"
    }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item -Path $binary -Destination (Join-Path $installDir "epos.exe") -Force
    Write-Host "Installed epos to $installDir\epos.exe"

    $pathEntries = [Environment]::GetEnvironmentVariable("PATH", "User") -split [IO.Path]::PathSeparator
    if ($pathEntries -notcontains $installDir) {
        Write-Host "Add this directory to your PATH: $installDir"
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
