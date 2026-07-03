// Theme handling. Dark is the default; the choice is persisted to localStorage and applied as a
// data-theme attribute on the document root, which the stylesheet keys off.
export type Theme = 'dark' | 'light'

const storageKey = 'pigeonpost-theme'

export function loadTheme(): Theme {
    return localStorage.getItem(storageKey) === 'light' ? 'light' : 'dark'
}

export function applyTheme(theme: Theme): void {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem(storageKey, theme)
}
