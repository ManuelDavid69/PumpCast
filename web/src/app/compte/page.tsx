import type { Metadata } from "next";
import { AccountClient } from "@/components/AccountClient";

export const metadata: Metadata = {
  title: "Mon compte",
  // Page privée : pas besoin qu'elle soit dans les moteurs de recherche
  robots: { index: false },
};

export default function AccountPage() {
  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <h1 className="mb-6 font-board text-4xl font-semibold">Mon compte</h1>
      <AccountClient />
    </div>
  );
}
