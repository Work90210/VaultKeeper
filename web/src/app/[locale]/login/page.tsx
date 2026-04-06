export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-md space-y-6 rounded-lg border p-8 shadow-sm">
        <div className="space-y-2 text-center">
          <h1 className="text-2xl font-bold">VaultKeeper</h1>
          <p className="text-muted-foreground text-sm">
            Sovereign evidence management platform
          </p>
        </div>
        <button
          className="w-full rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800"
          type="button"
        >
          Sign in with Keycloak
        </button>
      </div>
    </main>
  );
}
