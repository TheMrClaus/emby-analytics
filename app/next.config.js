/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',        // <-- replaces `next export`
  images: { unoptimized: true }, // no image optimizer needed
};
module.exports = nextConfig;

