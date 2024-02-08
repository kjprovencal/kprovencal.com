import { SkillsEntry } from "@/lib/resume-entry";
import resumeData from '@/lib/resume-data.json';

function SkillRow({ data }: { data: SkillsEntry }) {

  return (
    <li key={data.name} className="relative mb-[60px] bg-[#ccc] h-10 rounded-sm">
      <em className="font-open_sans font-bold uppercase not-italic text-accent dark:text-foreground text-sm tracking-widest relative -top-9">{data.name}</em>
      <span
        className={"absolute left-0 top-0 m-0 pr-6 bg-altBackground inline-block h-10 leading-10 rounded-l-sm"}
        style={{ width: `${data.level}%`}}
      />
    </li>
  );
}

export default function Skills({ data }: { data: SkillsEntry[]}) {
  return (
  <>
    <p className="mb-8 mx-0 mt-0">{resumeData.resume.skillmessage}</p>
    <div className="w-[95%] float-left p-0 text-left">
      <ul className="mt-9 list-none mb-6">
        {data.map(s => <SkillRow key={s.name} data={s} />)}
      </ul>
    </div>
  </>
  );
}