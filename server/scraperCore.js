// 스크래핑 핵심 로직 (함수형 프로그래밍 원칙)

import { normalizeDate, isDateInRange, isDateOlder } from './dateUtils.js';

// 브라우저 페이지 설정을 위한 순수 함수
export const configurePageForScraping = async (page) => {
  await page.setRequestInterception(true);
  
  page.on('request', (req) => {
    const resourceType = req.resourceType();
    if (resourceType === 'stylesheet' || resourceType === 'image' || resourceType === 'font') {
      req.abort();
    } else {
      req.continue();
    }
  });
  
  return page;
};

// 페이지에서 게시물 데이터를 추출하는 순수 함수 (브라우저 컨텍스트에서 실행)
export const extractPostsFromPage = () => {
  const rows = document.querySelectorAll('tbody.listwrap2 tr');
  const posts = [];

  rows.forEach(row => {
    // 공지사항 행 스킵
    if (row.classList.contains('ub-notice')) return;
    
    // 공지사항 아이콘이 있는 게시물 스킵
    const noticeIcon = row.querySelector('em.icon_img.icon_notice');
    if (noticeIcon) return;
  
    const writerElement = row.querySelector('td.gall_writer');
    if (!writerElement) return;
  
    const dataUid = writerElement.getAttribute('data-uid');
    const nickname = writerElement.querySelector('.nickname')?.textContent?.trim() || 
                   writerElement.querySelector('.nick_comm')?.textContent?.trim() || 
                   'Unknown';
    const ip = writerElement.querySelector('.ip')?.textContent?.trim() || '';
    
    if (dataUid) {
      posts.push({
        uid: dataUid,
        nickname: nickname,
        ip: ip
      });
    }
  });
  
  return posts;
};

// 날짜 범위를 고려하여 게시물 데이터를 추출하는 순수 함수 (브라우저 컨텍스트에서 실행)
export const extractPostsWithDateRange = (targetStartDate, targetEndDate) => {
  const rows = document.querySelectorAll('tbody.listwrap2 tr');
  const posts = [];
  let foundOlderDate = false;
  
  // 날짜 정규화 함수 (브라우저 컨텍스트용)
  const normalizeDate = (dateStr) => {
    if (!dateStr) return '';
    
    // 이미 YYYY-MM-DD 형식인 경우
    if (/^\d{4}-\d{2}-\d{2}/.test(dateStr)) {
      return dateStr.split(' ')[0];
    }
    
    // 시간만 표시된 경우 (HH:MM)
    if (/^\d{2}:\d{2}$/.test(dateStr)) {
      const today = new Date();
      const year = today.getFullYear();
      const month = String(today.getMonth() + 1).padStart(2, '0');
      const day = String(today.getDate()).padStart(2, '0');
      return `${year}-${month}-${day}`;
    }
    
    // MM.DD 형식인 경우
    if (/^\d{2}\.\d{2}$/.test(dateStr)) {
      const today = new Date();
      const year = today.getFullYear();
      const [month, day] = dateStr.split('.');
      return `${year}-${month}-${day}`;
    }
    
    return '';
  };
  
  rows.forEach(row => {
    if (row.classList.contains('ub-notice')) return;
    
    const noticeIcon = row.querySelector('em.icon_img.icon_notice');
    if (noticeIcon) return;
    
    const dateElement = row.querySelector('td.gall_date');
    const writerElement = row.querySelector('td.gall_writer');
    
    if (!dateElement || !writerElement) return;
    
    // title 속성에서 먼저 날짜를 가져오고, 없으면 textContent 사용
    const dateTitle = dateElement.getAttribute('title');
    const dateText = dateElement.textContent.trim();
    const postDate = dateTitle || dateText;
    const postDateOnly = normalizeDate(postDate);
    
    // 유효한 날짜가 있을 때만 범위 체크
    let isInRange = true;
    if (postDateOnly) {
      if (targetStartDate && postDateOnly < targetStartDate) {
        isInRange = false;
        foundOlderDate = true;
      }
      if (targetEndDate && postDateOnly > targetEndDate) {
        isInRange = false;
      }
    } else {
      isInRange = false;
    }
    
    if (isInRange && postDateOnly) {
      const dataUid = writerElement.getAttribute('data-uid');
      const nickname = writerElement.querySelector('.nickname')?.textContent?.trim() || 
                     writerElement.querySelector('.nick_comm')?.textContent?.trim() || 
                     'Unknown';
      const ip = writerElement.querySelector('.ip')?.textContent?.trim() || '';
      
      if (dataUid) {
        posts.push({
          uid: dataUid,
          nickname: nickname,
          ip: ip,
          date: postDate
        });
      }
    }
  });
  
  return { posts, foundOlderDate };
};

// 재시도 로직을 포함한 페이지 스크래핑 함수 (고차함수)
export const createRetryableScraper = (browser, maxRetries = 5) => {
  return async (pageUrl, extractorFunction, ...extractorArgs) => {
    let retryCount = 0;
    
    while (retryCount < maxRetries) {
      let page;
      try {
        page = await browser.newPage();
        await configurePageForScraping(page);
        
        const response = await page.goto(pageUrl, { 
          waitUntil: 'networkidle2', 
          timeout: 60000 
        });
        
        // 서버 에러 체크
        if (response && [503, 500, 504].includes(response.status())) {
          throw new Error(`Server error: ${response.status()}`);
        }
        
        await page.waitForSelector('tbody.listwrap2', { timeout: 10000 }).catch(() => {});
        
        // 동적으로 전달받은 추출 함수 실행
        const pageData = await page.evaluate(extractorFunction, ...extractorArgs);
        
        return pageData;
        
      } catch (error) {
        retryCount++;
        console.log(`Error scraping ${pageUrl} (attempt ${retryCount}/${maxRetries}): ${error.message}`);
        
        if (retryCount >= maxRetries) {
          console.log(`Skipping ${pageUrl} after ${maxRetries} failed attempts`);
          return extractorFunction === extractPostsWithDateRange ? null : [];
        }
        
        // 지수 백오프
        await new Promise(resolve => 
          setTimeout(resolve, Math.min(1000 * Math.pow(2, retryCount - 1), 10000))
        );
        
      } finally {
        if (page) await page.close();
      }
    }
    
    return extractorFunction === extractPostsWithDateRange ? null : [];
  };
};

// 사용자 게시물 수를 집계하는 순수 함수
export const aggregateUserPosts = (posts) => {
  return posts.reduce((acc, post) => {
    const key = post.uid;
    if (!acc[key]) {
      acc[key] = {
        uid: post.uid,
        nickname: post.nickname,
        ip: post.ip,
        count: 0
      };
    }
    acc[key].count++;
    return acc;
  }, {});
};

// 사용자 통계를 정렬하는 순수 함수
export const sortUserStats = (userPostCount) => {
  return Object.values(userPostCount).sort((a, b) => b.count - a.count);
};

// 스크래핑 결과를 포맷하는 순수 함수
export const formatScrapeResult = (galleryInfo, userPostCount, totalPosts, pagesScraped, startDate, endDate) => {
  const sortedUsers = sortUserStats(userPostCount);
  
  return {
    success: true,
    type: 'dcgallery',
    galleryId: galleryInfo.galleryId,
    galleryType: galleryInfo.galleryType,
    url: galleryInfo.originalUrl,
    pagesScraped,
    startDate: startDate || null,
    endDate: endDate || null,
    totalPosts,
    uniqueUsers: sortedUsers.length,
    userStats: sortedUsers
  };
};