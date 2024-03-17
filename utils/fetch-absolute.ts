export default function fetchRelative(url: string, options: any = {}) {
    url = url.startsWith('http') ? url : `${process.env.API_URL || 'localhost:3000'}${url}`;
    return fetch(url, options);
}