// 브라우저 관리 모듈 (함수형 프로그래밍 원칙)

import puppeteer from 'puppeteer';

// 브라우저 설정을 위한 순수 함수
export const createBrowserConfig = () => ({
  headless: 'new',
  args: ['--no-sandbox', '--disable-setuid-sandbox']
});

// 브라우저 초기화 함수
export const initBrowser = async () => {
  const config = createBrowserConfig();
  return await puppeteer.launch(config);
};

// 브라우저 종료 함수
export const closeBrowser = async (browser) => {
  if (browser) {
    await browser.close();
  }
};