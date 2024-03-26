import Image from 'next/image';
import profilePic from '@/public/images/profilepic.jpg';
import resumeData from '@/lib/resume-data.json';
import type { ResumeMainData } from '@/utils/types';
import Heading from '@/components/layout/heading';

export default function About() {
   const mainData: ResumeMainData = resumeData.main;

   return (
      <main className="h-full mt-20">
         <Heading title="About Me" />
         <div className="pt-20 flex items-center">
            <div className="w-1/4 relative px-10 py-0 float-left">
               <Image src={profilePic} sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw" className="rounded-full" alt="Kyle Provencal Profile Pic" />
            </div>
            <div className="mx-auto my-0 w-2/3 pl-5 pr-[10%] py-0 min-h-[1px] float-left relative">
               {mainData.bio.map((paragraph, i) => {
                  return <p key={i} className="leading-8 text-secondary mb-8">{paragraph}</p>;
               })}
            </div>
         </div>
      </main>
   );
}
