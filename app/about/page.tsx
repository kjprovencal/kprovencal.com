import Image from 'next/image';
import profilePic from '@/public/images/profilepic.jpg';
import resumeData from '@/lib/resume-data.json';
import type { ResumeMainData } from '@/utils/types';
import Heading from '@/components/layout/heading';

const ProfilePic = ({ className }: { className: string }) => {
   return (
      <div className={`w-[200px] md:w-1/4 relative px-10 py-0 float-left ${className}`}>
         <Image src={profilePic} sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw" className="rounded-full" alt="Kyle Provencal Profile Pic" />
      </div>
   );
}

export default function About() {
   const mainData: ResumeMainData = resumeData.main;
   return (
      <main className="min-h-full mt-20">
         <div className="flex flex-col sm:flex-row items-center md:block">
         <ProfilePic className="md:hidden mt-5" />
         <Heading title="About Me" />
         </div>
         <div className="pt-20 flex md:flex-row items-center">
            <ProfilePic className="hidden md:block"/>
            <div className="mx-auto w-2/3 md:pl-5 md:pr-[10%] py-0 min-h-[1px] relative">
               {mainData.bio.map((paragraph, i) => {
                  return <p key={i} className="leading-8 text-secondary mb-8">{paragraph}</p>;
               })}
            </div>
         </div>
      </main>
   );
}
