"use client";

import { ReactNode, createContext, useMemo } from 'react';
import PocketBase from 'pocketbase';

export const PBContext = createContext({});

export default function PocketBaseProvider({ children }: { children: ReactNode }) {
  const pb = useMemo(() => {
    const pb = new PocketBase(process.env.PB_URL)
    pb.collection('users')
      .authWithPassword(process.env.PB_USER ?? '', process.env.PB_PASS ?? '')
      .then(res => console.log(pb.authStore.isValid))
      .catch(err => console.error(err))
    return pb;
  }, []);
  return <PBContext.Provider value={{ pb }}>{children}</PBContext.Provider>;
}