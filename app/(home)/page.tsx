import About from './sections/about'
import Resume from './sections/resume'
import Gallery from './sections/gallery'
import { Suspense } from 'react'
import Contact from './sections/contact'
import SectionWrapper from '../../components/layout/wrapper'
import { Section } from "./sections"
import { EducationEntry, WorkEntry, SkillsEntry, AlbumType } from '@/lib/resume-entry'
import fetchAbsolute from '@/utils/fetch-absolute'

async function fetchResumeData() {
  try {
    const apiUrl = process.env.PB_URL ?? 'localhost:8090';
    const data = await Promise.all([
      fetchAbsolute('/api/resume/education').then(res => res.json()),
      fetchAbsolute('/api/collections/work').then(res => res.json()),
      fetchAbsolute('/api/collections/skill').then(res => res.json()),
      fetchAbsolute('/api/collections/album').then(res => res.json())
    ]);
    return {
      educationData: data[0]?.['items'] ?? [],
      workData: data[1]?.['items'] ?? [],
      skillsData: data[2]?.['items'] ?? [],
      albumsData: data[3]?.['items']?.map((alb: AlbumType) => {
        return {
          ...alb,
          thumbnail: `${apiUrl}/api/files/${alb.collectionId}/${alb.id}/${alb.thumbnail}`
        };
      }) ?? []
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

export default async function Home({}) {
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
