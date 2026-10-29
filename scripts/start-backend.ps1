$ErrorActionPreference = 'Stop'
$backend = Join-Path $PSScriptRoot '..\bankend'
$config = Join-Path $backend '.env'
if (!(Test-Path -LiteralPath $config)) { throw '请先在 bankend/.env 中配置 MYSQL_DSN=连接串' }
foreach ($line in Get-Content -LiteralPath $config) {
 if ($line -match '^([A-Z_]+)=(.*)$') { [Environment]::SetEnvironmentVariable($Matches[1],$Matches[2],'Process') }
}
Push-Location $backend
try {
 if (Test-Path -LiteralPath '.\server.exe') { & .\server.exe } else { go run . }
} finally { Pop-Location }
