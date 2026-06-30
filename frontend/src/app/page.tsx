import { redirect } from "next/navigation";

// Entry point → the merchant dashboard (the (app) guard bounces to /login if needed).
export default function Home() {
  redirect("/dashboard");
}
