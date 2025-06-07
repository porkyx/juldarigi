// POST 요청 핸들러 (함수형 프로그래밍 원칙)

import { parseGalleryUrl, buildPageUrl } from './urlUtils.js';
import { 
  createRetryableScraper, 
  extractPostsFromPage, 
  extractPostsWithDateRange,
  aggregateUserPosts,
  formatScrapeResult 
} from './scraperCore.js';

// 날짜 범위 스크래핑 함수
const scrapeDateRange = async (browser, galleryInfo, startDate, endDate) => {
  const scraper = createRetryableScraper(browser);
  let currentPage = 1;
  let shouldContinue = true;
  let totalPosts = 0;
  const userPostCount = {};
  
  console.log(`Starting date range scraping from ${startDate || 'beginning'} to ${endDate || 'latest'}`);
  
  while (shouldContinue) {
    const pageUrl = buildPageUrl(galleryInfo.originalUrl, galleryInfo.galleryId, currentPage);
    console.log(`Scraping page ${currentPage} for date range: ${startDate || 'no start'} to ${endDate || 'no end'}`);
    
    const pageData = await scraper(pageUrl, extractPostsWithDateRange, startDate, endDate);
    
    if (pageData === null) {
      console.log(`Skipping page ${currentPage} due to repeated errors`);
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
    
    console.log(`Page ${currentPage} results: ${pageData.posts.length} posts found, foundOlderDate: ${pageData.foundOlderDate}`);
    
    // 중단 조건 확인
    if (startDate && pageData.foundOlderDate && pageData.posts.length === 0) {
      console.log('Stopping: Found older posts and no posts in range on this page');
      shouldContinue = false;
    } else if (!startDate && pageData.posts.length === 0) {
      console.log('Stopping: No start date specified and no posts found');
      shouldContinue = false;
    } else {
      currentPage++;
    }
  }
  
  return { userPostCount, totalPosts, pagesScraped: currentPage - 1 };
};

// 페이지 기반 스크래핑 함수 (단순)
const scrapeSimplePages = async (browser, galleryInfo, pages) => {
  const scraper = createRetryableScraper(browser);
  let totalPosts = 0;
  const userPostCount = {};
  
  for (let pageNum = 1; pageNum <= pages; pageNum++) {
    const pageUrl = buildPageUrl(galleryInfo.originalUrl, galleryInfo.galleryId, pageNum);
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
  }
  
  return { userPostCount, totalPosts, pagesScraped: pages };
};

// 페이지 기반 스크래핑 함수 (배치)
const scrapeBatchPages = async (browser, galleryInfo, pages) => {
  const scraper = createRetryableScraper(browser);
  const BATCH_SIZE = 20;
  let totalPosts = 0;
  const userPostCount = {};
  
  for (let batchStart = 1; batchStart <= pages; batchStart += BATCH_SIZE) {
    const batchEnd = Math.min(batchStart + BATCH_SIZE - 1, pages);
    const batchPromises = [];
    
    for (let pageNum = batchStart; pageNum <= batchEnd; pageNum++) {
      const pagePromise = async () => {
        const pageUrl = buildPageUrl(galleryInfo.originalUrl, galleryInfo.galleryId, pageNum);
        return await scraper(pageUrl, extractPostsFromPage);
      };
      
      batchPromises.push(pagePromise());
    }
    
    const batchResults = await Promise.all(batchPromises);
    
    batchResults.forEach(pageData => {
      const newUserCounts = aggregateUserPosts(pageData);
      Object.entries(newUserCounts).forEach(([uid, userData]) => {
        if (!userPostCount[uid]) {
          userPostCount[uid] = userData;
        } else {
          userPostCount[uid].count += userData.count;
        }
      });
      
      totalPosts += pageData.length;
    });
    
    const batchNumber = Math.ceil(batchStart / BATCH_SIZE);
    const totalBatches = Math.ceil(pages / BATCH_SIZE);
    console.log(`Completed batch ${batchNumber} of ${totalBatches} (pages ${batchStart}-${batchEnd})`);
    
    if (batchNumber % 5 === 0) {
      console.log(`Progress: ${Math.round((batchNumber / totalBatches) * 100)}%`);
    }
  }
  
  return { userPostCount, totalPosts, pagesScraped: pages };
};

// 일반 스크래핑 함수
const scrapeGeneralContent = async (browser, url, selector) => {
  let page;
  try {
    page = await browser.newPage();
    await page.goto(url, { waitUntil: 'networkidle2' });
    
    const content = await page.evaluate((sel) => {
      const element = document.querySelector(sel);
      if (!element) return null;
      
      return {
        text: element.innerText || element.textContent,
        html: element.innerHTML,
        tagName: element.tagName
      };
    }, selector);
    
    return {
      success: true,
      type: 'general',
      url,
      selector,
      content
    };
  } finally {
    if (page) await page.close();
  }
};

// POST 스크래핑 메인 핸들러
export const handlePOSTScraping = (browser) => {
  return async (req, res) => {
    // 타임아웃 비활성화
    req.setTimeout(0);
    res.setTimeout(0);
    
    const { url, selector = 'body', type = 'general', pages = 1, startDate, endDate } = req.body;
    
    if (!url) {
      return res.status(400).json({ error: 'URL is required' });
    }
    
    const galleryInfo = parseGalleryUrl(url);
    
    if (galleryInfo.isValid || type === 'dcgallery') {
      if (!galleryInfo.isValid) {
        return res.status(400).json({ 
          error: 'Invalid URL format. Expected DCInside gallery URL' 
        });
      }
      
      try {
        let result;
        
        if (startDate || endDate) {
          // 날짜 범위 스크래핑
          result = await scrapeDateRange(browser, galleryInfo, startDate, endDate);
          console.log(`Date range scraping completed. Total pages scraped: ${result.pagesScraped}, Total posts found: ${result.totalPosts}`);
        } else {
          // 페이지 기반 스크래핑
          if (pages <= 3) {
            result = await scrapeSimplePages(browser, galleryInfo, pages);
          } else {
            result = await scrapeBatchPages(browser, galleryInfo, pages);
          }
        }
        
        const formattedResult = formatScrapeResult(
          galleryInfo,
          result.userPostCount,
          result.totalPosts,
          result.pagesScraped,
          startDate,
          endDate
        );
        
        res.json(formattedResult);
        
      } catch (error) {
        res.status(500).json({ 
          success: false, 
          error: error.message 
        });
      }
    } else {
      // 일반 스크래핑
      try {
        const result = await scrapeGeneralContent(browser, url, selector);
        res.json(result);
      } catch (error) {
        res.status(500).json({ 
          success: false, 
          error: error.message 
        });
      }
    }
  };
};