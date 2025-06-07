import { app, BrowserWindow } from 'electron';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import express from 'express';
import cors from 'cors';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const isDev = process.env.NODE_ENV === 'development'

// 내장 서버 설정
let server;
let browser;
const SERVER_PORT = 4321;

// 서버 시작 함수
async function startServer() {
  try {
    console.log('Starting embedded server...');

    // 동적 import를 사용하여 서버 모듈들을 로드
    let browserManager, sseHandler, postHandler;

    if (isDev) {
      // 개발 모드
      browserManager = await import('../server/browserManager.js');
      sseHandler = await import('../server/sseHandler.js');
      postHandler = await import('../server/postHandler.js');
    } else {
      // 프로덕션 모드 - app.asar.unpacked에서 로드
      const serverPath = path.join(process.resourcesPath, 'app.asar.unpacked', 'server');
      browserManager = await import(path.join(serverPath, 'browserManager.js'));
      sseHandler = await import(path.join(serverPath, 'sseHandler.js'));
      postHandler = await import(path.join(serverPath, 'postHandler.js'));
    }

    const expressApp = express();

    // 미들웨어 설정
    expressApp.use(cors());
    expressApp.use(express.json());
    expressApp.timeout = 0;

    // 브라우저 초기화
    browser = await browserManager.initBrowser();
    console.log('Browser initialized successfully');

    // 라우트 설정
    expressApp.get('/scrape-stream', sseHandler.handleSSEScraping(browser));
    expressApp.post('/scrape', postHandler.handlePOSTScraping(browser));

    // 서버 시작
    server = expressApp.listen(SERVER_PORT, () => {
      console.log(`Embedded server running on http://localhost:${SERVER_PORT}`);
    });

    // 서버 설정
    server.timeout = 0;
    server.keepAliveTimeout = 0;
    server.headersTimeout = 0;

    return server;
  } catch (error) {
    console.error('Failed to start embedded server:', error);
    throw error;
  }
}

// 서버 정리 함수
async function stopServer() {
  try {
    if (browser) {
      const browserManager = isDev
        ? await import('../server/browserManager.js')
        : await import(path.join(process.resourcesPath, 'app.asar.unpacked', 'server', 'browserManager.js'));

      await browserManager.closeBrowser(browser);
      browser = null;
    }

    if (server) {
      server.close();
      server = null;
    }

    console.log('Server stopped successfully');
  } catch (error) {
    console.error('Error stopping server:', error);
  }
}

async function createWindow() {
  const mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false,
      enableRemoteModule: false,
      webSecurity: false,
      allowRunningInsecureContent: true
    }
  })

  // 전역 변수로 mainWindow 저장
  global.mainWindow = mainWindow;

  if (isDev) {
    await mainWindow.loadURL('http://localhost:3000')
      .catch(console.error)
  } else {
    // 프로덕션 빌드에서는 리소스 폴더의 dist에서 파일들을 로드
    const indexPath = path.join(process.resourcesPath, 'dist', 'index.html');
    await mainWindow.loadFile(indexPath)
      .catch(console.error)
  }

  // Content Security Policy 설정
  mainWindow.webContents.session.webRequest.onHeadersReceived((details, callback) => {
    callback({
      responseHeaders: {
        ...details.responseHeaders,
        'Content-Security-Policy': ['default-src \'self\' \'unsafe-inline\' \'unsafe-eval\' data: blob: file: http: https:']
      }
    });
  });

  // 개발 모드에서만 개발자 도구 열기
  if (isDev) {
    mainWindow.webContents.openDevTools();
  }
}

app.whenReady().then(async () => {
  try {
    // 서버 먼저 시작
    await startServer();

    // 서버가 시작된 후 윈도우 생성
    await createWindow();
  } catch (error) {
    console.error('Failed to start application:', error);
    app.quit();
  }
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('activate', async () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    await createWindow().catch(console.error)
  }
})

// 앱 종료 시 서버 정리
app.on('before-quit', async (event) => {
  event.preventDefault();
  await stopServer();
  app.exit();
})