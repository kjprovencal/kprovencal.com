/** @type {import('next').NextConfig} */
const nextConfig = {
    images: {
        remotePatterns: [
            {
                protocol: 'http',
                hostname: '127.0.0.1',
                port: '8090',
                pathname: '/api/files/**'
            },
            {
                protocol: 'https',
                hostname: 'api.kprovencal.com',
                port: '',
                pathname: '/api/files/**'
            }
        ]
    }
};

export default nextConfig;
