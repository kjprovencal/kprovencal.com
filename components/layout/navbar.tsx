"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { FaBars } from "react-icons/fa";
import { NavbarSectionProps, Section } from "@/utils/types";
import useClickOutside from "@/hooks/useClickOutside";

const navbarVariants = {
  common: "py-2 px-4 uppercase font-bold text-sm tracking-widest",
  current: "text-tiger",
  nonCurrent: "text-pakistan dark:text-cornsilk hover:text-tiger",
  home: "text-white hover:text-tiger",
  hamburger: "focus:outline-none",
};

function useTailwindBreakpoint() {
  const [breakpoint, setBreakpoint] = useState<string>("sm");
  useEffect(() => {
    const setBreakpointOnResize = () => {
      if (window.innerWidth >= 1024) {
        setBreakpoint("lg");
      } else if (window.innerWidth >= 768) {
        setBreakpoint("md");
      } else {
        setBreakpoint("sm");
      }
    };
    window.addEventListener("resize", setBreakpointOnResize);
    return () => window.removeEventListener("resize", setBreakpointOnResize);
  }, []);
  return breakpoint;
}

function NavbarSection({ section, current }: NavbarSectionProps) {
  let className = navbarVariants.common;
  const anchor = section === Section.Home ? "/" : `/${section}`;
  if (current === Section.Home) {
    className += ` ${navbarVariants.home}`;
  } else {
    className += ` ${
      section === current ? navbarVariants.current : navbarVariants.nonCurrent
    }`;
  }
  return (
    <li>
      <Link href={anchor} className={className} aria-current="page">
        {section}
      </Link>
    </li>
  );
}

const sections = Object.values(Section);

export default function Navbar() {
  const [isOpen, setIsOpen] = useState(false);
  const hasClickedOutsideMenu = useRef(null);
  const isOutside = useClickOutside(hasClickedOutsideMenu);
  const pathname = usePathname();
  const currentSection =
    pathname === "/" ? Section.Home : (pathname.slice(1) as Section);
  const isHome = currentSection === Section.Home;
  const breakpoint = useTailwindBreakpoint();

  useEffect(() => {
    if (isOutside) {
      setIsOpen(false);
    }
  }, [isOutside]);

  useEffect(() => {
    if (breakpoint !== "sm") {
      setIsOpen(false);
    }
  }, [breakpoint]);

  return (
    <nav
      className={`fixed top-0 start-0 z-20 items-center w-full ${
        isHome
          ? "bg-transparent text-cornsilk dark:text-pakistan"
          : "bg-cornsilk dark:bg-pakistan text-pakistan dark:text-cornsilk"
      }`}
    >
      <div className="max-w-screen-xl flex flex-wrap items-center justify-between mx-auto p-4 sm:hidden">
        <button
          type="button"
          className="relative inline-flex items-center justify-center p-2 w-10 h-10 text-sm rounded-lg md:hidden hover:bg-tiger focus:outline-none focus:ring-2 focus:ring-earth"
          aria-controls="mobile-menu"
          aria-expanded="false"
          onClick={() => setIsOpen((prev) => !prev)}
          ref={hasClickedOutsideMenu}
        >
          <span className="sr-only">Open main menu</span>
          <FaBars className="w-6 h-6" />
        </button>
      </div>
      <div className="hidden sm:block sm:w-auto" id="navbar">
        <ul
          className={`font-open_sans font-bold p-0 mt-0 rtl:space-x-reverse flex flex-row justify-center`}
        >
          {sections.map((sec) => (
            <NavbarSection key={sec} section={sec} current={currentSection} />
          ))}
        </ul>
      </div>
      <div
        className={isOpen ? "space-y-1 px-2 pt-2 pb-3" : "hidden"}
        id="mobile-menu"
      >
        <span>{`Navbar is: ${isOpen ? 'open' : 'closed'}`}</span>
        <ul>
          {sections.map((sec) => (
            <NavbarSection key={sec} section={sec} current={currentSection} />
          ))}
        </ul>
      </div>
    </nav>
  );
}
