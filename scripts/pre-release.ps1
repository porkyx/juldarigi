$ErrorActionPreference = "Stop"

go test -coverprofile C:\tmp\juldarigi-cover.out ./...
go tool cover -func C:\tmp\juldarigi-cover.out
go test -run=^$ -fuzz=FuzzParseGalleryURL -fuzztime=10s ./...
go test -run=^$ -fuzz=FuzzNormalizeDateWithNow -fuzztime=10s ./...
go test -race ./...

Push-Location frontend
try {
  npm test
  npm run build
}
finally {
  Pop-Location
}

go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 build
