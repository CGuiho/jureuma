<#
.SYNOPSIS
Install jureuma, the GUIHO agent-safe configuration CLI.

.DESCRIPTION
Windows (PowerShell):
  irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1 | iex
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1))) -Version '0.1.0'
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1))) -Channel 'beta'

Installs the verified release payload into the immutable directory
~/.guiho/jureuma/versions/<version>/, the stable command ~/.guiho/bin/jureuma.exe,
the active-version pointer ~/.guiho/jureuma/current.json, the installed artifact
manifest, the guiho-s-jureuma agent skill copies and (inside a project) the
jureuma block in AGENTS.md, and adds ~/.guiho/bin to the user PATH. Downloads
and verification happen in ~/.guiho/.temp/jureuma-install-<id>/. Any failure
rolls back to the previous installation. Configuration, keys, state and the
audit log are never touched. The outcome matches devops/install.sh.

The script also runs under PowerShell 7 on macOS and Linux, where it installs
the same layout as install.sh (including the POSIX launcher script).

Environment overrides (testing and mirrors only):
  JUREUMA_RELEASES_URL       release catalog API (default: GitHub releases API)
  JUREUMA_DOWNLOAD_BASE_URL  asset base; assets are <base>/<tag>/<asset>
#>
param(
  [string]$Version = '',
  [string]$Channel = '',
  [switch]$DryRun,
  [switch]$NoModifyPath,
  [switch]$Help
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Repo = 'CGuiho/jureuma'
$SkillId = 'guiho-s-jureuma'
$AgentAssets = @('guiho-s-jureuma.zip', 'jureuma-instruction.md', 'guiho-p-jureuma.md', 'guiho-p-jureuma-install.md', 'guiho-p-jureuma-uninstall.md')
$SemverPattern = '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
$ReinstallSh = 'curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh'

function Show-Usage {
  Write-Output @'
Install jureuma, the GUIHO agent-safe configuration CLI.

Usage: install.ps1 [-Version <semver> | -Channel <name>] [-DryRun] [-NoModifyPath]

Options:
  -Version <semver>  install this exact release (for example 0.1.0 or 0.2.0-beta.1)
  -Channel <name>    install the newest release of a channel (stable, beta, canary, ...)
  -DryRun            resolve the release and print the plan; change nothing
  -NoModifyPath      do not add ~/.guiho/bin to the user PATH
  -Help              show this help

Without -Version and -Channel the latest stable release is installed.
Re-running the installer performs a complete, safe reinstall.
'@
}

if ($Help) { Show-Usage; return }

function Fail([string]$Message) { throw "jureuma install: $Message" }

$OnWindows = [Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT
if ($OnWindows -and $PSVersionTable.PSVersion.Major -lt 6) {
  [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
}

if ($Version -and $Channel) { Fail '-Version and -Channel are mutually exclusive' }
if ($Version) {
  $Version = $Version -replace '^v', ''
  if ($Version -notmatch $SemverPattern) { Fail "not a valid SemVer version: $Version" }
}
if ($Channel -and $Channel -notmatch '^[0-9A-Za-z-]+$') { Fail "not a valid channel name: $Channel" }
if (-not $Version -and -not $Channel) { $Channel = 'stable' }

# --- SemVer ------------------------------------------------------------------
function Get-Core([string]$V) { return ($V -split '\+', 2)[0] }
function Get-Channel([string]$V) {
  $c = Get-Core $V
  $i = $c.IndexOf('-')
  if ($i -lt 0) { return 'stable' }
  return ($c.Substring($i + 1) -split '\.')[0]
}
function Compare-Numeric([string]$A, [string]$B) {
  if ($A.Length -ne $B.Length) { if ($A.Length -lt $B.Length) { return -1 } else { return 1 } }
  return [Math]::Sign([string]::CompareOrdinal($A, $B))
}
function Compare-Semver([string]$A, [string]$B) {
  $a = Get-Core $A; $b = Get-Core $B
  $pa = ''; $pb = ''
  $i = $a.IndexOf('-'); if ($i -ge 0) { $pa = $a.Substring($i + 1); $a = $a.Substring(0, $i) }
  $i = $b.IndexOf('-'); if ($i -ge 0) { $pb = $b.Substring($i + 1); $b = $b.Substring(0, $i) }
  $xa = $a -split '\.'; $xb = $b -split '\.'
  for ($k = 0; $k -lt 3; $k++) { $c = Compare-Numeric $xa[$k] $xb[$k]; if ($c -ne 0) { return $c } }
  if ($pa -eq $pb) { return 0 }
  if ($pa -eq '') { return 1 }
  if ($pb -eq '') { return -1 }
  $ia = $pa -split '\.'; $ib = $pb -split '\.'
  for ($k = 0; $k -lt [Math]::Min($ia.Count, $ib.Count); $k++) {
    $x = $ia[$k]; $y = $ib[$k]
    if ($x -ceq $y) { continue }
    $nx = $x -match '^[0-9]+$'; $ny = $y -match '^[0-9]+$'
    if ($nx -and $ny) { return Compare-Numeric $x $y }
    if ($nx) { return -1 }
    if ($ny) { return 1 }
    return [Math]::Sign([string]::CompareOrdinal($x, $y))
  }
  return [Math]::Sign($ia.Count - $ib.Count)
}

# --- Paths -------------------------------------------------------------------
if ($OnWindows) { $UserHome = [Environment]::GetFolderPath('UserProfile') } else { $UserHome = $env:HOME }
if ([string]::IsNullOrWhiteSpace($UserHome)) { Fail 'cannot resolve the user home directory' }
if (-not [IO.Path]::IsPathRooted($UserHome)) { Fail "the user home is not an absolute path: $UserHome" }
if (-not (Test-Path -LiteralPath $UserHome -PathType Container)) { Fail "the user home does not exist: $UserHome" }
$UserHome = [IO.Path]::GetFullPath($UserHome).TrimEnd([char[]]@('\', '/'))
$Guiho = Join-Path $UserHome '.guiho'
$BinDir = Join-Path $Guiho 'bin'
$TempRoot = Join-Path $Guiho '.temp'
$JHome = Join-Path $Guiho 'jureuma'
$VersionsDir = Join-Path $JHome 'versions'
$ExeName = if ($OnWindows) { 'jureuma.exe' } else { 'jureuma' }
$Launcher = Join-Path $BinDir $ExeName

# --- Platform ----------------------------------------------------------------
if ($OnWindows) {
  $os = 'windows'
  $procArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
  switch (([string]$procArch).ToUpperInvariant()) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { Fail "unsupported Windows architecture: $procArch" }
  }
} else {
  $ri = [System.Runtime.InteropServices.RuntimeInformation]
  if ($ri::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Linux)) { $os = 'linux' }
  elseif ($ri::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::OSX)) { $os = 'darwin' }
  else { Fail 'unsupported operating system' }
  switch ([string]$ri::OSArchitecture) {
    'X64' { $arch = 'amd64' }
    'Arm64' { $arch = 'arm64' }
    default { Fail "unsupported architecture: $($ri::OSArchitecture)" }
  }
}
$Asset = "jureuma-$os-$arch"
if ($OnWindows) { $Asset = "$Asset.exe" }

# --- Release selection across every catalog page -----------------------------
$ReleasesUrl = if ($env:JUREUMA_RELEASES_URL) { $env:JUREUMA_RELEASES_URL } else { "https://api.github.com/repos/$Repo/releases" }
$DownloadBase = if ($env:JUREUMA_DOWNLOAD_BASE_URL) { $env:JUREUMA_DOWNLOAD_BASE_URL.TrimEnd('/') } else { "https://github.com/$Repo/releases/download" }
$Headers = @{ 'Accept' = 'application/vnd.github+json'; 'User-Agent' = 'jureuma-installer' }
$Required = @($Asset, 'checksums.txt', 'artifacts.json') + $AgentAssets

Write-Output 'jureuma installer'
Write-Output "Resolving the release catalog ($ReleasesUrl)..."
$best = $null; $bestTag = $null
for ($page = 1; $page -le 50; $page++) {
  $sep = if ($ReleasesUrl.Contains('?')) { '&' } else { '?' }
  try {
    $raw = Invoke-WebRequest -Uri "$ReleasesUrl${sep}per_page=100&page=$page" -Headers $Headers -UseBasicParsing
    $releases = @($raw.Content | ConvertFrom-Json)
  } catch {
    Fail "the release catalog request failed (page $page); nothing was changed"
  }
  if ($releases.Count -eq 1 -and $releases[0] -is [array]) { $releases = @($releases[0]) }
  foreach ($r in $releases) {
    if ($null -eq $r -or -not ($r.PSObject.Properties.Name -contains 'tag_name')) { continue }
    if ($r.PSObject.Properties.Name -contains 'draft' -and $r.draft -eq $true) { continue }
    $v = ([string]$r.tag_name) -replace '^v', ''
    if ($v -notmatch $SemverPattern) { continue }
    $names = @()
    if ($r.PSObject.Properties.Name -contains 'assets') { $names = @($r.assets | ForEach-Object { [string]$_.name }) }
    $complete = $true
    foreach ($q in $Required) { if ($names -notcontains $q) { $complete = $false; break } }
    if (-not $complete) { continue }
    if ($Version) {
      if ((Get-Core $v) -eq (Get-Core $Version)) { $best = $v; $bestTag = [string]$r.tag_name }
      continue
    }
    if ((Get-Channel $v) -ne $Channel) { continue }
    if ($null -eq $best -or (Compare-Semver $v $best) -gt 0) { $best = $v; $bestTag = [string]$r.tag_name }
  }
  if ($releases.Count -lt 100) { break }
}
if ($null -eq $best) {
  if ($Version) { Fail "no published release $Version with every required asset for $os/$arch; nothing was changed" }
  Fail "no published $Channel release with every required asset for $os/$arch; nothing was changed"
}
$TargetVersion = $best
$Tag = $bestTag
if ($Tag -notmatch '^[0-9A-Za-z.+-]+$') { Fail 'the release tag contains unexpected characters' }
$BaseUrl = "$DownloadBase/$Tag"

$CurrentPath = Join-Path $JHome 'current.json'
$CurrentActive = ''
$CurrentPrevious = ''
if (Test-Path -LiteralPath $CurrentPath -PathType Leaf) {
  $text = [IO.File]::ReadAllText($CurrentPath)
  if ($text -match '^\{"protocol":1,"active":"([^"]+)","previous":(null|"([^"]+)")\}\s*$') {
    $CurrentActive = $Matches[1]
    if ($Matches.Count -gt 3 -and $Matches[3]) { $CurrentPrevious = $Matches[3] }
  }
}

Write-Output ''
Write-Output "Version:      $TargetVersion (tag $Tag)"
Write-Output "Platform:     $os/$arch"
Write-Output "Asset:        $Asset"
Write-Output "Source URL:   $BaseUrl/$Asset"
Write-Output "Payload:      $(Join-Path (Join-Path $VersionsDir $TargetVersion) $ExeName)"
Write-Output "Launcher:     $Launcher"
Write-Output "CLI home:     $JHome"
if ($CurrentActive) { Write-Output "Installed:    $CurrentActive (reinstall/upgrade; configuration, keys, state and audit log are preserved)" }

function Test-PathListContains([string]$PathValue, [string]$Directory) {
  if ([string]::IsNullOrWhiteSpace($PathValue)) { return $false }
  $want = $Directory.TrimEnd([char[]]@('\', '/'))
  $splitChar = [IO.Path]::PathSeparator
  foreach ($e in ($PathValue -split [Regex]::Escape([string]$splitChar))) {
    if ([string]::IsNullOrWhiteSpace($e)) { continue }
    $x = [Environment]::ExpandEnvironmentVariables($e.Trim().Trim('"')).TrimEnd([char[]]@('\', '/'))
    if ($OnWindows) { if ([string]::Equals($x, $want, [StringComparison]::OrdinalIgnoreCase)) { return $true } }
    elseif ($x -ceq $want) { return $true }
  }
  return $false
}
$PathPresent = Test-PathListContains $env:PATH $BinDir
$ProjectRoot = (Get-Location).ProviderPath
$ApplyInstruction = (Test-Path -LiteralPath (Join-Path $ProjectRoot 'jureuma.yaml') -PathType Leaf) -or (
  (Test-Path -LiteralPath (Join-Path $ProjectRoot 'AGENTS.md') -PathType Leaf) -and (Test-Path -LiteralPath (Join-Path $ProjectRoot '.git')))

if ($DryRun) {
  Write-Output ''
  Write-Output 'Dry run: the plan would'
  Write-Output "  download and verify $Asset, artifacts.json, checksums.txt and the agent artifacts"
  Write-Output "  install $(Join-Path $VersionsDir $TargetVersion) (immutable) and point $CurrentPath at it"
  Write-Output "  write the stable command $Launcher"
  Write-Output "  install the $SkillId skill into the supported global agent-skill directories"
  if ($ApplyInstruction) { Write-Output "  apply the jureuma block in $(Join-Path $ProjectRoot 'AGENTS.md')" }
  if (-not $PathPresent -and -not $NoModifyPath) { Write-Output "  add $BinDir to the user PATH" }
  Write-Output 'Dry run: nothing was changed.'
  return
}

# --- Helpers -----------------------------------------------------------------
$Utf8NoBom = New-Object System.Text.UTF8Encoding -ArgumentList $false
function Get-Sha256([string]$Path) { return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant() }
function Invoke-Quiet([string]$Exe, [string[]]$Arguments) {
  $ErrorActionPreference = 'Continue'  # native stderr must not become a terminating error
  $old = $env:JUREUMA_NO_UPDATE_CHECK
  $env:JUREUMA_NO_UPDATE_CHECK = '1'
  try {
    $out = & $Exe @Arguments 2>$null
    return [pscustomobject]@{ Code = $LASTEXITCODE; Output = (($out | Out-String).Trim()) }
  } finally { $env:JUREUMA_NO_UPDATE_CHECK = $old }
}

# --- Staging -----------------------------------------------------------------
New-Item -ItemType Directory -Force -Path $TempRoot | Out-Null
$Id = [Guid]::NewGuid().ToString('N').Substring(0, 12)
$Stage = Join-Path $TempRoot "jureuma-install-$Id"
$StageFull = [IO.Path]::GetFullPath($Stage)
$TempPrefix = [IO.Path]::GetFullPath($TempRoot).TrimEnd([char[]]@('\', '/')) + [IO.Path]::DirectorySeparatorChar
if (-not $StageFull.StartsWith($TempPrefix + 'jureuma-install-', [StringComparison]::Ordinal)) { Fail 'the staging directory is outside ~/.guiho/.temp' }
New-Item -ItemType Directory -Path $Stage | Out-Null
$Dl = Join-Path $Stage 'dl'
New-Item -ItemType Directory -Path $Dl | Out-Null

$T = @{
  Committed = $false; CreatedVersion = $false; VersionBackup = $null
  LauncherBackup = $null; LauncherWritten = $false
  CurrentBackup = $null; CurrentWritten = $false
  ManifestBackup = $null; ManifestWritten = $false
  SkillBackups = @(); SkillsWritten = @()
  AgentsTarget = $null; AgentsBackup = $null
}
$VersionDir = Join-Path $VersionsDir $TargetVersion
$CandidateDir = Join-Path $VersionsDir ".candidate-$Id"
$InstalledManifest = Join-Path $JHome 'installed-artifacts.json'

function Undo-Install {
  Write-Warning 'Rolling back to the previous installation...'
  try {
    if ($T.AgentsTarget -and $T.AgentsBackup -and (Test-Path -LiteralPath $T.AgentsBackup)) { Copy-Item -LiteralPath $T.AgentsBackup -Destination $T.AgentsTarget -Force }
    if ($T.CurrentWritten) {
      if ($T.CurrentBackup) { Copy-Item -LiteralPath $T.CurrentBackup -Destination $CurrentPath -Force } else { Remove-Item -LiteralPath $CurrentPath -Force -ErrorAction SilentlyContinue }
    }
    if ($T.ManifestWritten) {
      if ($T.ManifestBackup) { Copy-Item -LiteralPath $T.ManifestBackup -Destination $InstalledManifest -Force } else { Remove-Item -LiteralPath $InstalledManifest -Force -ErrorAction SilentlyContinue }
    }
    if ($T.LauncherWritten) {
      Remove-Item -LiteralPath $Launcher -Force -ErrorAction SilentlyContinue
      if ($T.LauncherBackup) { Move-Item -LiteralPath $T.LauncherBackup -Destination $Launcher -Force }
    }
    foreach ($s in $T.SkillsWritten) { if (Test-Path -LiteralPath $s) { Remove-Item -LiteralPath $s -Recurse -Force } }
    foreach ($b in $T.SkillBackups) { if (Test-Path -LiteralPath $b.From) { Move-Item -LiteralPath $b.From -Destination $b.To -Force } }
    if ($T.CreatedVersion -and (Test-Path -LiteralPath $VersionDir)) { Remove-Item -LiteralPath $VersionDir -Recurse -Force }
    if ($T.VersionBackup -and (Test-Path -LiteralPath $T.VersionBackup)) { Move-Item -LiteralPath $T.VersionBackup -Destination $VersionDir }
  } catch {
    Write-Warning "rollback was incomplete: $($_.Exception.Message). Reinstall with the install command."
  }
}

try {
  # --- Download and verify -----------------------------------------------------
  function Get-Asset([string]$Name) {
    if ($Name -match '[\\/]' -or $Name -eq '.' -or $Name -eq '..') { Fail "unsafe asset name: $Name" }
    try { Invoke-WebRequest -Uri "$BaseUrl/$Name" -OutFile (Join-Path $Dl $Name) -Headers @{ 'User-Agent' = 'jureuma-installer' } -UseBasicParsing }
    catch { Fail "download failed: $Name" }
  }
  Write-Output ''
  Write-Output 'Downloading...'
  Get-Asset 'checksums.txt'
  Get-Asset 'artifacts.json'
  $Sums = @{}
  $lines = @(Get-Content -LiteralPath (Join-Path $Dl 'checksums.txt') | Where-Object { $_ -ne '' })
  if ($lines.Count -eq 0) { Fail 'checksums.txt is empty' }
  foreach ($line in $lines) {
    if ($line -notmatch '^([0-9A-Fa-f]{64}) [ *]?([^\s]+)$') { Fail 'checksums.txt is malformed' }
    $name = $Matches[2]
    if ($Sums.ContainsKey($name)) { Fail "checksums.txt has duplicate entries for $name" }
    $Sums[$name] = $Matches[1].ToLowerInvariant()
  }
  function Assert-Checksum([string]$Name) {
    if (-not $Sums.ContainsKey($Name)) { Fail "checksums.txt has no entry for $Name" }
    if ((Get-Sha256 (Join-Path $Dl $Name)) -ne $Sums[$Name]) { Fail "SHA-256 mismatch for $Name" }
  }
  Assert-Checksum 'artifacts.json'
  $Manifest = [IO.File]::ReadAllText((Join-Path $Dl 'artifacts.json')) | ConvertFrom-Json
  if ([string]$Manifest.version -ne $TargetVersion) { Fail "artifacts.json declares version $($Manifest.version), expected $TargetVersion" }
  $ToInstall = @($Manifest.artifacts | Where-Object { $_.kind -ne 'binary' -or $_.asset -eq $Asset })
  $names = @($ToInstall | ForEach-Object { [string]$_.asset })
  foreach ($q in @($Asset) + $AgentAssets) { if ($names -notcontains $q) { Fail "artifacts.json does not declare $q" } }
  foreach ($a in $ToInstall) {
    Get-Asset ([string]$a.asset)
    Assert-Checksum ([string]$a.asset)
    if (([string]$a.sha256).ToLowerInvariant() -ne $Sums[[string]$a.asset]) { Fail "artifacts.json and checksums.txt disagree about $($a.asset)" }
  }
  Write-Output '[OK] SHA-256 verification complete.'

  # Skill archive: every entry must stay below guiho-s-jureuma/.
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $zipPath = Join-Path $Dl 'guiho-s-jureuma.zip'
  $zip = [IO.Compression.ZipFile]::OpenRead($zipPath)
  try {
    foreach ($e in $zip.Entries) {
      $n = $e.FullName.Replace('\', '/')
      if (-not $n.StartsWith("$SkillId/") -or $n -match '(^|/)\.\.(/|$)' -or $n.StartsWith('/')) { Fail 'guiho-s-jureuma.zip contains unexpected paths' }
    }
  } finally { $zip.Dispose() }
  $SkillStage = Join-Path $Stage 'skill'
  Expand-Archive -LiteralPath $zipPath -DestinationPath $SkillStage
  if (-not (Test-Path -LiteralPath (Join-Path (Join-Path $SkillStage $SkillId) 'SKILL.md'))) { Fail "guiho-s-jureuma.zip is missing $SkillId/SKILL.md" }

  # Candidate checks.
  $CandidateExe = Join-Path $Dl $Asset
  if (-not $OnWindows) { & chmod 0755 -- $CandidateExe }
  $r = Invoke-Quiet $CandidateExe @('--version')
  if ($r.Code -ne 0) { Fail 'the downloaded payload failed to run' }
  if ($r.Output -ne $TargetVersion) { Fail "the downloaded payload reports version $($r.Output), expected $TargetVersion" }
  $r = Invoke-Quiet $CandidateExe @('__self-test')
  if ($r.Code -ne 0) { Fail 'the downloaded payload failed its self-test' }
  Write-Output "[OK] Payload self-test passed ($TargetVersion)."

  # Immutable version directory.
  New-Item -ItemType Directory -Force -Path $VersionsDir | Out-Null
  $art = Join-Path $CandidateDir 'artifacts'
  foreach ($d in @('skills', 'prompts', 'instruction', 'schemas', 'examples')) { New-Item -ItemType Directory -Force -Path (Join-Path $art $d) | Out-Null }
  Copy-Item -LiteralPath $CandidateExe -Destination (Join-Path $CandidateDir $ExeName)
  Copy-Item -LiteralPath (Join-Path $SkillStage $SkillId) -Destination (Join-Path (Join-Path $art 'skills') $SkillId) -Recurse
  foreach ($p in @('guiho-p-jureuma', 'guiho-p-jureuma-install', 'guiho-p-jureuma-uninstall')) { Copy-Item -LiteralPath (Join-Path $Dl "$p.md") -Destination (Join-Path (Join-Path $art 'prompts') "$p.md") }
  Copy-Item -LiteralPath (Join-Path $Dl 'jureuma-instruction.md') -Destination (Join-Path (Join-Path $art 'instruction') 'jureuma-instruction.md')
  foreach ($f in @('jureuma.schema.json', 'jureuma.global.schema.json')) { if (Test-Path -LiteralPath (Join-Path $Dl $f)) { Copy-Item -LiteralPath (Join-Path $Dl $f) -Destination (Join-Path (Join-Path $art 'schemas') $f) } }
  foreach ($f in @('jureuma.example.yaml', 'jureuma.global.example.yaml')) { if (Test-Path -LiteralPath (Join-Path $Dl $f)) { Copy-Item -LiteralPath (Join-Path $Dl $f) -Destination (Join-Path (Join-Path $art 'examples') $f) } }
  Copy-Item -LiteralPath (Join-Path $Dl 'artifacts.json') -Destination (Join-Path $art 'artifacts.json')
  Copy-Item -LiteralPath (Join-Path $Dl 'checksums.txt') -Destination (Join-Path $art 'checksums.txt')
  if (-not $OnWindows) {
    Get-ChildItem -LiteralPath $CandidateDir -Recurse -File | ForEach-Object { & chmod a-w -- $_.FullName }
    & chmod 0555 -- (Join-Path $CandidateDir $ExeName)
  }
  if (Test-Path -LiteralPath $VersionDir) {
    $T.VersionBackup = Join-Path $Stage 'backup-version'
    try { Move-Item -LiteralPath $VersionDir -Destination $T.VersionBackup }
    catch { $T.VersionBackup = $null; Fail "cannot replace $VersionDir; stop running jureuma processes and retry" }
  }
  Move-Item -LiteralPath $CandidateDir -Destination $VersionDir
  $T.CreatedVersion = $true
  $Payload = Join-Path $VersionDir $ExeName

  # Stable command in ~/.guiho/bin.
  New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
  if (Test-Path -LiteralPath $Launcher) {
    $T.LauncherBackup = Join-Path $Stage 'backup-launcher'
    Move-Item -LiteralPath $Launcher -Destination $T.LauncherBackup
  }
  $T.LauncherWritten = $true
  if ($OnWindows) {
    # No native launcher exists yet: the command is a verified copy of the active payload.
    Copy-Item -LiteralPath $Payload -Destination $Launcher -Force
  } else {
    $launcherText = @'
#!/bin/sh
# jureuma stable launcher (managed by the jureuma installer; do not edit).
# It runs the payload selected by ~/.guiho/jureuma/current.json with the
# original arguments and standard streams, and returns its exact exit code.
self=$0
case "$self" in */*) ;; *) self=$(command -v -- "$self" 2>/dev/null || printf '%s' "$self") ;; esac
bin_dir=$(CDPATH='' cd -- "$(dirname -- "$self")" 2>/dev/null && pwd) || bin_dir=
home_dir=
if [ -n "$bin_dir" ]; then home_dir="$(dirname -- "$bin_dir")/jureuma"; fi
if [ -z "$home_dir" ] || [ ! -f "$home_dir/current.json" ]; then home_dir="${HOME:-}/.guiho/jureuma"; fi
pointer=$(cat -- "$home_dir/current.json" 2>/dev/null) || pointer=
semver='(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?'
if ! printf '%s\n' "$pointer" | grep -Eqx "\{\"protocol\":1,\"active\":\"$semver\",\"previous\":(null|\"$semver\")\}"; then
  echo 'jureuma: no valid active installation (current.json); reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh' >&2
  exit 1
fi
active=$(printf '%s\n' "$pointer" | sed 's/^{"protocol":1,"active":"\([^"]*\)".*/\1/')
payload="$home_dir/versions/$active/jureuma"
if [ ! -x "$payload" ]; then
  # Fall back to the previous verified payload when the active one is gone.
  previous=$(printf '%s\n' "$pointer" | sed -n 's/.*"previous":"\([^"]*\)".*/\1/p')
  if [ -n "$previous" ] && [ -x "$home_dir/versions/$previous/jureuma" ]; then
    echo "jureuma: warning: the active payload $active is missing; running the previous version $previous (reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh)" >&2
    exec "$home_dir/versions/$previous/jureuma" "$@"
  fi
  echo "jureuma: the active payload $active is missing; reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh" >&2
  exit 1
fi
exec "$payload" "$@"
'@
    [IO.File]::WriteAllText($Launcher, $launcherText.Replace("`r`n", "`n"), $Utf8NoBom)
    & chmod 0755 -- $Launcher
  }

  # Installed artifact manifest.
  $previousManifest = $null
  if (Test-Path -LiteralPath $InstalledManifest) {
    $T.ManifestBackup = Join-Path $Stage 'backup-installed-artifacts.json'
    Copy-Item -LiteralPath $InstalledManifest -Destination $T.ManifestBackup
    $previousManifest = $T.ManifestBackup
  }
  $T.ManifestWritten = $true
  Copy-Item -LiteralPath (Join-Path $Dl 'artifacts.json') -Destination $InstalledManifest -Force

  # Agent skill projections: replace ours, remove retired ones.
  $skillRoots = @()
  if (Test-Path -LiteralPath (Join-Path $UserHome '.claude') -PathType Container) { $skillRoots += (Join-Path (Join-Path $UserHome '.claude') 'skills') }
  $skillRoots += (Join-Path (Join-Path $UserHome '.agents') 'skills')
  if (Test-Path -LiteralPath (Join-Path $UserHome '.codex') -PathType Container) { $skillRoots += (Join-Path (Join-Path $UserHome '.codex') 'skills') }
  $ids = @($SkillId)
  if ($previousManifest) {
    try {
      $old = [IO.File]::ReadAllText($previousManifest) | ConvertFrom-Json
      foreach ($a in @($old.artifacts | Where-Object { $_.kind -eq 'skill' })) {
        $oldId = Split-Path -Leaf ([string]$a.installedPath)
        if ($oldId -match '^[A-Za-z0-9._-]+$' -and $ids -notcontains $oldId) { $ids += $oldId }
      }
    } catch { Write-Warning 'the previous installed-artifacts.json is unreadable; retired skills were not checked' }
  }
  $n = 0
  foreach ($root in $skillRoots) {
    foreach ($sid in $ids) {
      $target = Join-Path $root $sid
      if (Test-Path -LiteralPath $target) {
        $n++
        $bk = Join-Path $Stage "backup-skill-$n"
        Move-Item -LiteralPath $target -Destination $bk
        $T.SkillBackups += [pscustomobject]@{ From = $bk; To = $target }
      }
    }
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    $dest = Join-Path $root $SkillId
    $T.SkillsWritten += $dest
    Copy-Item -LiteralPath (Join-Path (Join-Path (Join-Path $VersionDir 'artifacts') 'skills') $SkillId) -Destination $dest -Recurse
    if (-not $OnWindows) { & chmod -R u+w -- $dest }
  }

  # Activate atomically and verify through the stable command.
  $previous = ''
  if (Test-Path -LiteralPath $CurrentPath) {
    $T.CurrentBackup = Join-Path $Stage 'backup-current.json'
    Copy-Item -LiteralPath $CurrentPath -Destination $T.CurrentBackup
    $previous = if ($CurrentActive -eq $TargetVersion) { $CurrentPrevious } else { $CurrentActive }
  }
  if ($previous -and $previous -match $SemverPattern -and (Test-Path -LiteralPath (Join-Path (Join-Path $VersionsDir $previous) $ExeName))) { $prevJson = "`"$previous`"" } else { $previous = ''; $prevJson = 'null' }
  $tmpCurrent = Join-Path $JHome ".current.json.$Id"
  [IO.File]::WriteAllText($tmpCurrent, "{`"protocol`":1,`"active`":`"$TargetVersion`",`"previous`":$prevJson}`n", $Utf8NoBom)
  $T.CurrentWritten = $true
  Move-Item -LiteralPath $tmpCurrent -Destination $CurrentPath -Force
  $r = Invoke-Quiet $Launcher @('--version')
  if ($r.Code -ne 0 -or $r.Output -ne $TargetVersion) { Fail "the stable command reports '$($r.Output)', expected $TargetVersion" }
  Write-Output "[OK] Activated $TargetVersion through $Launcher."

  # Managed AGENTS.md block, only when run from a project root.
  if ($ApplyInstruction) {
    $T.AgentsTarget = Join-Path $ProjectRoot 'AGENTS.md'
    if (Test-Path -LiteralPath $T.AgentsTarget -PathType Leaf) {
      $T.AgentsBackup = Join-Path $Stage 'backup-AGENTS.md'
      Copy-Item -LiteralPath $T.AgentsTarget -Destination $T.AgentsBackup
    }
    $r = Invoke-Quiet $Launcher @('agent', 'instruction', 'apply')
    if ($r.Code -ne 0) { Fail 'could not apply the jureuma block to AGENTS.md' }
  }
  $T.Committed = $true
} catch {
  if (-not $T.Committed) { Undo-Install }
  throw
} finally {
  foreach ($p in @((Join-Path $JHome ".current.json.$Id"), $CandidateDir)) {
    if (Test-Path -LiteralPath $p) { Remove-Item -LiteralPath $p -Recurse -Force -ErrorAction SilentlyContinue }
  }
  if ((Test-Path -LiteralPath $Stage) -and ([IO.Path]::GetFullPath($Stage)).StartsWith($TempPrefix + 'jureuma-install-', [StringComparison]::Ordinal)) {
    Remove-Item -LiteralPath $Stage -Recurse -Force -ErrorAction SilentlyContinue
  }
}

# --- PATH (user level, idempotent) -------------------------------------------
$pathNote = ''
if (-not $NoModifyPath) {
  if ($OnWindows) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not (Test-PathListContains $userPath $BinDir)) {
      $newPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $BinDir } else { "$BinDir;$userPath" }
      [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
      Write-Output "[OK] Added $BinDir to the user Path."
    }
    if (-not $PathPresent) { $env:Path = "$BinDir;$env:Path" }
    # Git Bash reads ~/.bashrc, not the Windows user Path of already-open sessions.
    $bashrc = Join-Path $UserHome '.bashrc'
    $existing = if (Test-Path -LiteralPath $bashrc) { [IO.File]::ReadAllText($bashrc) } else { '' }
    if ($existing -notmatch '\.guiho/bin') {
      $nl = if ($existing.Contains("`r`n")) { "`r`n" } else { "`n" }
      $prefix = if ($existing.Length -eq 0 -or $existing.EndsWith("`n")) { '' } else { $nl }
      [IO.File]::AppendAllText($bashrc, "$prefix# Added by the jureuma installer (GUIHO shared bin directory).$nl" + 'export PATH="$HOME/.guiho/bin:$PATH"' + $nl, $Utf8NoBom)
      Write-Output "[OK] Added ~/.guiho/bin to Git Bash startup ($bashrc)."
    }
    if (-not $PathPresent) { $pathNote = 'Open a new terminal so the updated user Path applies everywhere.' }
  } else {
    $files = @(Join-Path $UserHome '.profile')
    foreach ($f in @('.bashrc', '.bash_profile', '.zshrc', '.zprofile')) { if (Test-Path -LiteralPath (Join-Path $UserHome $f)) { $files += (Join-Path $UserHome $f) } }
    if ($os -eq 'darwin' -and -not (Test-Path -LiteralPath (Join-Path $UserHome '.zshrc'))) { $files += (Join-Path $UserHome '.zshrc') }
    foreach ($f in $files) {
      if ((Test-Path -LiteralPath $f) -and ([IO.File]::ReadAllText($f) -match '\.guiho/bin')) { continue }
      [IO.File]::AppendAllText($f, "`n# Added by the jureuma installer (GUIHO shared bin directory).`n" + 'export PATH="$HOME/.guiho/bin:$PATH"' + "`n", $Utf8NoBom)
      Write-Output "[OK] Added ~/.guiho/bin to PATH in $f"
    }
    if (-not $PathPresent) { $pathNote = 'Open a new shell, or run: export PATH="$HOME/.guiho/bin:$PATH"' }
  }
} elseif (-not $PathPresent) {
  $pathNote = "Add $BinDir to PATH to run jureuma by name."
}

# --- Disposable state and obsolete payloads ----------------------------------
foreach ($f in @('cache.json', 'update-worker.lease')) { Remove-Item -LiteralPath (Join-Path $JHome $f) -Force -ErrorAction SilentlyContinue }
foreach ($d in @(Get-ChildItem -LiteralPath $VersionsDir -Directory)) {
  if ($d.Name -notmatch $SemverPattern -or $d.Name -eq $TargetVersion -or ($previous -and $d.Name -eq $previous)) { continue }
  try { Remove-Item -LiteralPath $d.FullName -Recurse -Force; Write-Output "Removed obsolete payload $($d.Name)" }
  catch { Write-Warning "could not remove obsolete payload $($d.Name) (in use?)" }
}

Write-Output ''
Write-Output "[OK] Installed jureuma $TargetVersion"
Write-Output "  command:   $Launcher"
Write-Output "  payload:   $Payload"
Write-Output "  CLI home:  $JHome"
foreach ($root in $skillRoots) { Write-Output "  skill:     $(Join-Path $root $SkillId)" }
if ($T.AgentsTarget) { Write-Output "  AGENTS.md: $($T.AgentsTarget) (jureuma block)" }
if ($pathNote) { Write-Output $pathNote }
Write-Output 'Next: run `jureuma init` in your project root, then `jureuma --version`.'
