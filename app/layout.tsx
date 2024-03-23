import { Metadata } from 'next';
import Script from 'next/script';
import { Libre_Baskerville, Open_Sans } from 'next/font/google';
import './globals.css';
import Navbar from '@/components/layout/navbar';
import Footer from '@/components/layout/footer';

const openSans = Open_Sans({
  subsets: ['latin'],
  style: ['italic', 'normal'],
  display: 'swap',
  variable: '--font-open-sans'
});

const libreBaskerville = Libre_Baskerville({
  weight: ['400', '700'],
  subsets: ['latin'],
  style: ['italic', 'normal'],
  display: 'swap',
  variable: '--font-libre-baskerville'
});

export const metadata: Metadata = {
  title: "Kyle's Personal Website",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className={`${openSans.variable} ${libreBaskerville.variable}`}>
      {/*
        <head /> will contain the components returned by the nearest parent head.tsx. Find out more at https://beta.nextjs.org/docs/api-reference/file-conventions/head
      */}
      <head />
      <body className='bg-cornsilk dark:bg-pakistan text-coffee dark:text-cornsilk h-screen'>
        <header>
          <Navbar />
        </header>
        {children}
        <Footer />
      </body>
    </html>
  )
}
