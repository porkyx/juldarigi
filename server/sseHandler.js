// Server-Sent Events 핸들러 (함수형 프로그래밍 원칙)

import { parseGalleryUrl, buildPageUrl } from './urlUtils.js';
import { 
  createRetryableScraper, 
  extractPostsFromPage, 
  extractPostsWithDateRange,
  aggregateUserPosts,
  formatScrapeResult 
} from './scraperCore.js';

// SSE 응답 헤더 설정하는 순수 함수
export const setupSSEHeaders = (res) => {
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');
  res.setHeader('Access-Control-Allow-Origin', '*');
  return res;
};

// SSE 이벤트 전송하는 순수 함수
export const createEventSender = (res) => {
  return (event, data) => {
    res.write(`event: ${event}\n`);
    res.write(`data: ${JSON.stringify(data)}\n\n`);
  };
};

// 날짜 범위 스크래핑을 위한 고차함수
export const createDateRangeScraper = (browser, galleryInfo, sendEvent) => {
  const scraper = createRetryableScraper(browser);
  
  return async (startDate, endDate) => {
    let currentPage = 1;
    let shouldContinue = true;
    let totalPosts = 0;
    const userPostCount = {};
    
    sendEvent('info', { 
      message: `Starting date range scraping from ${startDate || 'beginning'} to ${endDate || 'latest'}` 
    });
    
    while (shouldContinue) {
      const pageUrl = buildPageUrl(galleryInfo.originalUrl, galleryInfo.galleryId, currentPage);
      
      sendEvent('progress', { 
        currentPage, 
        totalPosts,
        uniqueUsers: Object.keys(userPostCount).length,
        message: `Scraping page ${currentPage}...` 
      });
      
      const pageData = await scraper(pageUrl, extractPostsWithDateRange, startDate, endDate);
      
      if (pageData === null) {
        sendEvent('warning', { message: `Skipping page ${currentPage} due to errors` });
        currentPage++;
        continue;
      }
      
      // 사용자별 게시물 수 집계
      const newUserCounts = aggregateUserPosts(pageData.posts);
      Object.entries(newUserCounts).forEach(([uid, userData]) => {
        if (!userPostCount[uid]) {
          userPostCount[uid] = userData;
        } else {
          userPostCount[uid].count += userData.count;
        }
      });
      
      totalPosts += pageData.posts.length;
      
      sendEvent('pageComplete', { 
        page: currentPage,
        postsFound: pageData.posts.length,
        totalPosts,
        uniqueUsers: Object.keys(userPostCount).length
      });
      
      // 중단 조건 확인
      if (startDate && pageData.foundOlderDate && pageData.posts.length === 0) {
        sendEvent('info', { message: 'Found posts older than target date, stopping...' });
        shouldContinue = false;
      } else if (!startDate && pageData.posts.length === 0) {
        sendEvent('info', { message: 'No more posts found, stopping...' });
        shouldContinue = false;
      } else {
        currentPage++;
      }
    }
    
    return { userPostCount, totalPosts, pagesScraped: currentPage - 1 };
  };
};

// 페이지 기반 스크래핑을 위한 고차함수
export const createPageBasedScraper = (browser, galleryInfo, sendEvent) => {
  const scraper = createRetryableScraper(browser);
  
  return async (numPages) => {
    let totalPosts = 0;
    const userPostCount = {};
    
    sendEvent('info', { message: `Starting page-based scraping for ${numPages} pages` });
    
    for (let pageNum = 1; pageNum <= numPages; pageNum++) {
      const pageUrl = buildPageUrl(galleryInfo.originalUrl, galleryInfo.galleryId, pageNum);
      
      sendEvent('progress', { 
        currentPage: pageNum,
        totalPages: numPages,
        totalPosts,
        uniqueUsers: Object.keys(userPostCount).length,
        message: `Scraping page ${pageNum} of ${numPages}...` 
      });
      
      const pageData = await scraper(pageUrl, extractPostsFromPage);
      
      // 사용자별 게시물 수 집계
      const newUserCounts = aggregateUserPosts(pageData);
      Object.entries(newUserCounts).forEach(([uid, userData]) => {
        if (!userPostCount[uid]) {
          userPostCount[uid] = userData;
        } else {
          userPostCount[uid].count += userData.count;
        }
      });
      
      totalPosts += pageData.length;
      
      sendEvent('pageComplete', { 
        page: pageNum,
        postsFound: pageData.length,
        totalPosts,
        uniqueUsers: Object.keys(userPostCount).length
      });
    }
    
    return { userPostCount, totalPosts, pagesScraped: numPages };
  };
};

// SSE 스크래핑 메인 핸들러
export const handleSSEScraping = (browser) => {
  return async (req, res) => {
    setupSSEHeaders(res);
    
    const { url, pages = '1', startDate, endDate } = req.query;
    const numPages = parseInt(pages);
    const sendEvent = createEventSender(res);
    
    // 클라이언트 연결 해제 처리
    req.on('close', () => {
      console.log('Client disconnected from SSE');
      res.end();
    });
    
    if (!url) {
      sendEvent('error', { message: 'URL is required' });
      res.end();
      return;
    }
    
    const galleryInfo = parseGalleryUrl(url);
    
    if (!galleryInfo.isValid) {
      sendEvent('error', { message: 'Invalid URL format. Expected DCInside gallery URL' });
      res.end();
      return;
    }
    
    try {
      sendEvent('start', { 
        galleryId: galleryInfo.galleryId, 
        galleryType: galleryInfo.galleryType,
        url: galleryInfo.originalUrl, 
        startDate, 
        endDate 
      });
      
      let result;
      
      if (startDate || endDate) {
        const dateRangeScraper = createDateRangeScraper(browser, galleryInfo, sendEvent);
        result = await dateRangeScraper(startDate, endDate);
      } else {
        const pageBasedScraper = createPageBasedScraper(browser, galleryInfo, sendEvent);
        result = await pageBasedScraper(numPages);
      }
      
      const formattedResult = formatScrapeResult(
        galleryInfo,
        result.userPostCount,
        result.totalPosts,
        result.pagesScraped,
        startDate,
        endDate
      );
      
      sendEvent('complete', formattedResult);
      
    } catch (error) {
      sendEvent('error', { message: error.message });
    } finally {
      res.end();
    }
  };
};