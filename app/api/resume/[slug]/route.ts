import { EDUCATION, SKILL, WORK } from '@/utils/constants';
import { monthYear } from '@/utils/date-format';
import { EducationEntry, SkillsEntry, WorkEntry, Record } from '@/lib/record';
import { type NextRequest } from 'next/server';

function buildEntry(r: any, slug: string) {
  let e;
  switch (slug) {
    case EDUCATION:
      r.degree ??= '';
      r.description ??= '';
      r.graduated = r.graduated ? monthYear(new Date(r.graduated)) : 'N/A';
      r.school ??= '';
      e = r as EducationEntry;
      break;
    case SKILL:
      r.name ??= '';
      r.level ??= 0;
      e = r as SkillsEntry;
      break;
    case WORK:
      const years = `${monthYear(new Date(r.start))} - ${r.end ? monthYear(new Date(r.end)) : 'Present'}`;
      r.company ??= '';
      r.description ??= '';
      r.title ??= '';
      r.years = years;
      e = r as WorkEntry;
      break;
    default:
      throw new Error('Invalid collection');
  }
  return e as Record;
}


export async function GET(request: NextRequest, { params }: { params: { slug: string } }) {
  const { slug } = params;
  const sort = request.nextUrl.searchParams.get('sort') ?? '-created';
  const res = await fetch(`${process.env.PB_URL}/api/collections/${slug}/records?sort=${sort}`, { headers: { 'Authorization': process.env.PB_API_KEY || '' } });
  const results = await res.json();
  if (!results.items) { throw results.message };

  const entries: Record[] = [];

  results.items.forEach((r: any) => {
    entries.push(buildEntry(r, slug));
  });

  return Response.json(entries);
}