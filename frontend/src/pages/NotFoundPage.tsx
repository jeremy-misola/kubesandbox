import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <div className="flex min-h-[55vh] flex-col items-center justify-center text-center">
      <p className="overline text-muted-foreground">
        <span className="text-accent">❯</span> kubectl get page
      </p>
      <h1 className="mt-4 font-display text-8xl font-normal italic tracking-tight text-accent">
        404
      </h1>
      <p className="mt-3 font-mono text-sm text-muted-foreground">
        Error from server (NotFound): page not found
      </p>
      <p className="mt-2 max-w-sm text-sm font-light text-muted-foreground">
        Like an expired sandbox, whatever was here has been cleaned up.
      </p>
      <Link
        to="/"
        className="overline mt-8 text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
      >
        ← go home
      </Link>
    </div>
  );
}
