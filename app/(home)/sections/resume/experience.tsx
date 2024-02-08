import { EducationEntry, WorkEntry } from '@/lib/resume-entry';

export default function Experience({data}:{data: EducationEntry | WorkEntry }){
  const location = (data as EducationEntry).school || (data as WorkEntry).company;
  const details = (data as EducationEntry).degree || (data as WorkEntry).title;
  const time = (data as EducationEntry).graduated || (data as WorkEntry).years;

  return (
    <div key={location}>
      <h3 className="text-2xl font-bold text-accent dark:text-foreground">{location}</h3>
      <p className="text-base dark:text-secondary font-libre_baskerville italic mb-5 mt-2 flex flex-grow">{details}
        <span className="mx-1">&bull;</span>
        <em className="text-base not-italic">{time}</em>
      </p>
      <p className="mb-8 dark:text-secondary">{data.description}</p>
    </div>
  )
}