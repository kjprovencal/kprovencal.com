import { ResumeSections } from "@/utils/constants";
import Experience from "./experience";
import Skills from "./skills";
import { EducationEntry, SkillsEntry, WorkEntry } from "@/lib/resume-entry";

function ResumeSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="w-[96%] max-w-5xl h-fit mx-auto mt-6 pt-6 pb-6 border-b border-gray-200 before:table after:table after:clear-both">
      <div className="pt-2 w-1/4 relative px-5 py-0 min-h-[1px] float-left">
        <h1 className="uppercase tracking-widest text-lg font-bold text-accent dark:text-foreground">
          <span className="border-b-2 border-orange-400 pb-1">
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

export default function Resume({ data }: { data: { educationData: EducationEntry[], workData: WorkEntry[], skillsData: SkillsEntry[] } }) {
  const { educationData, workData, skillsData } = data;
  return (
    <>
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
  );
}
