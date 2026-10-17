<#
.SYNOPSIS
Uninstall jureuma, the GUIHO agent-safe configuration CLI.

.DESCRIPTION
Windows (PowerShell):
  irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.ps1 | iex
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.ps1))) -DryRun
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.ps1))) -PreserveConfig -PreserveData

Same contract as `jureuma uninstall` and devops/uninstall.sh: by default every
jureuma-owned artifact is removed (stable command, juju alias, versioned
payloads, ~/.guiho/jureuma with configuration, state, audit log and caches, the
guiho-s-jureuma skill copies, the jureuma block in the project's AGENTS.md, the
deny rules jureuma added and the project's jureuma.yaml). Private keys
(~/.guiho/jureuma/keys) are removed only after the human-only confirmation in an
interactive console; a non-interactive run preserves them. The project
.gitignore block, the shared ~/.guiho, ~/.guiho/bin and ~/.guiho/.temp
directories and PATH are never touched. Run it from the project root.

When the installed payload works, this script delegates to it
(`jureuma uninstall`); otherwise it applies the same plan itself.
#>
param(
  [switch]$PreserveConfig,
  [switch]$PreserveData,
  [switch]$DryRun,
  [switch]$Yes,
  [switch]$Help
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

if ($Help) {
  Write-Output @'
Uninstall jureuma, the GUIHO agent-safe configuration CLI.

Usage: uninstall.ps1 [-PreserveConfig] [-PreserveData] [-DryRun] [-Yes]

By default uninstallation removes ALL jureuma-owned data: the stable command,
every installed version, ~/.guiho/jureuma (global configuration, state, audit
log, caches), the guiho-s-jureuma skill copies, the jureuma block in AGENTS.md and
the project's jureuma.yaml. Private keys are removed only after a human confirms
in an interactive console; back them up first with `jureuma key export`.

Options:
  -PreserveConfig  keep jureuma.global.yaml and the project's jureuma.yaml
  -PreserveData    keep keys/, state/ and audit.log
  -DryRun          print the REMOVE / PRESERVE plan; change nothing
  -Yes             confirm the plan without a prompt (keys still need a human)
  -Help            show this help
'@
  return
}

$SkillId = 'guiho-s-jureuma'
$BeginMarker = '<!-- >>> jureuma managed block >>> -->'
$EndMarker = '<!-- <<< jureuma managed block <<< -->'
$AgentMarkers = @('CLAUDECODE', 'CLAUDE_CODE_ENTRYPOINT', 'CODEX_SANDBOX', 'CODEX_HOME', 'CURSOR_AGENT', 'CURSOR_TRACE_ID', 'GEMINI_CLI', 'AIDER', 'OPENCODE', 'AMP_AGENT', 'CLINE', 'GOOSE_SESSION', 'WINDSURF_AGENT', 'COPILOT_AGENT', 'JUREUMA_AGENT')
$SemverPattern = '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

$OnWindows = [Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT
if ($OnWindows) { $UserHome = [Environment]::GetFolderPath('UserProfile') } else { $UserHome = $env:HOME }
if ([string]::IsNullOrWhiteSpace($UserHome) -or -not [IO.Path]::IsPathRooted($UserHome)) { throw 'jureuma uninstall: cannot resolve the user home directory' }
$UserHome = [IO.Path]::GetFullPath($UserHome).TrimEnd([char[]]@('\', '/'))
$Guiho = Join-Path $UserHome '.guiho'
$BinDir = Join-Path $Guiho 'bin'
$JHome = Join-Path $Guiho 'jureuma'
$ExeName = if ($OnWindows) { 'jureuma.exe' } else { 'jureuma' }
$Launcher = Join-Path $BinDir $ExeName
$JujuPath = Join-Path $BinDir $(if ($OnWindows) { 'juju.exe' } else { 'juju' })

# --- Delegate to the installed payload when it runs --------------------------
$payload = $null
if (-not $env:JUREUMA_UNINSTALL_NATIVE) {
  $currentPath = Join-Path $JHome 'current.json'
  if (Test-Path -LiteralPath $currentPath -PathType Leaf) {
    $text = [IO.File]::ReadAllText($currentPath)
    if ($text -match '^\{"protocol":1,"active":"([^"]+)","previous":') {
      $active = $Matches[1]
      $candidate = Join-Path (Join-Path (Join-Path $JHome 'versions') $active) $ExeName
      if ($active -match $SemverPattern -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
        $ErrorActionPreference = 'Continue'
        $env:JUREUMA_NO_UPDATE_CHECK = '1'
        & $candidate --version *> $null
        if ($LASTEXITCODE -eq 0) { $payload = $candidate }
        $ErrorActionPreference = 'Stop'
      }
    }
  }
}
if ($payload) {
  $argsList = @('uninstall')
  if ($PreserveConfig) { $argsList += '--preserve-config' }
  if ($PreserveData) { $argsList += '--preserve-data' }
  if ($DryRun) { $argsList += '--dry-run' }
  if ($Yes) { $argsList += '--yes' }
  Write-Output "Using the installed jureuma payload ($payload)."
  $env:JUREUMA_NO_UPDATE_CHECK = '1'
  & $payload @argsList
  # No `exit` here: under `irm ... | iex` it would close the caller's session.
  if ($LASTEXITCODE -ne 0) { throw "jureuma uninstall exited with code $LASTEXITCODE" }
  return
}

Write-Output 'The installed jureuma payload is unavailable; applying the uninstall plan directly.'

function Find-Up([string]$Name) {
  $d = (Get-Location).ProviderPath
  while ($d) {
    if (Test-Path -LiteralPath (Join-Path $d $Name)) { return $d }
    $parent = Split-Path -Parent $d
    if (-not $parent -or $parent -eq $d) { break }
    $d = $parent
  }
  return $null
}
$Root = Find-Up 'jureuma.yaml'
if (-not $Root) { $Root = Find-Up '.git' }
if (-not $Root) { $Root = (Get-Location).ProviderPath }

$interactive = [Environment]::UserInteractive -and -not [Console]::IsInputRedirected -and -not [Console]::IsErrorRedirected
$isAgent = $false
foreach ($m in $AgentMarkers) { if ([Environment]::GetEnvironmentVariable($m)) { $isAgent = $true } }

$plan = New-Object System.Collections.Generic.List[object]
function Add-Item([bool]$Remove, [string]$Kind, [string]$Path, [string]$Reason) {
  $plan.Add([pscustomobject]@{ Remove = $Remove; Kind = $Kind; Path = $Path; Reason = $Reason })
}

$denyState = Join-Path (Join-Path $JHome 'state') 'deny-rules.json'
if ((Test-Path -LiteralPath $denyState) -and ([IO.File]::ReadAllText($denyState).Contains(($Root | ConvertTo-Json)))) {
  Add-Item $false 'deny' (Join-Path (Join-Path $Root '.claude') 'settings.json') 'jureuma deny rules need the jureuma payload to be removed exactly; remove the *.unencrypted.* deny entries jureuma added by hand'
  Add-Item $false 'deny' (Join-Path $Root 'opencode.json') 'jureuma deny rules need the jureuma payload to be removed exactly; remove the *.unencrypted.* deny entries jureuma added by hand'
}
$aliasState = Join-Path (Join-Path $JHome 'state') 'alias.json'
if (Test-Path -LiteralPath $aliasState) {
  try {
    $alias = [IO.File]::ReadAllText($aliasState) | ConvertFrom-Json
    $ap = [string]$alias.path
    if ($ap -and (Test-Path -LiteralPath $ap -PathType Leaf)) {
      if ([IO.Path]::GetFullPath($ap) -ne [IO.Path]::GetFullPath($JujuPath)) {
        Add-Item $false 'file' $ap 'recorded juju alias is outside ~/.guiho/bin; not removed'
      } elseif ((Get-FileHash -Algorithm SHA256 -LiteralPath $ap).Hash.ToLowerInvariant() -eq ([string]$alias.sha256).ToLowerInvariant()) {
        Add-Item $true 'file' $ap 'juju alias'
      } else {
        Add-Item $false 'file' $ap 'juju alias changed since jureuma installed it; not removed'
      }
    }
  } catch { Write-Warning 'state/alias.json is unreadable; the juju alias was not checked' }
}
$skillRoots = @()
if (Test-Path -LiteralPath (Join-Path $UserHome '.claude') -PathType Container) { $skillRoots += (Join-Path (Join-Path $UserHome '.claude') 'skills') }
$skillRoots += (Join-Path (Join-Path $UserHome '.agents') 'skills')
if (Test-Path -LiteralPath (Join-Path $UserHome '.codex') -PathType Container) { $skillRoots += (Join-Path (Join-Path $UserHome '.codex') 'skills') }
foreach ($r in $skillRoots) {
  $t = Join-Path $r $SkillId
  if (Test-Path -LiteralPath $t -PathType Container) { Add-Item $true 'dir' $t 'managed skill copy' }
}
$agents = Join-Path $Root 'AGENTS.md'
if ((Test-Path -LiteralPath $agents -PathType Leaf) -and (@(Get-Content -LiteralPath $agents) -contains $BeginMarker)) {
  Add-Item $true 'block' $agents 'jureuma managed block only; the rest of AGENTS.md is kept'
}
$projCfg = Join-Path $Root 'jureuma.yaml'
if (Test-Path -LiteralPath $projCfg -PathType Leaf) {
  if ($PreserveConfig) { Add-Item $false 'file' $projCfg '--preserve-config' } else { Add-Item $true 'file' $projCfg 'project configuration' }
}
$gitignore = Join-Path $Root '.gitignore'
if ((Test-Path -LiteralPath $gitignore -PathType Leaf) -and ([IO.File]::ReadAllText($gitignore) -match 'jureuma')) {
  Add-Item $false 'file' $gitignore 'jureuma block is persistent project data: plaintext files may outlive jureuma'
}
if (Test-Path -LiteralPath $Launcher) { Add-Item $true 'file' $Launcher 'stable launcher' }
$preservedInHome = $false
$removeKeys = $false
$keysDir = Join-Path $JHome 'keys'
if (Test-Path -LiteralPath $JHome -PathType Container) {
  foreach ($c in @(Get-ChildItem -LiteralPath $JHome -Force)) {
    switch ($c.Name) {
      'keys' { continue }
      'jureuma.global.yaml' {
        if ($PreserveConfig) { Add-Item $false 'path' $c.FullName '--preserve-config'; $preservedInHome = $true }
        else { Add-Item $true 'path' $c.FullName 'global configuration' }
      }
      { $_ -eq 'state' -or $_ -eq 'audit.log' } {
        if ($PreserveData) { Add-Item $false 'path' $c.FullName '--preserve-data'; $preservedInHome = $true }
        else { Add-Item $true 'path' $c.FullName 'persistent data' }
      }
      default { Add-Item $true 'path' $c.FullName 'installation, cache or disposable state' }
    }
  }
  if (Test-Path -LiteralPath $keysDir) {
    if ($PreserveData) { Add-Item $false 'keys' $keysDir '--preserve-data'; $preservedInHome = $true }
    elseif (-not $interactive -or $isAgent) { Add-Item $false 'keys' $keysDir 'private keys are removed only through the human-only confirmation (non-interactive run)'; $preservedInHome = $true }
    else { $removeKeys = $true; Add-Item $true 'keys' $keysDir 'private keys; requires the human-only confirmation' }
  }
}

Write-Output 'jureuma uninstall plan'
foreach ($label in @('REMOVE', 'PRESERVE')) {
  Write-Output $label
  $items = @($plan | Where-Object { $_.Remove -eq ($label -eq 'REMOVE') })
  if ($items.Count -eq 0) { Write-Output '  (nothing)' }
  foreach ($i in $items) { Write-Output "  $($i.Path)  ($($i.Reason))" }
}
if (Test-Path -LiteralPath $keysDir) { Write-Output 'back up private keys first with: jureuma key export <keyId>' }
if ($DryRun) { Write-Output 'dry run: nothing was changed'; return }
if (-not $Yes) {
  if (-not $interactive) { throw 'jureuma uninstall: confirmation needed: run it in an interactive console or pass -Yes (nothing was changed)' }
  $answer = Read-Host 'Type "uninstall" to remove the items listed under REMOVE'
  if ($answer -ne 'uninstall') { throw 'jureuma uninstall: cancelled; nothing was changed' }
}

$jhomePrefix = [IO.Path]::GetFullPath($JHome).TrimEnd([char[]]@('\', '/')) + [IO.Path]::DirectorySeparatorChar
$allowed = @([IO.Path]::GetFullPath($Launcher), [IO.Path]::GetFullPath($JujuPath), [IO.Path]::GetFullPath($projCfg), [IO.Path]::GetFullPath($agents))
$failed = 0
foreach ($i in @($plan | Where-Object { $_.Remove -and $_.Kind -ne 'keys' })) {
  $full = [IO.Path]::GetFullPath($i.Path)
  $ok = $full.StartsWith($jhomePrefix, [StringComparison]::Ordinal) -or ($allowed -contains $full) -or ((Split-Path -Leaf $full) -eq $SkillId)
  if (-not $ok) { Write-Warning "refusing to remove unexpected path $full"; $failed++; continue }
  try {
    if ($i.Kind -eq 'block') {
      $text = [IO.File]::ReadAllText($full)
      $nl = if ($text.Contains("`r`n")) { "`r`n" } else { "`n" }
      $out = New-Object System.Collections.Generic.List[string]
      $skip = $false
      foreach ($line in ($text -split "`r?`n")) {
        if (-not $skip -and $line -eq $BeginMarker) { $skip = $true; continue }
        if ($skip -and $line -eq $EndMarker) { $skip = $false; continue }
        if (-not $skip) { $out.Add($line) }
      }
      [IO.File]::WriteAllText($full, ($out -join $nl), (New-Object System.Text.UTF8Encoding -ArgumentList $false))
    } else {
      Remove-Item -LiteralPath $full -Recurse -Force
    }
    Write-Output "removed $full"
  } catch { Write-Warning "could not remove $full"; $failed++ }
}
if ($removeKeys) {
  $word = 'keys-' + (Get-Random -Minimum 1000 -Maximum 99999)
  $answer = Read-Host "Type `"$word`" to permanently delete the private keys in $keysDir"
  if ($answer -eq $word) {
    try { Remove-Item -LiteralPath $keysDir -Recurse -Force; Write-Output "removed $keysDir" } catch { $failed++ }
  } else {
    Write-Output "preserved $keysDir (the human-only confirmation was not given)"
    $preservedInHome = $true
  }
}
if (-not $preservedInHome -and (Test-Path -LiteralPath $JHome) -and @(Get-ChildItem -LiteralPath $JHome -Force).Count -eq 0) {
  Remove-Item -LiteralPath $JHome -Force
  Write-Output "removed $JHome"
}
if ($failed -gt 0) { throw "jureuma uninstall: $failed item(s) could not be removed" }
Write-Output 'jureuma was uninstalled; ~/.guiho, ~/.guiho/bin, ~/.guiho/.temp and PATH were left untouched'
