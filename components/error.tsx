import { Alert } from "flowbite-react";
import { FaCircleExclamation } from "react-icons/fa6";

export function reset() {
  return { message: '', status: 0 };
}

export default function Error({ title, message, onDismiss }: { title?: string; message?: string; onDismiss?: () => void }) {
  return (
    <Alert icon={FaCircleExclamation} color="failure" onDismiss={onDismiss}>
      <span className="font-bold">{title}</span> {message}
    </Alert>
  );
}