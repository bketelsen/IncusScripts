import Footer from "@/components/Footer";
import Navbar from "@/components/Navbar";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { analytics, basePath } from "@/config/siteConfig";
import "@/styles/globals.css";
import { Inter } from "next/font/google";
import { NuqsAdapter } from "nuqs/adapters/next/app";
import React from "react";

const inter = Inter({ subsets: ["latin"] });

export const metadata = {
  title: "Incus Helper Scripts",
  generator: "Next.js",
  applicationName: "Incus Helper Scripts",
  referrer: "origin-when-cross-origin",
  keywords: [
    "Proxmox VE",
    "Helper-Scripts",
    "tteck",
    "helper",
    "scripts",
    "proxmox",
    "VE",
  ],
  authors: { name: "Brian Ketelsen" },
  creator: "Brian Ketelsen",
  publisher: "Brian Ketelsen",
  description:
    "A Front-end for the Incus Helper Scripts (Community) Repository. Featuring over 200+ scripts to help you manage your Incus deployments.",
  favicon: "/app/favicon.ico",
  formatDetection: {
    email: false,
    address: false,
    telephone: false,
  },
  metadataBase: new URL(`https://bketelsen.github.io/${basePath}/`),
  openGraph: {
    title: "Incus Helper Scripts",
    description:
      "A Front-end for the Incus Helper Scripts (Community) Repository. Featuring over 200+ scripts to help you manage your Incus deployments.",
    url: "/defaultimg.png",
    images: [
      {
        url: `https://bketelsen.github.io/${basePath}/defaultimg.png`,
      },
    ],
    locale: "en_US",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          defer
          src={`https://${analytics.url}/script.js`}
          data-website-id={analytics.token}
        ></script>
        <link rel="canonical" href={metadata.metadataBase.href} />
        <link rel="manifest" href="manifest.webmanifest" />
        <link rel="preconnect" href="https://api.github.com" />
      </head>
      <body className={inter.className}>
        <ThemeProvider
          attribute="class"
          defaultTheme="dark"
          enableSystem
          disableTransitionOnChange
        >
          <div className="flex w-full flex-col justify-center">
            <Navbar />
            <div className="flex min-h-screen flex-col justify-center">
              <div className="flex w-full justify-center">
                <div className="w-full max-w-7xl ">
                  <NuqsAdapter>{children}</NuqsAdapter>
                  <Toaster richColors />
                </div>
              </div>
              <Footer />
            </div>
          </div>
        </ThemeProvider>
      </body>
    </html>
  );
}
