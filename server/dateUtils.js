// 날짜 관련 유틸리티 함수들 (순수 함수)

// 날짜 형식을 정규화하는 순수 함수
export const normalizeDate = (dateStr) => {
  if (!dateStr || typeof dateStr !== 'string') return '';
  
  const trimmed = dateStr.trim();
  
  // 이미 YYYY-MM-DD 형식인 경우
  if (/^\d{4}-\d{2}-\d{2}/.test(trimmed)) {
    return trimmed.split(' ')[0];
  }
  
  // 시간만 표시된 경우 (HH:MM) - 오늘 날짜로 가정
  if (/^\d{2}:\d{2}$/.test(trimmed)) {
    const today = new Date();
    const year = today.getFullYear();
    const month = String(today.getMonth() + 1).padStart(2, '0');
    const day = String(today.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }
  
  // MM.DD 형식인 경우 - 현재 연도 추가
  if (/^\d{2}\.\d{2}$/.test(trimmed)) {
    const today = new Date();
    const year = today.getFullYear();
    const [month, day] = trimmed.split('.');
    return `${year}-${month}-${day}`;
  }
  
  return '';
};

// 날짜가 범위 내에 있는지 확인하는 순수 함수
export const isDateInRange = (dateStr, startDate, endDate) => {
  const normalizedDate = normalizeDate(dateStr);
  if (!normalizedDate) return false;
  
  let isInRange = true;
  
  if (startDate && normalizedDate < startDate) {
    isInRange = false;
  }
  
  if (endDate && normalizedDate > endDate) {
    isInRange = false;
  }
  
  return isInRange;
};

// 날짜가 시작일보다 오래된지 확인하는 순수 함수
export const isDateOlder = (dateStr, startDate) => {
  if (!startDate) return false;
  
  const normalizedDate = normalizeDate(dateStr);
  if (!normalizedDate) return false;
  
  return normalizedDate < startDate;
};

// 오늘 날짜를 YYYY-MM-DD 형식으로 반환하는 순수 함수
export const getTodayString = () => {
  const today = new Date();
  const year = today.getFullYear();
  const month = String(today.getMonth() + 1).padStart(2, '0');
  const day = String(today.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
};