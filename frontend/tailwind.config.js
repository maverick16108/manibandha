/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts}'],
  theme: {
    extend: {
      colors: {
        // Warm parchment / literary-spiritual palette
        parchment: {
          50: '#fdfbf7',
          100: '#faf6ee',
          200: '#f3ead8',
          300: '#e8d8ba',
        },
        ink: {
          700: '#3d342c',
          800: '#2b2420',
          900: '#1c1815',
        },
        saffron: {
          400: '#d98a3d',
          500: '#c8742a',
          600: '#a85e1f',
          700: '#8a4d1a',
        },
        sage: {
          300: '#a7b08f',
          400: '#8a9470',
          500: '#6f7a5a',
          600: '#5a6349',
        },
      },
      fontFamily: {
        display: ['"Cormorant Garamond"', 'Georgia', 'serif'],
        serif: ['"PT Serif"', 'Georgia', 'serif'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
        script: ['Caveat', 'cursive'],
      },
    },
  },
  plugins: [],
}
