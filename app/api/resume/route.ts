import { EDUCATION, SKILL, WORK, ALBUM } from '../../../utils/constants';
import { monthYear } from '../../../utils/date-format';
import { AlbumType, EducationEntry, SkillsEntry, WorkEntry } from '../../../lib/resumeEntry';
import { type NextRequest } from 'next/server';
import { ResumeEntry } from '../../../lib/resumeEntry';

function buildEntry(r: any, slug: string) {
  let e;
  switch (slug) {
    case EDUCATION:
      r.degree ??= '';
      r.description ??= '';
      r.graduated = r.graduated ? monthYear(new Date(r.graduated)) : 'N/A';
      r.school ??= '';
      e = r as unknown as EducationEntry;
      break;
    case SKILL:
      r.name ??= '';
      r.level ??= 0;
      e = r as unknown as SkillsEntry;
      break;
    case WORK:
      const years = `${monthYear(new Date(r.start))} - ${r.end ? monthYear(new Date(r.end)) : 'Present'}`;
      r.company ??= '';
      r.description ??= '';
      r.title ??= '';
      r.years = years;
      e = r as unknown as WorkEntry;
      break;
    case ALBUM:
      r.id ??= '';
      r.title ??= '';
      r.url ??= '';
      r.thumbnail = `${process.env.PB_URL}/api/files/${r.collectionId}/${r.id}/${r.thumbnail}`;
      e = r as unknown as AlbumType;
      break;
    default:
      throw new Error('Invalid collection');
  }
  return e as ResumeEntry;
}
  

export async function GET(request: NextRequest, { params }: { params: { slug: string } }) {
  const { slug } = params;
  const sort = request.nextUrl.searchParams.get('sort') ?? '-created';
  const res = await fetch(`${process.env.PB_URL}/api/collections/${slug}?sort=${sort}`);
  const results = await res.json();
  if (!results.items) { throw results.message };

  const entries: ResumeEntry[] = [];
  
  results.items.forEach((r: any) => {
    entries.push(buildEntry(r, slug));
  });
  
  return Response.json(entries);
}