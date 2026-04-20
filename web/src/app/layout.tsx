import type { Metadata } from 'next';
import { headers } from 'next/headers';
import './globals.css';
import './design-marketing.css';
import './design-dashboard.css';

export const metadata: Metadata = {
  title: 'VaultKeeper',
  description: 'Sovereign evidence management platform',
};

// Inline script to apply saved theme before first paint, preventing flash.
const themeInitScript = `(function(){try{var t=localStorage.getItem('vk-theme');if(t==='light'||t==='dark')document.documentElement.setAttribute('data-theme',t);}catch(e){}})();`;

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const nonce = (await headers()).get('x-nonce') ?? '';

  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        {/* Load the exact same fonts as the design prototype — via Google Fonts CDN */}
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Fraunces:ital,opsz,wght@0,9..144,300;0,9..144,400;0,9..144,500;1,9..144,300;1,9..144,400&family=Inter:wght@300;400;500;600&family=JetBrains+Mono:wght@400;500&display=swap"
          rel="stylesheet"
        />
        <script nonce={nonce} dangerouslySetInnerHTML={{ __html: themeInitScript }} />
      </head>
      <body>
        {children}
      </body>
    </html>
  );
}
