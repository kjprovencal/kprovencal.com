import resumeData from '@/lib/resume-data.json';

export default function Footer() {
  return (
    <footer className="text-center py-6 text-sm relative min-h-1 max-h-12">
      <div className="w-full relative px-5 py-0 float-left">
        <p className="text-xs text-tiger dark:text-cornsilk">
          &copy; {new Date().getFullYear()} {resumeData.main.name}. All rights reserved.
        </p>
      </div>
    </footer>
  );
}
