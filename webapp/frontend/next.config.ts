/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  compress: false, // 由 Nginx 壓縮
  poweredByHeader: false,
  modularizeImports: {
    "@mui/material": { transform: "@mui/material/{{member}}" },
    "@mui/icons-material": { transform: "@mui/icons-material/{{member}}" },
  },
  headers: async () => {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "X-Frame-Options", value: "SAMEORIGIN" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        ],
      },
    ];
  },
};
export default nextConfig;
