/** @type {import('tailwindcss').Config} */
export const darkMode = 'media';
export const content = [
  './node_modules/flowbite-react/lib/esm/**/*.js',
  './pages/**/*.{js,ts,jsx,tsx,mdx}',
  './components/**/*.{js,ts,jsx,tsx,mdx}',
  './app/**/*.{js,ts,jsx,tsx,mdx}'
];
export const theme = {
  extend: {
    backgroundImage: {
      'header': 'url("/images/header-background.jpg")',
    },
    fontFamily: {
      open_sans: ['var(--font-open-sans)'],
      libre_baskerville: ['var(--font-libre-baskerville)']
    },
  },
  colors: {
    foreground: 'rgb(var(--foreground-rgb) / <alpha-value>)',
    background: 'rgb(var(--background-rgb) / <alpha-value>)',
    primary: 'rgb(var(--color-primary) / <alpha-value>)',
    secondary: 'rgb(var(--color-secondary) / <alpha-value>)',
    altBackground: 'rgb(var(--color-alt-background) / <alpha-value>)',
    accent: 'rgb(var(--color-accent) / <alpha-value>)',
  },
  corePlugins: {
    preflight: false,
  },
};
export const plugins = [
  require('flowbite/plugin')
];
