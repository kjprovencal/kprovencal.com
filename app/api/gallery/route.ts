import { AlbumType } from "@/lib/record";
import { NextRequest } from "next/server";

function buildAlbum(record: any){
  record.id ??= '';
  record.title ??= '';
  record.url ??= '';
  record.thumbnail = `${process.env.PB_URL}/api/files/${record.collectionId}/${record.id}/${record.thumbnail}`;
  return record as AlbumType;
}

export async function GET(request: NextRequest) {
  const res = await fetch(`${process.env.PB_URL}/api/collections/album/records?sort=-created`, { headers: { 'Authorization': process.env.PB_API_KEY || '' } });
  const results = await res.json();

  const entries: AlbumType[] = [];

  results?.items?.forEach((r: any) => {
    entries.push(buildAlbum(r));
  });

  return Response.json(entries);
}