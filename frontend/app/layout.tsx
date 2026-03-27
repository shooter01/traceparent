import './globals.css';
import type { Metadata } from 'next';
import OtelClient from './otel-client';

export const metadata: Metadata = {
  title: 'OTel GitVerse Demo',
  description: 'Next.js frontend for the OTel demo',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru">
      <body>
        <OtelClient />
        {children}
      </body>
    </html>
  );
}
