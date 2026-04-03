import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "OpenChip",
  description: "A free, open source pet microchip registry"
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
