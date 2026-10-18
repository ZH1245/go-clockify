import type { Metadata } from "next";
import Link from "next/link";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

import { TimerHeartbeat } from "@/components/timer-heartbeat";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Time tracker",
  description: "A small Clockify-style time tracker",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        <nav className="border-b border-zinc-200 dark:border-zinc-800">
          <div className="mx-auto flex w-full max-w-3xl items-center gap-5 px-4 py-3 text-sm">
            <Link href="/" className="font-semibold">
              Time tracker
            </Link>
            <Link href="/dashboard" className="text-zinc-500 transition-colors hover:text-zinc-900 dark:hover:text-zinc-100">
              Dashboard
            </Link>
            <Link href="/projects" className="text-zinc-500 transition-colors hover:text-zinc-900 dark:hover:text-zinc-100">
              Projects
            </Link>
          </div>
        </nav>
        <TimerHeartbeat />
        {children}
      </body>
    </html>
  );
}
