import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center text-center">
      <h1 className="text-3xl font-bold">404</h1>
      <p className="mt-2 text-muted-foreground">This page doesn't exist.</p>
      <Link to="/" className="mt-4 text-primary hover:underline">
        Go home
      </Link>
    </div>
  );
}
