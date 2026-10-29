param(
 [Parameter(Mandatory=$true)][string]$Repository,
 [Parameter(Mandatory=$true)][string]$Apk,
 [string]$Tag = 'v1.0.0'
)
$ErrorActionPreference = 'Stop'
if (!(Test-Path -LiteralPath $Apk) -or [IO.Path]::GetExtension($Apk) -ne '.apk') { throw '请提供实际生成的 APK 文件' }
Get-Command gh -ErrorAction Stop | Out-Null
gh release create $Tag $Apk --repo $Repository --verify-tag --title "拾光自习 $Tag" --notes-file "$PSScriptRoot\..\docs\release-notes.md"
if ($LASTEXITCODE -ne 0) { throw '发布失败，请检查 gh 登录和 tag 推送状态' }
