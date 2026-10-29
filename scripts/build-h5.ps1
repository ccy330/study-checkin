param([string]$HBuilderX = 'D:\HB\HBuilderX')
$ErrorActionPreference = 'Stop'
$project = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\page'))
$compiler = Join-Path $HBuilderX 'plugins\uniapp-cli-vite'
if (!(Test-Path -LiteralPath $compiler)) { throw '请通过 -HBuilderX 参数指定安装目录，并先安装 uni-app Vue3 编译插件' }
$env:HX_APP_ROOT = $HBuilderX
$env:UNI_INPUT_DIR = $project
$env:UNI_OUTPUT_DIR = Join-Path $project 'unpackage\dist\build\h5'
Push-Location $compiler
try {
 node (Join-Path $compiler 'node_modules\@dcloudio\vite-plugin-uni\bin\uni.js') build -p h5 --config (Join-Path $compiler 'vite.config.js')
 if ($LASTEXITCODE -ne 0) { throw 'H5 编译失败' }
} finally { Pop-Location }
