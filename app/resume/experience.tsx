import { EducationEntry, WorkEntry } from '@/lib/record';

export default function Experience({data}:{data: EducationEntry | WorkEntry }){
  const location = (data as EducationEntry).school || (data as WorkEntry).company;
  const details = (data as EducationEntry).degree || (data as WorkEntry).title;
  const time = (data as EducationEntry).graduated || (data as WorkEntry).years;

  const getDescription = () => {
    try {
      const description = JSON.parse(data.description);
      return <ul className='mb-8 list-disc'>
        {description.map((point: string, i: number) => <li key={i}>{point}</li>)}
      </ul>
    } catch (e) {
      const description = data.description;
      return <p className='mb-8'>{description}</p>
    }
  }
  return (
    <div key={location}>
      <h3 className="text-2xl font-bold">{location}</h3>
      <p className="text-base font-libre_baskerville italic mb-5 mt-2 flex flex-grow">{details}
        <span className="mx-1">&bull;</span>
        <em className="text-base not-italic">{time}</em>
      </p>
      <>{getDescription()}</>
    </div>
  )
}