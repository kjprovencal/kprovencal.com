"use client"
import { useContext, useEffect } from 'react';
import { useInView } from 'react-intersection-observer';
import { Section } from "../../app/(home)/sections";
import { SectionContext } from '../../app/(home)/section-provider';
import { SectionState } from "../../app/(home)/sections";

export default function SectionWrapper({ section, children }: { section: Section, children: React.ReactNode }) {
  const { sectionState, setSectionState } = useContext(SectionContext);
  const { ref, inView } = useInView({
    initialInView: true,
  });

  useEffect(() => {
    const newSectionState = new SectionState();
    newSectionState.clone(sectionState);
    newSectionState.setSection(section, inView);
    setSectionState(newSectionState);
  }, [inView, section, sectionState, setSectionState]);

  return (
    <section id={section} ref={ref}
      className={(section === Section.About ? 'bg-altBackground py-24 ' : 'bg-background ') + 'overflow-hidden'} >
      {children}
    </section>
  )
}