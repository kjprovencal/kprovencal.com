import About from './sections/about'
import Resume from './sections/resume'
import Gallery from './sections/gallery'
import { Suspense } from 'react'
import Contact from './sections/contact'
import SectionWrapper from '@/components/layout/wrapper'
import { Section } from "./sections"
import fetchAbsolute from '@/utils/fetch-absolute'

async function fetchResumeData() {
  try {
    const data = await Promise.all([
      fetchAbsolute('/api/resume/education?sort=-graduated').then(res => res.json()),
      fetchAbsolute('/api/resume/work?sort=-end').then(res => res.json()),
      fetchAbsolute('/api/resume/skill?sort=-level').then(res => res.json()),
      fetchAbsolute('/api/resume/album').then(res => res.json()),
    ]);
    return {
      educationData: data[0] ?? [],
      workData: data[1] ?? [],
      skillsData: data[2] ?? [],
      albumsData: data[3] ?? []
    };
  } catch (error) {
    console.error('Error fetching resume data', error);
    return {
      educationData: [],
      workData: [],
      skillsData: [],
      albumsData: []
    };
  }
}

export default async function Home({ }) {
  const { educationData, workData, skillsData, albumsData } = await fetchResumeData();
  return (
    <main>
      <SectionWrapper section={Section.About}>
        <About />
      </SectionWrapper>
      <Suspense>
        <SectionWrapper section={Section.Resume}>
          <Resume data={{ educationData, workData, skillsData }} />
        </SectionWrapper>
      </Suspense>
      <SectionWrapper section={Section.Gallery}>
        <Suspense>
          <Gallery data={albumsData} />
        </Suspense>
      </SectionWrapper>
      <SectionWrapper section={Section.Contact}>
        <Contact />
      </SectionWrapper>
    </main>
  );
}
