import { load } from "recaptcha-v3";

export async function GET() {
  if (process.env.NODE_ENV === 'production') {
    try {
      const recaptcha = await load(process.env.RECAPTCHA_CLIENT_KEY || '');
      const token = await recaptcha.execute('contact');
      if (!token) throw new Error('Recaptcha failed');
    } catch (err) {
      if ((err as Error).message === 'Recaptcha failed') {
        return new Response('Recaptcha failed', { status: 400 });
      } else {
        return new Response('Recaptcha not loaded', { status: 500 });
      }
    }
  }
}