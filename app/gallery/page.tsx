import Link from "next/link";
import Image from 'next/image';
import { AlbumType } from "@/lib/record";
import { Suspense } from "react";
import fetchRelative from "@/utils/fetch-absolute";

function Album({ album }: { album: AlbumType }) {
  const albumLink = '/albums' + album.url;
  return (
    <div key={album.title} className="w-1/4 h-full float-left px-2 py-2 flex-row">
      <div className="relative rounded-md shadow-lg hover:shadow-xl transition duration-500 ease-in-out transform hover:-translate-y-1 hover:scale-105 h-full">
        <Link href={albumLink}>
          <Image alt={album.title} src={album.thumbnail} fill style={{ objectFit: 'contain' }} />
          <div className="relative hover:opacity-100 opacity-0 hover:bg-opacity-20 bg-opacity:0 bg-black w-full h-full">
            <h5 className="text-center text-white">{album.title}</h5>
          </div>
        </Link>
      </div>
    </div>
  );
}

async function Content() {
  const data = await fetchRelative('/api/gallery').then(res => res.json()).catch(err => {
    console.error(err);
    return [];
  });
  return <div className="w-full h-60">
    {data.map((album: AlbumType) => <Album key={album.title} album={album} />)}
  </div>
}
export default function Gallery() {
  return (
    <main className="h-full flex items-center">
      <div className="w-[96%] max-w-5xl h-fit mx-auto mt-6 mb-12 py-6 before:table after:table after:clear-both">
        <div className="pt-2 relative py-0 min-h-[1px]">
          <h1 className="uppercase tracking-widest text-2xl font-bold text-center">
            <span className="border-b-2 border-tiger pb-1">
              Browse My Photos
            </span>
          </h1>
        </div>
        <div className="mx-auto w-3/4 relative py-0 min-h-[1px]">
          <p className="text-base text-center font-libre_baskerville mb-5 mt-2">
            Click on an album to view its photos.
          </p>
        </div>
        <Suspense fallback={<div>Loading...</div>}>
          <Content />
        </Suspense>
      </div>
    </main>
  );
};
