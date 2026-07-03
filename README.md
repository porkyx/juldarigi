# 줄다리기 - 갤창랭킹 수집기

Wails + React TypeScript 기반의 standalone 데스크톱 앱입니다.

## 요구사항

- Go
- Node.js
- Wails CLI

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 개발 실행

```powershell
wails dev
```

## 빌드

```powershell
wails build
```

빌드된 앱은 별도 Node.js 서버를 실행하지 않고 Go 백엔드와 embed된 React 프론트엔드만으로 동작합니다.
