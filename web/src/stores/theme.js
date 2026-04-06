import { create } from 'zustand'

export const useThemeStore = create((set) => ({
    // 'light' or 'dark'
    theme: localStorage.getItem('serverdash-theme') || localStorage.getItem('webcasa-theme') || 'light',

    setTheme: (theme) => {
        localStorage.setItem('serverdash-theme', theme)
        localStorage.removeItem('webcasa-theme')
        document.documentElement.className = theme === 'dark' ? 'dark-theme' : 'light-theme'
        set({ theme })
    },

    toggle: () => {
        set((state) => {
            const next = state.theme === 'dark' ? 'light' : 'dark'
            localStorage.setItem('serverdash-theme', next)
            localStorage.removeItem('webcasa-theme')
            document.documentElement.className = next === 'dark' ? 'dark-theme' : 'light-theme'
            return { theme: next }
        })
    },

    // Call on app init to sync class
    init: () => {
        const saved = localStorage.getItem('serverdash-theme') || localStorage.getItem('webcasa-theme') || 'light'
        if (saved) localStorage.setItem('serverdash-theme', saved)
        document.documentElement.className = saved === 'dark' ? 'dark-theme' : 'light-theme'
        set({ theme: saved })
    },
}))
