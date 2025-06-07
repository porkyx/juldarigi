// URL 관련 유틸리티 함수들 (순수 함수)

// DCInside 갤러리 URL 패턴 정의
const URL_PATTERNS = {
  MINI_BOARD: /gall\.dcinside\.com\/mini\/board\/lists\/?\?id=([^&]+)/,
  MINI_SHORT: /gall\.dcinside\.com\/mini\/([^\/?]+)/,
  MGALLERY_BOARD: /gall\.dcinside\.com\/mgallery\/board\/lists\/?\?id=([^&]+)/,
  MGALLERY_SHORT: /gall\.dcinside\.com\/mgallery\/([^\/?]+)/,
  BOARD_SHORT: /gall\.dcinside\.com\/([^\/?]+)$/
};

// URL에서 갤러리 ID 추출하는 순수 함수
export const extractGalleryId = (url) => {
  if (!url || typeof url !== 'string') return null;
  
  // 패턴 순서대로 매칭 시도
  for (const pattern of Object.values(URL_PATTERNS)) {
    const match = url.match(pattern);
    if (match && match[1]) {
      return match[1];
    }
  }
  
  return null;
};

// URL이 유효한 DC갤러리 URL인지 확인하는 순수 함수
export const isValidDCGalleryUrl = (url) => {
  return url && url.includes('gall.dcinside.com') && extractGalleryId(url) !== null;
};

// 갤러리 타입 확인하는 순수 함수
export const getGalleryType = (url) => {
  if (!url) return null;
  
  if (URL_PATTERNS.MGALLERY_BOARD.test(url) || URL_PATTERNS.MGALLERY_SHORT.test(url)) {
    return 'mgallery';
  }
  
  if (URL_PATTERNS.MINI_BOARD.test(url) || URL_PATTERNS.MINI_SHORT.test(url)) {
    return 'mini';
  }
  
  if (URL_PATTERNS.BOARD_SHORT.test(url)) {
    return 'board'; // 일반 갤러리
  }
  
  return null;
};

// 페이지 URL 생성하는 순수 함수
export const buildPageUrl = (url, galleryId, pageNumber) => {
  const galleryType = getGalleryType(url);
  
  switch (galleryType) {
    case 'mgallery':
      return `https://gall.dcinside.com/mgallery/board/lists/?id=${galleryId}&page=${pageNumber}`;
    case 'mini':
      return `https://gall.dcinside.com/mini/board/lists/?id=${galleryId}&page=${pageNumber}`;
    case 'board':
      return `https://gall.dcinside.com/board/lists/?id=${galleryId}&page=${pageNumber}`;
    default:
      // 기본값으로 mini 갤러리 형식 사용
      return `https://gall.dcinside.com/mini/board/lists/?id=${galleryId}&page=${pageNumber}`;
  }
};

// URL 정보를 파싱하는 순합성 함수
export const parseGalleryUrl = (url) => {
  const galleryId = extractGalleryId(url);
  const galleryType = getGalleryType(url);
  const isValid = isValidDCGalleryUrl(url);
  
  return {
    galleryId,
    galleryType,
    isValid,
    originalUrl: url
  };
};