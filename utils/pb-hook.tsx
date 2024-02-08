import PocketBase from 'pocketbase';

// may need to replace with a hook that can read/modify server req/res
async function getPB(pb: PocketBase) {
  try {
    await pb.collection('users').authWithPassword(process.env.PB_USER ?? '', process.env.PB_PASS ?? '')
    pb.authStore.isValid && await pb.collection('users').authRefresh();
  } catch (err) {
    pb.authStore.clear();
    throw err;
  }
}

export default function initPocketBase(): PocketBase {
  const pb = new PocketBase(process.env.PB_URL);
  getPB(pb).catch(err => console.error(err));
  return pb;
}