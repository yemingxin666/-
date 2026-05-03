const ICON_MAP = {
  fabric: '🧶',
  fit: '👗',
  design: '✨',
  quality: '💎',
  comfort: '☁️',
  function: '⚙️',
  scene: '🎭',
}

const STYLE_DESC_MAP = {
  default_shoot:     '标准电商商拍，干净明亮，重点突出商品',
  lifestyle_mag:     '自然光，有氛围感和生活质感',
  minimal_cold:      '极简留白，高反差，奢侈品质感',
  energetic_hit:     '高饱和度，大字冲击，活力感强',
  dark_quality:      '深色系，电影质感，戏剧性打光',
  asymmetric_layout: '非对称布局，左侧大图突出主体，右侧细节图',
}

export function formatAnalysisToText(analysis, fallbackContent = '') {
  if (!analysis?.selling_points?.length) return fallbackContent
  return analysis.selling_points.map(item => {
    const emoji = ICON_MAP[item.icon] || '📍'
    return `${emoji} ${item.zh}${item.zh_desc ? '\n   ' + item.zh_desc : ''}`
  }).join('\n')
}

export function getStyleDesc(recommendedStyle) {
  return STYLE_DESC_MAP[recommendedStyle] || ''
}
