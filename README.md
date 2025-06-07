# Juldarigi

Scala.js + Vite + Electron boilerplate with Laminar

## Tech Stack

- **Scala 3.7.1** with Scala.js
- **sbt 1.11.1** for build management
- **Laminar** for reactive UI
- **Vite** for fast development and building
- **Electron** for desktop application

## Development

1. Install dependencies:
```bash
npm install
```

2. Start development server:
```bash
npm run dev
```

3. Run Electron app in development:
```bash
npm run electron:dev
```

## Build

Build for production:
```bash
npm run build
```

Build Electron app:
```bash
npm run electron:build
```

## Scripts

- `npm run dev` - Start Scala.js compilation and Vite dev server
- `npm run build` - Build for production
- `npm run electron` - Run Electron app
- `npm run electron:dev` - Run Electron app with hot reload
- `npm run electron:build` - Build Electron app for distribution
- `npm run clean` - Clean build artifacts