// Домены самой платформы (svistok.io) против доменов пространств (manibandha.ru и т.п.).
export function isPlatformHost() {
  return /(^|\.)svistok\.io$/i.test(window.location.hostname)
}
