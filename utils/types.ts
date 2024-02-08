import { SectionState } from "../app/(home)/sections";
import PocketBase from 'pocketbase';

export type PBContextType = {
  pb: PocketBase;
};

export type SectionContextType = {
  sectionState: SectionState,
  setSectionState: (state: SectionState) => void
}

export type SocialType = {
  name: string;
  url: string;
  className: string;
};

export type ResumeMainData = {
  name: string;
  occupation: string;
  description: string;
  image: string;
  bio: string;
  contactMessage: string;
  email: string;
  phone: string;
  address: {
    city: string;
    state: string;
    zip: string;
  };
  website: string;
  resumeDownload: string;
  social: Array<SocialType>;
};

export type ResumeParams = {
  slug: string;
};

export type Error = {
  message: string;
  status: number;
};