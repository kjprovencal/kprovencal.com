"use client"
import { useEffect, useMemo, useState } from 'react';
import { useFormState } from 'react-dom';
import { Button } from 'flowbite-react';
import resumeData from '@/lib/resume-data.json';
import { submitContactForm } from '@/app/actions';
import Error, { handleDismissError } from '@/components/error';
import { SocialMediaLink } from '@/components/social-media-link';
import { FormField } from '@/components/form-field';
import { ContactState } from '@/utils/types';


const initialState: ContactState = {
  contactInfo: {
    name: '',
    email: '',
    subject: '',
    message: '',
  },
  canSubmit: true,
  error: {
    message: '',
    status: 0,
  }
};

export default function Contact() {
  const [contactState, setContactState] = useState<ContactState>(initialState);
  const [state, formAction] = useFormState<ContactState, FormData>(submitContactForm, initialState);

  useEffect(() => {
    const newState = {...state} as ContactState;
    setContactState(newState);
  }, [state])

  return (
    <div className='w-[96%] max-w-5xl h-fit mx-auto mt-6 mb-12 py-6 before:table'>
      <div className='pt-2 relative py-0 min-h-[1px]'>
        <h1 className="uppercase tracking-widest text-2xl font-bold text-accent dark:text-foreground text-center">
          <span className="border-b-2 border-orange-400 pb-1">Get In Touch</span>
        </h1>
      </div>

      <div className='mx-auto w-3/4 relative py-0 min-h-[1px]'>
        <p className='text-base text-center dark:text-secondary font-libre_baskerville mb-5 mt-2'>{resumeData.main.contactMessage}</p>
      </div>

      <div className='w-2/3 bg-altBackground mx-auto rounded-md px-3 py-2'>
        <form name='contactForm' className='mb-8' action={formAction}>
          <FormField label='Name' name='name' required />
          <FormField label='Email' name='email' type='email' required />
          <FormField label='Subject' name='subject' required />
          <FormField label='Message' name='message' type='textarea' />
          {!!contactState.error?.status &&
            <Error title='Error' message={contactState.error?.message} onDismiss={() => handleDismissError(setContactState)} />}
          <div className='flex '>
            <Button type='submit' disabled={!contactState.canSubmit} className='bg-accent hover:bg-orange-400 hover:text-secondary w-1/5'>Submit</Button>
            <div className='flex-grow'></div>
            <div className='flex flex-row'>
              <SocialMediaLink name='linkedin' />
              <SocialMediaLink name='instagram' />
              <SocialMediaLink name='github' />
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
