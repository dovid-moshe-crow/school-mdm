# Keep school-mdm + Cloudflare tunnel up for App Review / local tunnel hosting.
# Runs each process in its own visible console window so you can monitor logs.
# Restarts only when health check fails, process absent, or schoolmdm.exe was rebuilt.
$ErrorActionPreference = 'Continue'
$Host.UI.RawUI.WindowTitle = 'SchoolMDM Keep-Alive'
try { $Host.UI.RawUI.BackgroundColor = 'Black'; $Host.UI.RawUI.ForegroundColor = 'Green' } catch {}

$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not $RepoRoot) { $RepoRoot = 'C:\Users\dovid\Desktop\github\school-mdm' }
$Bin = Join-Path $RepoRoot 'bin\schoolmdm.exe'
$EnvFile = Join-Path $RepoRoot '.env'
$LogDir = Join-Path $RepoRoot 'bin\keepalive-logs'
$CloudflaredConfig = Join-Path $env:USERPROFILE '.cloudflared\config.yml'
$HealthUrl = 'http://127.0.0.1:8080/healthz'
$PollSeconds = 20
$RestartCooldownSeconds = 10

New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
$LogFile = Join-Path $LogDir ("keepalive-{0:yyyyMMdd}.log" -f (Get-Date))

function Write-Log([string]$Message) {
  $line = "{0:yyyy-MM-dd HH:mm:ss} {1}" -f (Get-Date), $Message
  Add-Content -Path $LogFile -Value $line -Encoding UTF8
  Write-Host $line
}

function Import-DotEnv([string]$Path) {
  if (-not (Test-Path $Path)) { return }
  Get-Content $Path | ForEach-Object {
    if ($_ -match '^\s*#' -or $_ -match '^\s*$') { return }
    if ($_ -match '^([^=]+)=(.*)$') {
      $k = $matches[1].Trim()
      $v = $matches[2].Trim()
      if (($v.StartsWith('"') -and $v.EndsWith('"')) -or ($v.StartsWith("'") -and $v.EndsWith("'"))) {
        $v = $v.Substring(1, $v.Length - 2)
      }
      Set-Item -Path "Env:$k" -Value $v
    }
  }
}

function Get-CloudflaredPath {
  try {
    $p = & mise which cloudflared 2>$null
    if ($p -and (Test-Path $p)) { return $p.Trim() }
  } catch {}
  $cmd = Get-Command cloudflared -ErrorAction SilentlyContinue
  $candidates = @(
    "$env:LOCALAPPDATA\mise\installs\cloudflared\2026.7.3\cloudflared.exe",
    $(if ($cmd) { $cmd.Source })
  )
  foreach ($c in $candidates) {
    if ($c -and (Test-Path $c)) { return $c }
  }
  return $null
}

function Test-Health {
  try {
    $r = Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing -TimeoutSec 5
    return ($r.StatusCode -eq 200 -and $r.Content -match '"ok"\s*:\s*true')
  } catch {
    return $false
  }
}

function Start-SchoolMdm {
  if (-not (Test-Path $Bin)) {
    Write-Log "ERROR: missing binary $Bin"
    return
  }
  Import-DotEnv $EnvFile
  Get-Process schoolmdm -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
  # Visible console: schoolmdm loads .env from repo cwd
  $cmd = "title SchoolMDM Server && `"$Bin`""
  Start-Process -FilePath 'cmd.exe' -ArgumentList @('/k', $cmd) -WorkingDirectory $RepoRoot -WindowStyle Normal
  Write-Log 'started schoolmdm (new window)'
}

function Start-Tunnel {
  $cf = Get-CloudflaredPath
  if (-not $cf) {
    Write-Log 'ERROR: cloudflared not found'
    return
  }
  if (-not (Test-Path $CloudflaredConfig)) {
    Write-Log "ERROR: missing $CloudflaredConfig"
    return
  }
  Get-Process cloudflared -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
  $cmd = "title SchoolMDM Cloudflare Tunnel && `"$cf`" tunnel --config `"$CloudflaredConfig`" run"
  Start-Process -FilePath 'cmd.exe' -ArgumentList @('/k', $cmd) -WorkingDirectory $RepoRoot -WindowStyle Normal
  Write-Log 'started cloudflared (new window)'
}

function Wait-WithKeys([int]$Seconds) {
  $deadline = (Get-Date).AddSeconds($Seconds)
  while ((Get-Date) -lt $deadline) {
    if ($Host.UI.RawUI.KeyAvailable) {
      $key = $Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')
      $ch = [string]$key.Character
      if ($ch -eq 'r' -or $ch -eq 'R') {
        Write-Log 'manual restart: schoolmdm (key r)'
        Start-SchoolMdm
        Start-Sleep -Seconds $RestartCooldownSeconds
        return 'restarted'
      }
      if ($ch -eq 't' -or $ch -eq 'T') {
        Write-Log 'manual restart: cloudflared (key t)'
        Start-Tunnel
        Start-Sleep -Seconds $RestartCooldownSeconds
        return 'restarted'
      }
      if ($ch -eq 'b' -or $ch -eq 'B') {
        Write-Log 'manual restart: both (key b)'
        Start-SchoolMdm
        Start-Sleep -Seconds 2
        Start-Tunnel
        Start-Sleep -Seconds $RestartCooldownSeconds
        return 'restarted'
      }
      if ($ch -eq '?' -or $ch -eq 'h' -or $ch -eq 'H') {
        Write-Host ''
        Write-Host 'Keys:  r = restart app   t = restart tunnel   b = restart both   h = help'
        Write-Host ''
      }
    }
    Start-Sleep -Milliseconds 200
  }
  return 'ok'
}

Write-Log "keep-alive starting (visible) repo=$RepoRoot"
Write-Host ''
Write-Host 'Keys:  r = restart app   t = restart tunnel   b = restart both   h = help'
Write-Host '(click this window first, then press a key)'
Write-Host ''
Import-DotEnv $EnvFile
$lastBinWrite = $null
if (Test-Path $Bin) { $lastBinWrite = (Get-Item $Bin).LastWriteTimeUtc }

# Always bring up visible windows on start (replace any hidden instances)
Write-Log 'launching schoolmdm + tunnel windows'
Start-SchoolMdm
Start-Sleep -Seconds 3
Start-Tunnel

while ($true) {
  try {
    if (Test-Path $Bin) {
      $w = (Get-Item $Bin).LastWriteTimeUtc
      if ($lastBinWrite -and $w -gt $lastBinWrite) {
        Write-Log 'binary changed; restarting schoolmdm'
        Start-SchoolMdm
        $lastBinWrite = $w
        Start-Sleep -Seconds $RestartCooldownSeconds
      } elseif (-not $lastBinWrite) {
        $lastBinWrite = $w
      }
    }

    if (-not (Test-Health)) {
      Write-Log 'healthz failed; restarting schoolmdm'
      Start-SchoolMdm
      Start-Sleep -Seconds $RestartCooldownSeconds
    }

    if (-not (Get-Process cloudflared -ErrorAction SilentlyContinue)) {
      Write-Log 'cloudflared missing; restarting tunnel'
      Start-Tunnel
      Start-Sleep -Seconds $RestartCooldownSeconds
    } else {
      Write-Host ("{0:HH:mm:ss} ok  healthz + tunnel  |  r=app  t=tunnel  b=both" -f (Get-Date))
    }
  } catch {
    Write-Log "loop error: $($_.Exception.Message)"
  }
  [void](Wait-WithKeys -Seconds $PollSeconds)
}
