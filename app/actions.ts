"use server"

import fetchAbsolute from "@/utils/fetch-absolute";
import { ContactInfo, ContactState } from "@/utils/types";
import { cookies } from 'next/headers';


export async function submitContactForm(prevState: ContactState, formData: FormData): Promise<ContactState> {
  // prevent spamming
  if ((prevState?.canSubmit || false) === false) {
    return {
      ...prevState,
      canSubmit: false,
      error: { message: 'Unauthorized', status: 401 }
    };
  }
  const lastSubmit = cookies().get('contactFormLastSubmit')?.value;
  if (lastSubmit) {
    const lastSubmitDate = new Date(lastSubmit);
    const now = new Date();
    const diff = now.getTime() - lastSubmitDate.getTime();
    if (diff < 1000 * 60 * 60 * 24 * 7) {
      return {
        ...prevState,
        canSubmit: false,
        error: { message: 'You have already submitted this form.', status: 400 }
      };
    }
    console.log('last submit', lastSubmitDate);
  }
  const contactInfo: ContactInfo = {
    name: formData.get('name') as string,
    email: formData.get('email') as string,
    subject: formData.get('subject') as string,
    message: formData.get('message') as string,
  };
  const recpatcha = await fetchAbsolute('/api/validate');
  if (recpatcha.status === 400) {
    return {
      ...prevState,
      canSubmit: false,
      error: { message: 'Recaptcha failed', status: 400 }
    };
  } else if (recpatcha.status === 500) {
    return {
      ...prevState,
      canSubmit: false,
      error: { message: 'Recaptcha not validated', status: 500 }
    };
  }

  const contactRes = await fetchAbsolute('/api/contact', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(contactInfo)
  }).catch(err => {
    {
      return {
        ...prevState,
        canSubmit: false,
        error: { message: 'Failed to send email', status: 500 }
      };

    }
  }).then(res => {
    const error = { message: 'Failed to send email', status: 500 };
    if ((res instanceof Response && res.status === 200) || ((res as ContactState).error?.status === 200)) {
      cookies().set('contactFormLastSubmitted', new Date().toISOString());
      error.message = 'Email sent successfully';
      error.status = 200;
    }
    return {
      ...prevState,
      canSubmit: error.status !== 200,
      error
    };
  });
  return contactRes;
}