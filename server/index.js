// 메인 서버 파일 (함수형 프로그래밍 원칙)

import express from 'express';
import cors from 'cors';
import { initBrowser, closeBrowser } from './browserManager.js';
import { handleSSEScraping } from './sseHandler.js';
import { handlePOSTScraping } from './postHandler.js';


const app = express();
const PORT = 4321;

// 미들웨어 설정
app.use(cors());
app.use(express.json());

// 전역 타임아웃 비활성화
app.timeout = 0;

// 브라우저 인스턴스
let browser;

// 서버 설정을 위한 순수 함수
const configureServer = (server) => {
  server.timeout = 0;
  server.keepAliveTimeout = 0;
  server.headersTimeout = 0;
  return server;
};

// 애플리케이션 시작 함수
const startApplication = async () => {
  try {
    // 브라우저 초기화
    browser = await initBrowser();
    console.log('Browser initialized successfully');

    // 라우트 설정
    app.get('/scrape-stream', handleSSEScraping(browser));
    app.post('/scrape', handlePOSTScraping(browser));

    // 서버 시작
    const server = app.listen(PORT, () => {
      console.log(`Puppeteer server running on http://localhost:${PORT}`);
    });

    // 서버 설정 적용
    configureServer(server);

    return server;
  } catch (error) {
    console.error('Failed to start application:', error);
    process.exit(1);
  }
};

// 애플리케이션 종료 처리
const handleShutdown = async () => {
  console.log('Shutting down gracefully...');
  await closeBrowser(browser);
  process.exit(0);
};

// 시그널 핸들러 등록
process.on('SIGINT', handleShutdown);
process.on('SIGTERM', handleShutdown);

// 애플리케이션 시작
startApplication().catch(console.error);