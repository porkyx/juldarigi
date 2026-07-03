$ErrorActionPreference = "Stop"

function Invoke-Checked {
  param(
    [Parameter(Mandatory = $true)]
    [string]$FilePath,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgumentList
  )

  & $FilePath @ArgumentList
  if ($LASTEXITCODE -ne 0) {
    throw "Command failed with exit code $LASTEXITCODE`: $FilePath $($ArgumentList -join ' ')"
  }
}

function Add-KnownGccToPath {
  if (Get-Command gcc -ErrorAction SilentlyContinue) {
    return
  }

  $gccDirs = @(
    "C:\msys64\ucrt64\bin",
    "C:\msys64\mingw64\bin",
    "C:\msys64\clang64\bin",
    "C:\Strawberry\c\bin"
  )

  foreach ($gccDir in $gccDirs) {
    if (Test-Path (Join-Path $gccDir "gcc.exe")) {
      $env:Path = "$gccDir;$env:Path"
      return
    }
  }
}

Add-KnownGccToPath

$gofmtFiles = @(gofmt -l .)
if ($LASTEXITCODE -ne 0) {
  throw "Command failed with exit code $LASTEXITCODE`: gofmt -l ."
}
if ($gofmtFiles.Count -gt 0) {
  Write-Error ("Go files need gofmt:`n" + ($gofmtFiles -join "`n"))
}

Invoke-Checked go vet ./...
Invoke-Checked go test ./...

Push-Location frontend
try {
  Invoke-Checked pnpm test
  Invoke-Checked pnpm run build
}
finally {
  Pop-Location
}
