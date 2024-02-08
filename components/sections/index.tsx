export enum Section {
  Home = 'home',
  About = 'about',
  Resume = 'resume',
  Gallery = 'gallery',
  Contact = 'contact'
}

export class HomepageSection {
  constructor(public id: Section, public className: string) { };
  static Home = new HomepageSection(Section.Home, '');
  static About = new HomepageSection(Section.About, '');
  static Resume = new HomepageSection(Section.Resume, '');
  static Gallery = new HomepageSection(Section.Gallery, '');
  static Contact = new HomepageSection(Section.Contact, '');
}

export class SectionState {
  home: boolean;
  about: boolean;
  resume: boolean;
  gallery: boolean;
  contact: boolean;

  constructor(section?: Section) {
    this.home = false;
    this.about = false;
    this.resume = false;
    this.gallery = false;
    this.contact = false;
    if (section) {
      this[section] = true;
    } else {
      this.home = true;
    }
  }

  getCurrent() {
    if (this.home) return HomepageSection.Home.id;
    if (this.about) return HomepageSection.About.id;
    if (this.resume) return HomepageSection.Resume.id;
    if (this.gallery) return HomepageSection.Gallery.id;
    if (this.contact) return HomepageSection.Contact.id;
    return HomepageSection.Home.id;
  }

  setSection(section: Section, value: boolean) {
    this[section] = value;
  }

  clone(state: SectionState) {
    this.home = state.home;
    this.about = state.about;
    this.resume = state.resume;
    this.gallery = state.gallery;
    this.contact = state.contact;
  }
}

