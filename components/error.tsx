import { Alert } from "flowbite-react";
import { Dispatch, SetStateAction } from "react";
import { FaCircleExclamation } from "react-icons/fa6";

export default function Error({ title, message, onDismiss }: { title?: string; message?: string; onDismiss?: () => void }) {
  return (
    <Alert icon={FaCircleExclamation} color="failure" onDismiss={onDismiss}>
      <span className="font-bold">{title}</span> {message}
    </Alert>
  );
}

export function handleDismissError<s>(setState: Dispatch<SetStateAction<s>>) {
  setState((prev: s) => ({ ...prev, error: { message: '', status: 0 } }));
}