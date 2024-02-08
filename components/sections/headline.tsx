import resumeData from '../../lib/resumeData.json';
import { ResumeMainData } from '../../utils/types';
import Link from 'next/link';
import { FaChevronCircleDown, FaLinkedin, FaInstagram, FaGithub } from "react-icons/fa";


export default function Headline() {
  const mainData: ResumeMainData = resumeData.main;
  return (
    <section className="h-[800px] min-h-[500px] bg-center bg-no-repeat bg-cover bg-header  bg-blend-multiply">
      <div className="px-4 mx-auto max-w-screen-xl text-center py-24 lg:py-56">
        <h1 className="mb-4 text-7xl tracking-wide leading-none text-white md:text-[80px] lg:text-8xl font-bold text-shadow-lg">Hello!</h1>
        <p className="mx-auto my-0 w-[70%] text-lg text-shadow-xl font-libre_baskerville font-normal text-white lg:text-xl">{mainData.description}</p>
        <ul className="social py-6">
          <li className="inline-block mx-2">
            <Link href={mainData.social[0].url}>
              <FaLinkedin className="text-4xl text-white hover:text-orange-500" />
            </Link>
          </li>
          <li className="inline-block mx-2">
            <Link href={mainData.social[1].url}>
              <FaInstagram className="text-4xl text-white hover:text-orange-500" />
            </Link>
          </li>
          <li className="inline-block mx-2">
            <Link href={mainData.social[2].url}>
              <FaGithub className="text-4xl text-white hover:text-orange-500" />
            </Link>
          </li>
        </ul>
      </div>
      <Link className="scroll-smooth" href="#about">
        <FaChevronCircleDown className="text-4xl text-white animate-bounce mx-auto" />
      </Link>
    </section>
  );

}