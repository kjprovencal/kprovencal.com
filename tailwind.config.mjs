/** @type {import('tailwindcss').Config} */
export const darkMode = 'media';
export const content = [
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
    moss: "#606C38",
    pakistan: "#283618",
    cornsilk: "#FEFAE0",
    earth: "#DDA15E",
    tiger: "#BC6C25",
    ruddyBlue: "#77A7DB",
    coffee: "#6B4937",
  },
  corePlugins: {
    preflight: false,
  },
};
export const plugins = [
  require('flowbite/plugin')
];
