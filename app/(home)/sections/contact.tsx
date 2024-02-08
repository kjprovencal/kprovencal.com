"use client"
import { useEffect, useMemo, useState } from 'react';
import { Button } from 'flowbite-react';
import { FaGithub, FaInstagram, FaLink, FaLinkedin } from 'react-icons/fa';
import Link from 'next/link';
import resumeData from '../../../lib/resumeData.json';
import { submitContactForm } from '../../actions';
import Error, { reset } from '../../../components/error';
import { Error as AppError } from '../../../utils/types';

export class ContactInfo {
  name: string = '';
  email: string = '';
  subject: string = '';
  message: string = '';
}

function SocialMediaLink({ name }: { name: string }) {
  const social = resumeData.main.social.find(social => social.name === name);
  const icon = useMemo(() => {
    switch (name) {
      case 'linkedin':
        return <FaLinkedin />;
      case 'instagram':
        return <FaInstagram />;
      case 'github':
        return <FaGithub />;
      default:
        return <FaLink />;
    }
  }, [name]);

  return !!social?.url && (
    <Link href={social.url} className='text-4xl text-background dark:text-foreground px-1'>
      {icon}
    </Link>
  );
}

function FormField({ label, name, required = false, type = 'text' }: { label: string; name: string; required?: boolean; type?: string }) {
  return (
    <div className='mb-4 flex flex-col'>
      <label htmlFor={name} className='font-open_sans font-bold text-sm inline-block float-left'>
        {label}
        {required && <span className='text-xs text-orange-400'> *</span>}
      </label>
      {type === 'textarea' ?
        <textarea className='px-3 py-2 text-sm text-secondary rounded-md' /> :
        <input type={type} id={name} className='px-3 py-2 text-sm text-secondary rounded-md' aria-label={name} required={required} />
      }
    </div>
  );
}

export default function Contact() {
  const [canSubmit, setCanSubmit] = useState(true);
  const [isPosting, setIsPosting] = useState(false);
  const [error, setError] = useState<AppError>(reset());

  const submitContactFormWithState = async (formData: FormData) => submitContactForm(formData, {setCanSubmit, setIsPosting, setError});

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
        <form name='contactForm' className='mb-8' action={submitContactFormWithState}>
            <FormField label='Name' name='name' required />
            <FormField label='Email' name='email' type='email' required />
            <FormField label='Subject' name='subject' required />
            <FormField label='Message' name='message' type='textarea' />
            {error && <Error title='Error' message={error.message} onDismiss={() => setError(reset())}/>}
        <div className='flex '>
          <Button disabled={!canSubmit || isPosting} className='bg-accent hover:bg-orange-400 hover:text-secondary w-1/5'>Submit</Button>
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
