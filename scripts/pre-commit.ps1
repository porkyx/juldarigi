$ErrorActionPreference = "Stop"

go test ./...

Push-Location frontend
try {
  npm test
  npm run build
}
finally {
  Pop-Location
}
