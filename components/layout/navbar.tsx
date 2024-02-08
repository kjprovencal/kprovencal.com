"use client";
import { useContext } from "react";
import { SectionContext } from "../../app/(home)/section-provider";
import { SectionState, Section } from "../../app/(home)/sections";

const common = "w-full z-20 top-0 left-0 fixed ";
const transparent = common + "bg-transparent";
const opaque = common + "bg-[#333]";

function NavbarSection({ section }: { section: Section }) {
  const {sectionState, setSectionState} = useContext(SectionContext);
  const className = section === sectionState.getCurrent() ? "bg-blue-400 rounded md:bg-transparent md:text-orange-400 md:p-0" : "text-white rounded hover:bg-gray-100 md:hover:bg-transparent md:hover:text-orange-400 md:p-0 dark:text-white dark:hover:bg-gray-700 md:dark:hover:bg-transparent dark:border-gray-700";
  const anchor = section === Section.Home ? '#' : '#' + section;
  const handleSectionClick = () => {
    setSectionState(new SectionState(section));
  }
  return (
    <li>
      <a href={anchor} className={className + " block py-2 pl-3 pr-4 uppercase font-bold text-sm tracking-widest"} aria-current="page"
      onClick={handleSectionClick}>{section}</a>
    </li>
  )
}

const sections = Object.values(Section);

export default function Navbar() {
  const sectionState = useContext(SectionContext).sectionState;
  return (
    <nav className={sectionState.home ? transparent : opaque}>
      <div className="max-w-screen-xl flex flex-wrap items-center justify-between mx-auto p-4">
        <div className="items-center justify-between w-full md:flex md:w-auto md:order-1 mx-auto" id="navbar-sticky">
          <ul className="font-open_sans font-bold flex flex-col p-4 md:p-0 mt-4 md:flex-row md:space-x-8 md:mt-0 ">
          {
            sections.map(sec => <NavbarSection key={sec} section={sec} />)
           }
          </ul>
        </div>
      </div>
    </nav>
  );
}