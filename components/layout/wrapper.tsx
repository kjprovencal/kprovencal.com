"use client"
import { useContext, useEffect } from 'react';
import { useInView } from 'react-intersection-observer';
import { Section } from "@/app/(home)/sections";
import { SectionContext } from '@/app/(home)/section-provider';
import { SectionState } from "@/app/(home)/sections";

export default function SectionWrapper({ section, children }: { section: Section, children: React.ReactNode }) {
  const { setSectionState } = useContext(SectionContext);
  const { ref, inView } = useInView({
    initialInView: true,
  });

  useEffect(() => {
    if (inView) {
      setSectionState(new SectionState(section));
    }
  }, [inView, section, setSectionState]);

  return (
    <section id={section} ref={ref}
      className={(section === Section.About ? 'bg-altBackground py-24 ' : 'bg-background ') + 'overflow-hidden'} >
      {children}
    </section>
  )
}