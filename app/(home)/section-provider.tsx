"use client";

import { ReactNode, createContext, useState } from "react";
import { SectionContextType } from "../../utils/types";
import { SectionState } from "../../components/sections";

export const SectionContext = createContext<SectionContextType>({sectionState: new SectionState(), setSectionState: () => {}});

export default function SectionProvider({ children }: { children: ReactNode }) {
  const [sectionState, setSectionState] = useState(new SectionState());
  return <SectionContext.Provider value={{ sectionState, setSectionState }}>
    {children}
  </SectionContext.Provider>;
}