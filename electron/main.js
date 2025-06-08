import { app, BrowserWindow } from 'electron';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const isDev = process.env.NODE_ENV === 'development'

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
    // 윈도우 생성
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

