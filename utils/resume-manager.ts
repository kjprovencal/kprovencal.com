import { EducationEntry, SkillsEntry, WorkEntry } from '@/lib/resume-entry';
import PocketBase from 'pocketbase';

export default class ResumeManager {
  education: EducationEntry[] = [];
  skills: SkillsEntry[] = [];
  work: WorkEntry[] = [];
  pb: PocketBase;

  constructor(pb: PocketBase) {
    this.pb = pb;
  }

  async getPBRecords(collection: string, sort: string) {
    try {
      const records = await this.pb.collection(collection).getFullList({ sort });
      return records;
    } catch (err) {
      console.error(err);
      return [];
    }
  }
}
