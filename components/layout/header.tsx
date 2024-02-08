import Navbar from "./navbar";
import Headline from "../sections/headline";

export default function Header({ home }: { home: boolean }) {
  return (
    <header className="w-full h-full text-center">
      {home ?
        <>
          <Navbar />
          <Headline />
        </>
        : <Navbar />
      }
    </header>
  );
}
