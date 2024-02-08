import { ContactInfo } from '../../../components/sections/contact';

export async function POST(request: Request) {
  const contactInfo: ContactInfo = await request.json();
  // Send email
  const res = await fetch('/contact', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(contactInfo) });
  if (res.status === 200) {
    return new Response('Email sent', { status: 200 });
  } else if (res.status < 500) {
    return new Response('Unauthorized', { status: 400 });
  } else {
    return new Response('Email not sent', { status: 500 });
  }
}