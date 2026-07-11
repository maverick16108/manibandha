// Достать URL картинок из markdown (![](url)) и из готового HTML (<img src>).
export function extractImageUrls(text) {
  const urls = []
  const md = /!\[[^\]]*\]\(([^)\s]+)/g
  const html = /<img[^>]+src=["']([^"']+)["']/g
  let m
  while ((m = md.exec(text || ''))) urls.push(m[1])
  while ((m = html.exec(text || ''))) urls.push(m[1])
  return [...new Set(urls)]
}

// Дождаться загрузки картинок (с таймаутом), чтобы не было скачков вёрстки.
export function preloadImages(urls, timeout = 3000) {
  if (!urls || !urls.length) return Promise.resolve()
  return new Promise((resolve) => {
    let left = urls.length
    const done = () => { if (--left <= 0) resolve() }
    urls.forEach((u) => { const img = new Image(); img.onload = done; img.onerror = done; img.src = u })
    setTimeout(resolve, timeout)
  })
}
