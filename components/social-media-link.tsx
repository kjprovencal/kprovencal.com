"use client";
import { useMemo } from 'react';
import { FaGithub, FaInstagram, FaLink, FaLinkedin } from 'react-icons/fa';
import Link from 'next/link';
import resumeData from '@/lib/resume-data.json';

export function SocialMediaLink({ name }: { name: string; }) {
  const social = resumeData.main.social.find(social => social.name === name);
  const icon = useMemo(() => {
    switch (name) {
      case 'linkedin':
        return <FaLinkedin />;
      case 'instagram':
        return <FaInstagram />;
      case 'github':
        return <FaGithub />;
      default:
        return <FaLink />;
    }
  }, [name]);

  return !!social?.url && (
    <Link href={social.url} className='text-4xl px-1 hover:text-tiger'>
      {icon}
    </Link>
  );
}
