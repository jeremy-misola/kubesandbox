import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <div className="flex min-h-[55vh] flex-col items-center justify-center text-center">
      <p className="font-mono text-sm text-muted-foreground">
        <span className="text-primary">❯</span> kubectl get page
      </p>
      <h1 className="mt-3 font-display text-6xl font-bold tracking-tight text-primary">
        404
      </h1>
      <p className="mt-2 font-mono text-sm text-muted-foreground">
        Error from server (NotFound): page not found
      </p>
      <p className="mt-1 max-w-sm text-sm text-muted-foreground">
        Like an expired sandbox, whatever was here has been cleaned up.
      </p>
      <Link
        to="/"
        className="mt-6 rounded font-mono text-sm text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        ← go home
      </Link>
    </div>
  );
}
