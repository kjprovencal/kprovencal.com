export default function fetchAbsolute(url: string, options: any = {}) {
    url = url.startsWith('http') ? url : `${process.env.API_URL || 'localhost:3000'}${url}`;
    console.log('fetching', url);
    console.log('options', options);
    return fetch(url, options);
}