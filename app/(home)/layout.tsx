"use client"
import Footer from "../../components/layout/footer";
import Header from "../../components/layout/header";
import SectionWrapper from "../../components/layout/wrapper";
import SectionProvider from "./section-provider";
import { Section } from "../../components/sections";

export default function HomeLayout({ children }: { children: React.ReactNode }) {
  

  return (
    <SectionProvider>
      <SectionWrapper section={Section.Home}>
        <Header home />
      </SectionWrapper>
      {children}
      <Footer home />
    </SectionProvider>
  )
}