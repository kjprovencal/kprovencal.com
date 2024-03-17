"use client";
import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { NavbarSectionProps, Section } from "@/utils/types";
import { FaHamburger } from "react-icons/fa";

const navbarVariants = {
  common: "py-2 px-4 uppercase font-bold text-sm tracking-widest",
  current: "text-tiger",
  nonCurrent: "text-pakistan dark:text-cornsilk hover:text-tiger",
  home: "text-white hover:text-tiger",
  hamburger: "focus:outline-none",
}

function NavbarSection({ section, current }: NavbarSectionProps) {
  let className = navbarVariants.common;
  const anchor = section === Section.Home ? '/' : `/${section}`;
  if (current === Section.Home) {
    className += ` ${navbarVariants.home}`;
  } else {
    className += ` ${section === current ? navbarVariants.current : navbarVariants.nonCurrent}`;
  }
  return (
    <li>
      <Link href={anchor} className={className} aria-current="page">{section}</Link>
    </li>
  )
}

const sections = Object.values(Section);

export default function Navbar() {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const pathname = usePathname();
  const toggleMenu = () => {
    setIsMenuOpen(!isMenuOpen);
  };

  const currentSection = pathname === '/' ? Section.Home : pathname.slice(1) as Section;
  const isHome = currentSection === Section.Home;
  return (
    <nav className={`w-full z-20 top-0 left-0 fixed ${isHome ? "bg-transparent" : "bg-cornsilk dark:bg-pakistan"}`}>
      <div className="max-w-screen-xl items-center justify-between mx-auto" id="navbar-sticky">
        <div className={`${isMenuOpen ? "block" : "hidden"}`}>
          <button
            className="text-cornsilk hover:text-pakistan focus:outline-none"
            onClick={toggleMenu}
          >
            <FaHamburger className="fill-tiger" />
          </button>
        </div>
        <ul className={`font-open_sans font-bold p-0 md:p-4 mt-0 md:my-2 hidden md:flex md:flex-row md:justify-center`}>
          {sections.map((sec) => <NavbarSection key={sec} section={sec} current={currentSection} />)}
        </ul>
      </div>
    </nav>
  );
}