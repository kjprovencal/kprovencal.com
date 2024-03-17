import PocketBase from 'pocketbase';

export type ContactInfo = {
  name: string;
  email: string;
  subject: string;
  message: string;
}

export type ContactState = {
  contactInfo: ContactInfo;
  canSubmit: boolean;
  error: {
    message: string;
    status: number;
  };
};

export type PBContextType = {
  pb: PocketBase;
};

export type SocialType = {
  name: string;
  url: string;
};

export type ResumeMainData = {
  name: string;
  occupation: string;
  description: string;
  image: string;
  bio: Array<string>;
  contactMessage: string;
  resumeDownload: string;
  social: Array<SocialType>;
};

export type Error = {
  message: string;
  status: number;
};

export type NavbarSectionProps = {
  section: Section;
  current: Section;
};

export enum Section {
  Home = 'home',
  About = 'about',
  Resume = 'resume',
  Gallery = 'gallery',
  Contact = 'contact'
}
