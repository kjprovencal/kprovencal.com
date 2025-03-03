import resumeData from '@/lib/resume-data.json';
import { ResumeMainData } from '@/utils/types';
import { SocialMediaLink } from '@/components/social-media-link';

export default async function Home() {
  const mainData: ResumeMainData = resumeData.main;
  return (
    <main className="min-h-full bg-center bg-no-repeat bg-cover bg-header bg-blend-multiply">
      <div className="px-4 mx-auto max-w-(--breakpoint-xl) text-center py-24 lg:py-56">
        <h1 className="mb-4 text-7xl tracking-wide leading-none text-white md:text-[80px] lg:text-8xl font-bold text-shadow-lg">Hello!</h1>
        <p className="mx-auto my-0 w-[70%] text-lg text-shadow-xl font-libre_baskerville font-normal text-white lg:text-xl">{mainData.description}</p>
        <ul className="social py-6 text-white">
          <li className="inline-block mx-2">
            <SocialMediaLink name="linkedin" />
          </li>
          <li className="inline-block mx-2">
            <SocialMediaLink name="instagram" />
          </li>
          <li className="inline-block mx-2">
            <SocialMediaLink name="github" />
          </li>
        </ul>
      </div>
    </main>
  );
}
