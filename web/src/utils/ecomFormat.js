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

  const lines = []

  // 【商品品类】
  const category = analysis.product_type || analysis.product_name_zh || analysis.product_name || ''
  lines.push('【商品品类】' + category)
  lines.push('')

  // 【核心卖点】
  lines.push('【核心卖点】')
  analysis.selling_points.forEach(item => {
    const emoji = ICON_MAP[item.icon] || '📍'
    const desc = item.zh_desc ? '：' + item.zh_desc : ''
    lines.push(`${emoji} ${item.zh}${desc}`)
  })
  lines.push('')

  // 【补充描述】
  const audience = analysis.target_audience || ''
  const scenes = analysis.target_scenes_zh?.length
    ? analysis.target_scenes_zh
    : (analysis.target_scenes || [])
  let supplement = '【补充描述】' + audience
  if (scenes.length) {
    supplement += (audience ? '，' : '') + '适合' + scenes.join('、')
  }
  lines.push(supplement)

  return lines.join('\n').trim()
}

export function getStyleDesc(recommendedStyle) {
  return STYLE_DESC_MAP[recommendedStyle] || ''
}
