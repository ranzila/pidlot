# pidlot cross-platform build.
# Builds every target of the matrix into .\build\<platform>\ (created here).
#
# Usage:
#   .\build.ps1                  # build the full matrix
#   .\build.ps1 darwin           # only one platform
#   .\build.ps1 darwin/arm64     # only one platform/arch
param(
    [Parameter(Position = 0)]
    [string]$Target = ""
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$Go = if ($env:GO) { $env:GO } else { "go" }
$LdFlags = "-s -w"

# platform folder | GOOS | GOARCH
$Matrix = @(
    @{ Folder = "win";    Os = "windows"; Arch = "amd64" }
    @{ Folder = "win";    Os = "windows"; Arch = "arm64" }
    @{ Folder = "darwin"; Os = "darwin";  Arch = "amd64" }
    @{ Folder = "darwin"; Os = "darwin";  Arch = "arm64" }
    @{ Folder = "linux";  Os = "linux";   Arch = "amd64" }
    @{ Folder = "linux";  Os = "linux";   Arch = "arm64" }
)

$built = 0
foreach ($t in $Matrix) {
    if ($Target -ne "") {
        $selPlatform = ($Target -split '/')[0]
        $selArch    = if (($Target -split '/').Count -gt 1) { ($Target -split '/')[1] } else { "" }
        $platformMatch = ($selPlatform -eq $t.Os) -or ($selPlatform -eq $t.Folder)
        $archMatch = ($selArch -eq "") -or ($selArch -eq $t.Arch)
        if (-not ($platformMatch -and $archMatch)) { continue }
    }

    $out = "build\$($t.Folder)\pidlot-$($t.Os)-$($t.Arch)"
    if ($t.Os -eq "windows") { $out += ".exe" }

    New-Item -ItemType Directory -Force -Path "build\$($t.Folder)" | Out-Null
    Write-Host "==> building $($t.Os)/$($t.Arch) -> $out"
    & $Go build -trimpath -ldflags $LdFlags -o $out .
    if ($LASTEXITCODE -ne 0) { throw "build failed for $($t.Os)/$($t.Arch)" }
    $built++
}

Write-Host ""
Write-Host "$built binary(ies) in .\build:"
Get-ChildItem -Recurse -File build | ForEach-Object { $_.FullName.Replace("$PSScriptRoot\", "") }
