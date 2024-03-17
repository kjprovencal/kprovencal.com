export class Record {
  id: string;
  collectionId: string;
  constructor(data: any) {
    this.id = data.id;
    this.collectionId = data.collectionId;
  }
}
export class EducationEntry extends Record {
  degree: string;
  description: string;
  school: string;
  graduated: string;
  
  constructor(data: any) {
    super(data);
    this.degree = data.degree;
    this.description = data.description;
    this.school = data.school;
    this.graduated = data.graduated;
  }
};

export class WorkEntry extends Record {
  company: string;
  description: string;
  title: string;
  start: Date;
  end: Date;
  years: string;

  constructor(data: any) {
    super(data);
    this.company = data.company;
    this.description = data.description;
    this.title = data.title;
    this.start = data.start;
    this.end = data.end;
    this.years = data.years;
  }
};

export class SkillsEntry extends Record {
  level: number;
  name: string;
  constructor(data: any) {
    super(data);
    this.level = data.level;
    this.name = data.name;
  }
};

export class AlbumType extends Record {
  id: string;
  title: string;
  url: string;
  thumbnail: string;
  constructor(data: any) {
    super(data);
    this.id = data.id;
    this.title = data.title;
    this.url = data.url;
    this.thumbnail = data.thumbnail;
   }
};