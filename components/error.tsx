import { Dispatch, SetStateAction } from "react";
import { FaCircleExclamation } from "react-icons/fa6";

export default function Error({ title, message, onDismiss }: { title?: string; message?: string; onDismiss?: () => void }) {
  return (
    <div
      className="bg-red-100 text-red-500 hover:bg-red-200 focus:ring-red-400 dark:bg-red-200 dark:text-red-600 dark:hover:bg-red-300"
      role="alert"
    >

      <FaCircleExclamation />
      <span className="sr-only">Error:</span>
      <div>
        <span className="font-bold">{title}</span> {message}
      </div>
    </div>
  );
}

export function handleDismissError<s>(setState: Dispatch<SetStateAction<s>>) {
  setState((prev: s) => ({ ...prev, error: { message: '', status: 0 } }));
}