"use server"

import { ContactInfo } from "../components/sections/contact";
import { cookies } from 'next/headers'
export async function submitContactForm(formData: FormData, state: any) {
  // prevent spamming
  const lastSubmit = cookies().get('contactFormLastSubmit')?.value;
  if (lastSubmit) {
    const lastSubmitDate = new Date(lastSubmit);
    const now = new Date();
    const diff = now.getTime() - lastSubmitDate.getTime();
    if (diff < 1000 * 60 * 60 * 24 * 7) {
      state.setCanSubmit(false);
      state.setError({message: 'You can only submit the form once a week', status: 400});
      return;
    }
    console.log('last submit', lastSubmitDate);
  }
  const contactInfo: ContactInfo = {
    name: formData.get('name') as string,
    email: formData.get('email') as string,
    subject: formData.get('subject') as string,
    message: formData.get('message') as string,
  };
  state.setIsPosting(true);
  const recpatcha = await fetch('/api/validate', { method: 'GET' });
  if (recpatcha.status === 400) {
    state.setError({message: 'Recaptcha failed', status: 400});
    state.setIsPosting(false);
    state.setCanSubmit(false);
    return;
  } else if (recpatcha.status === 500) {
    state.setError({message: 'Recaptcha not loaded', status: 500});
    state.setIsPosting(false);
    return;
  }

  fetch('/api/contact', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(contactInfo)
  }).catch(err => {
    if (err.status === 400) {
      state.setError({message: 'Unauthorized', status: 400});
      state.setCanSubmit(false);
    } else {
      state.setError({message: 'Email not sent', status: 500});
    }
  }).then(res => {
    if (res && res.status === 200) {
      cookies().set('contactFormLastSubmitted', new Date().toISOString());
      state.setError({message: 'Email sent', status: 200 });
      state.setCanSubmit(false);
    }
  }).finally(() => state.setIsPosting(false));
}