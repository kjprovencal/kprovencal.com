import { ResumeSections } from "@/utils/constants";
import Experience from "./experience";
import Skills from "./skills";
import { EducationEntry, SkillsEntry, WorkEntry } from "@/lib/record";
import { FaDownload } from "react-icons/fa";
import Heading from "@/components/layout/heading";
import { Section } from "@/utils/types";
import fetchRelative from "@/utils/fetch-absolute";
import { Suspense } from "react";
import Loading from "@/components/loading";

function ResumeSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="w-[96%] max-w-5xl h-fit mx-auto mt-6 pt-6 pb-6 border-b border-gray-200 before:table after:table after:clear-both">
      <div className="pt-2 w-1/4 relative px-5 py-0 min-h-[1px] float-left">
        <h1 className="uppercase tracking-widest text-lg font-bold">
          <span className="border-b-2 border-tiger pb-1">
            {title}
          </span>
        </h1>
      </div>
      <div className="pr-[10%] float-left w-3/4 relative pl-5 py-0 min-h-[1px] px-5 max-w-none mx-[-20px] my-0 table">
        <div className="w-auto relative px-5 py-0 min-h-[1px] float-left max-w-none mx-[-20px] my-0 table">
          {children}
        </div>
      </div>
    </div>
  )
}

async function Content() {
  // If workData[i].end is null, then it is present. Move to the front of the array.
  const { educationData, workData, skillsData } = await fetchResumeData();
  workData.sort((a, b) => {
    if (!a.end) return -1;
    if (!b.end) return 1;
    return 0;
  });
  return <>
    <ResumeSection title={ResumeSections.Education}>
      {educationData.map((e: any) => <Experience key={e.school} data={e} />)}
    </ResumeSection>
    <ResumeSection title={ResumeSections.Work}>
      {workData.map((w: any) => <Experience key={w.company} data={w} />)}
    </ResumeSection>
    <ResumeSection title={ResumeSections.Skills}>
      <Skills data={skillsData} />
    </ResumeSection>
  </>
}

async function fetchResumeData(): Promise<{ educationData: EducationEntry[]; workData: WorkEntry[]; skillsData: SkillsEntry[]; }> {
  try {
    const data = await Promise.all([
      fetchRelative('/api/resume/education?sort=-graduated').then(res => res.json()),
      fetchRelative('/api/resume/work?sort=-end').then(res => res.json()),
      fetchRelative('/api/resume/skill?sort=-level').then(res => res.json()),
    ]);
    return {
      educationData: data[0] ?? [],
      workData: data[1] ?? [],
      skillsData: data[2] ?? [],
    };
  } catch (error) {
    console.error('Error fetching resume data', error);
    return {
      educationData: [],
      workData: [],
      skillsData: [],
    };
  }
}

export default async function Resume() {
  return (
    <main className="mt-20">
      <Suspense fallback={<Loading />}>
        <Content />
      </Suspense>
      <div className="w-auto max-w-none my-0 pt-9 min-h-[1px] flex justify-center">
        <p className="leading-8 text-secondary mb-8">
          <a href={'/static/resume.pdf'} className="mt-2 mb-4 px-5 py-3 text-cornsilk bg-pakistan font-open_sans font-bold text-base/8 inline-flex items-center rounded"><FaDownload className="mr-4 text-xl" />Download Resume</a>
        </p>
      </div>
    </main>
  );
}
