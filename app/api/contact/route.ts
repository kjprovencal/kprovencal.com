import { ContactInfo } from '@/utils/types';

export async function POST(request: Request) {
  const contactInfo: ContactInfo = await request.json();
  // Send email
  const res = await fetch(process.env.PB_URL + '/contact', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': process.env.PB_API_KEY || ''
    },
    body: JSON.stringify(contactInfo)
  });
  if (res.status === 200) {
    return new Response('Email sent', { status: 200 });
  } else if (res.status < 500) {
    return new Response('Unauthorized', { status: 400 });
  } else {
    return new Response('Email not sent', { status: res.status, statusText: res.statusText });
  }
}