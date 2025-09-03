/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',        // static export
  trailingSlash: true,     // export as /path/index.html so clean URLs work
  images: { unoptimized: true }, // no image optimizer needed
};
module.exports = nextConfig;
