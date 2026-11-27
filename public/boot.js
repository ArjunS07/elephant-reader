;(function () {
  try {
    var raw = localStorage.getItem('elephant.prefs') || localStorage.getItem('field.prefs')
    if (!raw) return
    var p = JSON.parse(raw)
    var root = document.documentElement
    if (p.theme === 'light' || p.theme === 'warm' || p.theme === 'dark') {
      root.setAttribute('data-theme', p.theme)
    }
    if (p.fontFamily === 'inter' || p.fontFamily === 'lora' || p.fontFamily === 'newsreader') {
      root.setAttribute('data-font', p.fontFamily)
    }
    var size = Number(p.fontSize)
    if (size >= 15 && size <= 20) root.style.setProperty('--reading-size', size + 'px')
    var leading = Number(p.lineHeight)
    if (leading >= 1.5 && leading <= 1.85) root.style.setProperty('--reading-leading', String(leading))
    var cols = { cozy: '600px', standard: '672px', wide: '760px', narrow: '600px', default: '672px' }
    if (cols[p.columnWidth]) root.style.setProperty('--column', cols[p.columnWidth])
    if (p.align === 'left' || p.align === 'center') root.setAttribute('data-align', p.align)
  } catch (e) {}
})()
