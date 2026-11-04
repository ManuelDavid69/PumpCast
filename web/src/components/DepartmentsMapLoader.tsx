"use client";

import dynamic from "next/dynamic";

// next/dynamic avec ssr: false n'est autorisé que dans un composant client
export const DepartmentsMapLoader = dynamic(() => import("@/components/DepartmentsMap"), {
  ssr: false,
  loading: () => <div className="h-full w-full animate-pulse bg-line/40" />,
});
