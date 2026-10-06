/**
 * 图标集：单文件 16×16 线性图标，stroke currentColor。
 *
 * 不用 Iconify/图标字体的原因：产物 go:embed 进二进制，必须离线自包含，
 * 且图标只有几十个 —— 一份 path 表比任何依赖都小且可控。
 *
 * 约定：所有图标按 16 网格绘制，只描边不填充（除 fill 显式标记），
 * 线宽由使用处的 CSS 控制，保证与文字基线对齐。
 */
export const ICONS = {
  /* 导航 */
  chat: '<path d="M2.5 4.2A1.7 1.7 0 0 1 4.2 2.5h7.6a1.7 1.7 0 0 1 1.7 1.7v5a1.7 1.7 0 0 1-1.7 1.7H7l-3.4 2.2a.5.5 0 0 1-.8-.4V9.9A1.7 1.7 0 0 1 2.5 8.2z"/>',
  files: '<path d="M2.5 4.5A1.5 1.5 0 0 1 4 3h2.4l1.3 1.6H12A1.5 1.5 0 0 1 13.5 6v5.5A1.5 1.5 0 0 1 12 13H4a1.5 1.5 0 0 1-1.5-1.5z"/>',
  settings: '<circle cx="8" cy="8" r="2.25"/><path d="M8 1.75v1.6M8 12.65v1.6M14.25 8h-1.6M3.35 8h-1.6M12.4 3.6l-1.13 1.13M4.73 11.27L3.6 12.4M12.4 12.4l-1.13-1.13M4.73 4.73L3.6 3.6"/>',
  info: '<circle cx="8" cy="8" r="6.25"/><path d="M8 7.4v3.4M8 5.2v.1"/>',

  /* 操作 */
  send: '<path d="M8 13V3.6M8 3.6 4.2 7.4M8 3.6l3.8 3.8"/>',
  plus: '<path d="M8 3.5v9M3.5 8h9"/>',
  close: '<path d="M4 4l8 8M12 4l-8 8"/>',
  menu: '<path d="M2 4.5h12M2 8h12M2 11.5h8"/>',
  panel: '<rect x="2" y="3" width="12" height="10" rx="2"/><path d="M10 3v10"/>',
  search: '<circle cx="7" cy="7" r="4.25"/><path d="M10.2 10.2 13.5 13.5"/>',
  trash: '<path d="M3.5 4.5h9M6.5 4.5V3.2h3v1.3M5 4.5l.5 8h5l.5-8"/>',
  pencil: '<path d="M11.2 2.9 13 4.7 5.6 12l-2.7.7.7-2.7z"/>',
  pin: '<path d="M6 2h4l-.5 4 2 2.5H4.5l2-2.5z"/><path d="M8 8.5V14"/>',
  download: '<path d="M8 2.5v7.5M5 7.5 8 10.5l3-3M3 13h10"/>',
  upload: '<path d="M8 10.5V3M5 6 8 3l3 3M3 13h10"/>',
  refresh: '<path d="M13 8a5 5 0 1 1-1.5-3.6"/><path d="M13.2 2.4v2.6h-2.6"/>',
  copy: '<rect x="5.5" y="5.5" width="8" height="8" rx="1.6"/><path d="M10.5 5.5V4A1.5 1.5 0 0 0 9 2.5H4A1.5 1.5 0 0 0 2.5 4v5A1.5 1.5 0 0 0 4 10.5h1.5"/>',
  check: '<path d="M3.5 8.5l3 3 6-7"/>',
  stop: '<rect x="4.5" y="4.5" width="7" height="7" rx="1.4"/>',
  branch: '<circle cx="4.5" cy="4" r="1.6"/><circle cx="4.5" cy="12" r="1.6"/><circle cx="11.5" cy="6" r="1.6"/><path d="M4.5 5.6v4.8M6.1 4.6c3 0 3.9.5 3.9 1.4"/>',
  folder: '<path d="M2.5 4.5A1.5 1.5 0 0 1 4 3h2.4l1.3 1.6H12A1.5 1.5 0 0 1 13.5 6v5.5A1.5 1.5 0 0 1 12 13H4a1.5 1.5 0 0 1-1.5-1.5z"/>',
  file: '<path d="M9 2.5H4.5A1.5 1.5 0 0 0 3 4v8a1.5 1.5 0 0 0 1.5 1.5h7A1.5 1.5 0 0 0 13 12V6.5z"/><path d="M9 2.5V6.5h4"/>',
  chevronDown: '<path d="M4 6l4 4 4-4"/>',
  chevronRight: '<path d="M6 4l4 4-4 4"/>',
  chevronLeft: '<path d="M10 4L6 8l4 4"/>',
  arrowUp: '<path d="M8 13V3.6M8 3.6 4.2 7.4M8 3.6l3.8 3.8"/>',
  dots: '<circle cx="4" cy="8" r=".9" fill="currentColor" stroke="none"/><circle cx="8" cy="8" r=".9" fill="currentColor" stroke="none"/><circle cx="12" cy="8" r=".9" fill="currentColor" stroke="none"/>',
  sun: '<circle cx="8" cy="8" r="3.1"/><path d="M8 1.5v1.4M8 13.1v1.4M14.5 8h-1.4M2.9 8H1.5M12.6 3.4 11.6 4.4M4.4 11.6l-1 1M12.6 12.6l-1-1M4.4 4.4l-1-1"/>',
  moon: '<path d="M13 9.6A5.4 5.4 0 0 1 6.4 3 5.5 5.5 0 1 0 13 9.6z"/>',
  brain: '<path d="M6 3.2A2 2 0 0 0 4 5.1a1.8 1.8 0 0 0-.6 3 2 2 0 0 0 .9 3.3A1.9 1.9 0 0 0 8 12.9V4.9A1.8 1.8 0 0 0 6 3.2z"/><path d="M10 3.2A2 2 0 0 1 12 5.1a1.8 1.8 0 0 1 .6 3 2 2 0 0 1-.9 3.3A1.9 1.9 0 0 1 8 12.9"/>',
  terminal: '<rect x="2" y="3" width="12" height="10" rx="2"/><path d="M5 7l2 1.8L5 10.6M8.6 11h2.4"/>',
  zap: '<path d="M9 2 4 9h3.4L7 14l5-7H8.6z"/>',
  shield: '<path d="M8 2 3.5 3.6v4c0 3 1.9 5 4.5 6 2.6-1 4.5-3 4.5-6v-4z"/><path d="M6.2 8 7.5 9.3l2.6-2.7"/>',
  alert: '<path d="M6.9 2.3 1.6 12a1.2 1.2 0 0 0 1.05 1.8h10.7A1.2 1.2 0 0 0 14.4 12L9.1 2.3a1.2 1.2 0 0 0-2.2 0z"/><path d="M8 5.9v3.4M8 11.4v.1"/>',
  error: '<circle cx="8" cy="8" r="6.25"/><path d="M8 5v4M8 11.3v.1"/>',
  ok: '<circle cx="8" cy="8" r="6.25"/><path d="M5.2 8.2 7 10l3.8-4.2"/>',
  clock: '<circle cx="8" cy="8" r="6.25"/><path d="M8 4.6V8l2.4 1.6"/>',
  cpu: '<rect x="4.5" y="4.5" width="7" height="7" rx="1.5"/><rect x="2" y="2" width="12" height="12" rx="2.5" stroke-dasharray="2 2"/>',
  logout: '<path d="M6.5 3H4a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h2.5"/><path d="M10 5.5 12.8 8 10 10.5M6.5 8h6.3"/>',
  link: '<path d="M6.8 9.2a2.6 2.6 0 0 0 3.7 0l1.9-1.9a2.6 2.6 0 0 0-3.7-3.7L7.6 4.8"/><path d="M9.2 6.8a2.6 2.6 0 0 0-3.7 0L3.6 8.7a2.6 2.6 0 0 0 3.7 3.7l1-1"/>',
  eye: '<path d="M1.6 8S4 4 8 4s6.4 4 6.4 4-2.4 4-6.4 4-6.4-4-6.4-4z"/><circle cx="8" cy="8" r="1.9"/>',
  eyeOff: '<path d="M6.4 4.3A5.9 5.9 0 0 1 8 4c4 0 6.4 4 6.4 4a11 11 0 0 1-1.7 2.2M4.2 5.5A11.4 11.4 0 0 0 1.6 8S4 12 8 12a6 6 0 0 0 2-.34"/><path d="M2.5 2.5l11 11"/>',
  spinner: '<path d="M8 1.75a6.25 6.25 0 1 0 6.25 6.25"/>',
} as const

export type IconName = keyof typeof ICONS
