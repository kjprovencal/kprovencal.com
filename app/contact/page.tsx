"use client";
import { useEffect, useState, useActionState } from "react";
import { Turnstile } from "@marsidev/react-turnstile";
import resumeData from "@/lib/resume-data.json";
import { submitContactForm } from "@/app/actions";
import Error, { handleDismissError } from "@/components/error";
import { SocialMediaLink } from "@/components/social-media-link";
import { FormField } from "@/components/form-field";
import { ContactState } from "@/utils/types";

const initialState: ContactState = {
  contactInfo: {
    name: "",
    email: "",
    subject: "",
    message: "",
  },
  canSubmit: true,
  error: {
    message: "",
    status: 0,
  },
};

export default function Contact() {
  const [contactState, setContactState] = useState<ContactState>(initialState);
  const [state, formAction] = useActionState<ContactState, FormData>(
    submitContactForm,
    initialState
  );

  useEffect(() => setContactState({ ...state }), [state]);

  return (
    <main className="min-h-full flex items-center">
      <div className="w-[96%] max-w-5xl h-fit mx-auto mt-12 py-6 before-table">
        <div className="pt-2 relative py-0 min-h-[1px] mb-8">
          <h1 className="uppercase tracking-widest text-2xl font-bold text-center">
            <span className="border-b-2 border-tiger pb-1">Get In Touch</span>
          </h1>
        </div>

        <div className="mx-auto w-3/4 relative py-0 min-h-[1px]">
          <p className="text-base text-center dark:text-secondary font-libre_baskerville mb-5 mt-2">
            {resumeData.main.contactMessage}
          </p>
        </div>

        <div className="w-2/3 mx-auto rounded-md px-3 py-2 ">
          <form name="contactForm" className="mb-8" action={formAction}>
            <FormField label="Name" name="name" required />
            <FormField label="Email" name="email" type="email" required />
            <FormField label="Subject" name="subject" required />
            <FormField label="Message" name="message" type="textarea" />
            {!!contactState.error?.status && (
              <Error
                title="Error"
                message={contactState.error?.message}
                onDismiss={() => handleDismissError(setContactState)}
              />
            )}
            <div className="flex flex-col sm:flex-row pb-5">
              <button
                type="submit"
                disabled={!contactState.canSubmit}
                className="dark:bg-cornsilk dark:text-tiger dark:hover:bg-tiger dark:hover:text-cornsilk bg-tiger text-cornsilk hover:bg-pakistan sm:w-1/5 rounded-xs mb-5 py-2"
              >
                Submit
              </button>
              <div className="sm:grow"></div>
              <div className="flex flex-row">
                <SocialMediaLink name="linkedin" />
                <SocialMediaLink name="instagram" />
                <SocialMediaLink name="github" />
              </div>
            </div>
          </form>
        </div>
        {process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY && (
          <Turnstile
            className="flex justify-center items-center mx-auto"
            siteKey={process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY}
          />
        )}
      </div>
    </main>
  );
}
