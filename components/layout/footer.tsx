import resumeData from '@/lib/resume-data.json';
import { FaChevronCircleUp } from 'react-icons/fa';
import { SocialMediaLink } from '../social-media-link';

export default function Footer({ home }: { home?: boolean }) {
  return (
    <footer className="text-center py-6 text-sm relative ">
      <div className="w-full relative px-5 py-0 min-h-1 float-left">
        <p className="text-xs text-tiger dark:text-cornsilk">
          &copy; {new Date().getFullYear()} {resumeData.main.name}. All rights reserved.
        </p>
      </div>
    </footer>
  );
}
