$ErrorActionPreference = "Stop"

go test ./...

Push-Location frontend
try {
  pnpm test
  pnpm run build
}
finally {
  Pop-Location
}
