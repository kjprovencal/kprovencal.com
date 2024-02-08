import About from '../../components/sections/about'
import Resume from '../../components/sections/resume'
import Gallery from '../../components/sections/gallery'
import { Suspense } from 'react'
import Contact from '../../components/sections/contact'
import SectionWrapper from '../../components/layout/wrapper'
import { Section } from "../../components/sections"
import { EducationEntry, WorkEntry, SkillsEntry, AlbumType } from '@/lib/resumeEntry'

async function fetchResumeData() {
  const apiUrl = process.env.PB_URL ?? 'localhost:8090';
  const data = await Promise.all([
    fetch(apiUrl + '/api/collections/education/records?sort=-graduated').then(res => res.json()),
    fetch(apiUrl + '/api/collections/work/records?sort=-end').then(res => res.json()),
    fetch(apiUrl + '/api/collections/skill/records?sort=-level').then(res => res.json()),
    fetch(apiUrl + '/api/collections/album/records').then(res => res.json())
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
