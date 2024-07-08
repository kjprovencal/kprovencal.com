import { RefObject, useEffect, useState } from "react";

export default function useClickOutside(
  ref: RefObject<any>,
) {
  const [isOutside, setIsOutside] = useState(false);
  useEffect(() => {
    function handleClickOutside(event: { target: any }) {
      if (ref.current && !ref.current.contains(event.target)) {
        setIsOutside(true);
      } else {
        setIsOutside(false);
      }
    }
    document.addEventListener("click", handleClickOutside);
    return () => document.removeEventListener("click", handleClickOutside);
  }, [ref]);
  return isOutside;
}
