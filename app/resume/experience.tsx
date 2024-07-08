import { EducationEntry, WorkEntry } from '@/lib/record';

const getDescription = (data: EducationEntry | WorkEntry) => {
  try {
    const description = JSON.parse(data.description);
    return <ul className='mb-8 p-5 sm:p-0 list-disc text-start'>
      {description.map((point: string, i: number) => <li key={i}>{point}</li>)}
    </ul>
  } catch (e) {
    const description = data.description;
    return <p className='mb-8'>{description}</p>
  }
}
export default function Experience({data}:{data: EducationEntry | WorkEntry }){
  const location = (data as EducationEntry).school || (data as WorkEntry).company;
  const details = (data as EducationEntry).degree || (data as WorkEntry).title;
  const time = (data as EducationEntry).graduated || (data as WorkEntry).years;

  return (
    <div key={`${location}_${time}`} className='text-center sm:text-left'>
      <h3 className="text-2xl font-bold ">{location}</h3>
      <p className="text-base font-libre_baskerville italic mb-5 mt-2 sm:flex sm:flex-grow">{details}
        <span className="mx-1">&bull;</span>
        <em className="text-base not-italic">{time}</em>
      </p>
      <>{getDescription(data)}</>
    </div>
  )
}