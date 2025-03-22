/** @type {import('next').NextConfig} */
const nextConfig = {
  rewrites: async () => {
    return [
      {
        source: '/pyapi/:path*',
        destination:
          process.env.NODE_ENV === 'development'
            ? 'http://127.0.0.1:5328/pyapi/:path*'
            : '/pyapi/',
      },
    ]
  },
}

module.exports = nextConfig
