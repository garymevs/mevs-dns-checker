#Requires -Version 5.1
<#
.SYNOPSIS
  Install mevs-dns-checker from GitHub Releases (or build from source).

.EXAMPLE
  irm https://raw.githubusercontent.com/garymevs/mevs-dns-checker/master/scripts/install.ps1 | iex

.EXAMPLE
  .\scripts\install.ps1 -FromSource
  .\scripts\install.ps1 -Version v0.1.0
#>
[CmdletBinding()]
param(
  [string]$Repo = "garymevs/mevs-dns-checker",
  [string]$RepoUrl = "",
  [string]$BinDir = (Join-Path $env:LOCALAPPDATA "mevs-dns-checker\bin"),
  [string]$Version = "latest",
  [switch]$FromSource
)

$ErrorActionPreference = "Stop"
$BinName = "dnscheck.exe"
if (-not $RepoUrl) {
  $RepoUrl = "https://github.com/$Repo.git"
}

function Write-Step([string]$Message) {
  Write-Host "==> $Message"
}

function Assert-Command([string]$Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "'$Name' is required but was not found in PATH."
  }
}

function Add-ToUserPath([string]$Directory) {
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if ([string]::IsNullOrEmpty($userPath)) { $userPath = "" }
  $parts = $userPath -split ";" | Where-Object { $_ -ne "" }
  if ($parts -contains $Directory) {
    Write-Step "PATH already includes $Directory"
  }
  else {
    Write-Step "Adding $Directory to user PATH"
    $newPath = if ($userPath.Trim() -eq "") { $Directory } else { "$Directory;$userPath" }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  }
  if (-not (($env:Path -split ";") -contains $Directory)) {
    $env:Path = "$Directory;$env:Path"
  }
}

function Get-ReleaseAssetUrl {
  param([string]$ApiUrl, [string]$AssetName)
  $headers = @{
    Accept                 = "application/vnd.github+json"
    "User-Agent"           = "mevs-dns-checker-install"
  }
  try {
    $release = Invoke-RestMethod -Uri $ApiUrl -Headers $headers
  }
  catch {
    throw "Could not fetch release info from $ApiUrl. Publish a tagged release, or use -FromSource. $($_.Exception.Message)"
  }
  $asset = $release.assets | Where-Object { $_.name -eq $AssetName } | Select-Object -First 1
  if (-not $asset) {
    throw "No asset named $AssetName in release $($release.tag_name)."
  }
  return @{
    Url = $asset.browser_download_url
    Tag = $release.tag_name
  }
}

function Install-FromRelease {
  $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
  switch -Regex ($arch) {
    "X64|Amd64" { $assetArch = "x86_64" }
    "Arm64" { $assetArch = "arm64" }
    default { throw "Unsupported architecture: $arch" }
  }
  $asset = "dnscheck_Windows_${assetArch}.zip"

  if ($Version -eq "latest") {
    $api = "https://api.github.com/repos/$Repo/releases/latest"
  }
  else {
    $api = "https://api.github.com/repos/$Repo/releases/tags/$Version"
  }

  Write-Step "Fetching release metadata ($Version)"
  $meta = Get-ReleaseAssetUrl -ApiUrl $api -AssetName $asset
  Write-Step "Downloading $asset ($($meta.Tag))"

  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("mevs-dns-checker-" + [guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  try {
    $zipPath = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $meta.Url -OutFile $zipPath -UseBasicParsing
    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
    $built = Join-Path $tmp $BinName
    if (-not (Test-Path $built)) {
      throw "Archive did not contain $BinName"
    }
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Copy-Item -Force $built (Join-Path $BinDir $BinName)
    Write-Step "Installed $($meta.Tag): $(Join-Path $BinDir $BinName)"
  }
  finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
  }
}

function Install-FromSource {
  Assert-Command git
  Assert-Command go

  $src = Join-Path $env:LOCALAPPDATA "mevs-dns-checker\src"
  Write-Step "Source install from $RepoUrl"
  if (Test-Path (Join-Path $src ".git")) {
    $status = git -C $src status --porcelain
    if ($status) {
      Write-Step "Local changes detected — skipping pull, building current tree"
    }
    else {
      git -C $src fetch --quiet origin
      $branch = (git -C $src rev-parse --abbrev-ref HEAD).Trim()
      git -C $src merge --ff-only --quiet "origin/$branch" 2>$null
    }
  }
  else {
    New-Item -ItemType Directory -Force -Path (Split-Path $src -Parent) | Out-Null
    git clone --depth 1 $RepoUrl $src
  }

  Write-Step "Building $BinName"
  New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
  $out = Join-Path $BinDir $BinName
  Push-Location $src
  try {
    go build -trimpath -ldflags "-s -w" -o $out ./cmd/dnscheck
  }
  finally {
    Pop-Location
  }
  Write-Step "Installed: $out"
}

if ($FromSource) {
  Install-FromSource
}
else {
  Install-FromRelease
}

Add-ToUserPath $BinDir
$exe = Join-Path $BinDir $BinName
& $exe -version
Write-Host ""
Write-Host "Try: dnscheck example.com"
Write-Host "     dnscheck            # interactive TUI"
Write-Host "Open a new terminal if 'dnscheck' is not found yet (PATH refresh)."
