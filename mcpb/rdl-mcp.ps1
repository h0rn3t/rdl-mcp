$ErrorActionPreference = "Stop"

$architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
switch ($architecture) {
    "Arm64" { $binary = "rdl-mcp_windows_arm64.exe" }
    "X64" { $binary = "rdl-mcp_windows_amd64.exe" }
    default { Write-Error "RDL MCP не підтримує архітектуру Windows $architecture"; exit 1 }
}

$path = Join-Path $PSScriptRoot $binary
if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
    Write-Error "Бінарник RDL MCP не знайдено: $path"
    exit 1
}

& $path @args
exit $LASTEXITCODE
