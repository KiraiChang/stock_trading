/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,js,ts}'],
  theme: {
    extend: {
      colors: {
        surface: '#0f0f1a',
        panel: '#1a1a2e',
        border: '#2a2a4a',
        muted: '#6b6b8a',
        rise: '#e74c3c',   // 台灣股市：漲紅
        fall: '#2ecc71',   // 台灣股市：跌綠
        flat: '#95a5a6',
      },
    },
  },
  plugins: [],
}
