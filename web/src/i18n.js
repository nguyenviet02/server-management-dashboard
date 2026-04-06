import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'

import en from './locales/en.json'
import vi from './locales/vi.json'

const storedLang = localStorage.getItem('serverdash-lang') || localStorage.getItem('webcasa-lang')
if (storedLang && !localStorage.getItem('serverdash-lang')) {
    localStorage.setItem('serverdash-lang', storedLang)
}

i18n
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
        resources: {
            en: { translation: en },
            vi: { translation: vi },
        },
        fallbackLng: 'en',
        interpolation: {
            escapeValue: false, // React already escapes
        },
        detection: {
            order: ['localStorage', 'navigator'],
            lookupLocalStorage: 'serverdash-lang',
            caches: ['localStorage'],
        },
    })

export default i18n
