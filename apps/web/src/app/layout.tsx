import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { IconSprite, PrefsBoot, RouteFocus, SkipLink } from "@clusterx/ui";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "CLUSTERX: jobs that outlive their workers", template: "%s: CLUSTERX" },
  description: "Open-source, self-hosted distributed job processing. At-least-once, fenced, inspectable.",
};
export const viewport: Viewport = { width: "device-width", initialScale: 1, themeColor: "#FFFFFF" };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <link rel="preload" href="/fonts/ibm-plex-sans-latin-400-normal.woff2" as="font" type="font/woff2" crossOrigin="anonymous" />
        <link rel="preload" href="/fonts/ibm-plex-mono-latin-400-normal.woff2" as="font" type="font/woff2" crossOrigin="anonymous" />
        <link rel="preload" href="/fonts/newsreader-latin-wght-normal.woff2" as="font" type="font/woff2" crossOrigin="anonymous" />
        <link rel="preload" href="/fonts/ibm-plex-sans-latin-600-normal.woff2" as="font" type="font/woff2" crossOrigin="anonymous" />
        {/* Marks the document so reveal animations only apply once script runs (content is complete without JS). */}
        <script dangerouslySetInnerHTML={{ __html: "document.documentElement.classList.add('js')" }} />
      </head>
      <body className="bg-white text-ink">
        <IconSprite />
        <SkipLink />
        <PrefsBoot />
        <RouteFocus />
        {children}
      </body>
    </html>
  );
}
