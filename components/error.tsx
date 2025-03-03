import { Dispatch, SetStateAction } from "react";
import { FaCircleXmark } from "react-icons/fa6";

export default function Error({
  title,
  message,
  onDismiss,
}: {
  title: string;
  message: string;
  onDismiss?: () => void;
}) {
  return (
    <div
      className="bg-red-100 text-red-500 hover:bg-red-200 focus:ring-red-400 dark:bg-red-200 dark:text-red-600 dark:hover:bg-red-300 rounded-xs my-2"
      role="alert"
    >
      <span className="sr-only">Error</span>
      <div className="flex flex-row p-2">
        <span className="grow">
          <span className="font-bold">{title}: </span>
          <p>{message}</p>
        </span>
        <button onClick={onDismiss} aria-label="Dismiss">
          <FaCircleXmark />
        </button>
      </div>
    </div>
  );
}

export function handleDismissError<s>(setState: Dispatch<SetStateAction<s>>) {
  setState((prev: s) => ({ ...prev, error: { message: "", status: 0 } }));
}
