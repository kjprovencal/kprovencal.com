import Image from 'next/image';
import { FaDownload } from 'react-icons/fa';
import resumeData from '@/lib/resume-data.json';
import type { ResumeMainData } from '@/utils/types';

export default function About() {
   const mainData: ResumeMainData = resumeData.main;

   return (
      <div className="w-[96%] max-w-5xl mx-auto my-0">
         <div className="w-1/4 relative px-5 py-0 float-left ">
            <Image src={'/images/profilepic.jpg'} width={120} height={120} className="rounded-full" alt="Kyle Provencal Profile Pic" />
         </div>
         <div className="w-3/4 pl-5 pr-[10%] py-0 min-h-[1px] float-left relative">
            <h2 className="font-open_sans font-bold text-xl mb-3 text-background dark:text-foreground">
               About Me
            </h2>
            <>{
               mainData.bio.map((paragraph, i) => {
                  return <p key={i} className="leading-8 text-secondary mb-8">{paragraph}</p>;
               })
            }</>
            <div className="w-auto max-w-none my-0">
               <div className="w-[42%] relative py-0 float-left">
                  <h2 className="font-open_sans font-bold text-xl mb-3 text-background dark:text-foreground">
                     Contact Details
                  </h2>
                  <p className="text-secondary leading-8 mb-8">
                     <span>{mainData.name}</span><br />
                     <span>
                        {mainData.address.city} {mainData.address.state}, {mainData.address.zip}
                     </span><br />
                     <span>{mainData.phone}</span><br />
                     <span>{mainData.email}</span>
                  </p>
               </div>
               <div className="text-secondary w-[58%] pt-9 min-h-[1px] float-right relative">
                  <p className="leading-8 text-secondary mb-8">
                     <a href={'/static/resume.pdf'} className="mt-2 mb-4 px-5 py-3 bg-accent text-background dark:text-foreground font-open_sans font-bold text-base/8 inline-flex items-center"><FaDownload className="mr-4 text-xl" />Download Resume</a>
                  </p>
               </div>
            </div>
         </div>
      </div>
   );
}
