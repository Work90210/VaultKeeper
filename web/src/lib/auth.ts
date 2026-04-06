import type { AuthOptions } from 'next-auth';

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} environment variable is required`);
  }
  return value;
}

export const authOptions: AuthOptions = {
  providers: [
    {
      id: 'keycloak',
      name: 'Keycloak',
      type: 'oauth',
      wellKnown: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/.well-known/openid-configuration`,
      authorization: { params: { scope: 'openid email profile' } },
      clientId: requireEnv('KEYCLOAK_CLIENT_ID'),
      clientSecret: requireEnv('KEYCLOAK_CLIENT_SECRET'),
      idToken: true,
      checks: ['pkce', 'state'],
      profile(profile) {
        return {
          id: profile.sub,
          name: profile.preferred_username || profile.name,
          email: profile.email,
        };
      },
    },
  ],
  callbacks: {
    async jwt({ token, account, profile }) {
      // Initial sign in
      if (account && profile) {
        token.accessToken = account.access_token;
        token.refreshToken = account.refresh_token;
        token.expiresAt = account.expires_at;
        token.userId = profile.sub;

        // Extract system role from access token (realm_access is in the AT, not always in profile)
        const roles = extractRolesFromAccessToken(account.access_token);
        token.systemRole = extractSystemRole(roles);

        return token;
      }

      // Return previous token if the access token has not expired
      if (token.expiresAt && Date.now() < token.expiresAt * 1000 - 60_000) {
        return token;
      }

      // Access token has expired, try to refresh
      return refreshAccessToken(token);
    },
    async session({ session, token }) {
      session.accessToken = token.accessToken;
      session.error = token.error;
      session.user = {
        ...session.user,
        id: token.userId || '',
        systemRole: token.systemRole || 'user',
      };
      return session;
    },
  },
  pages: {
    signIn: '/en/login',
    error: '/en/login',
  },
  session: {
    strategy: 'jwt',
    maxAge: 24 * 60 * 60, // 24 hours
  },
};

function extractRolesFromAccessToken(accessToken?: string): string[] {
  if (!accessToken) return [];
  try {
    const payload = accessToken.split('.')[1];
    const decoded = JSON.parse(Buffer.from(payload, 'base64url').toString());
    return decoded?.realm_access?.roles || [];
  } catch {
    return [];
  }
}

function extractSystemRole(roles: string[]): string {
  const roleHierarchy = ['system_admin', 'case_admin', 'user', 'api_service'];
  for (const role of roleHierarchy) {
    if (roles.includes(role)) {
      return role;
    }
  }
  return 'user';
}

async function refreshAccessToken(
  token: Record<string, unknown>
): Promise<Record<string, unknown>> {
  const url = `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/token`;

  const params = new URLSearchParams({
    client_id: process.env.KEYCLOAK_CLIENT_ID || '',
    client_secret: process.env.KEYCLOAK_CLIENT_SECRET || '',
    grant_type: 'refresh_token',
    refresh_token: (token.refreshToken as string) || '',
  });

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: params,
    });

    const refreshed = await response.json();

    if (!response.ok) {
      return { ...token, error: 'RefreshAccessTokenError' };
    }

    return {
      ...token,
      accessToken: refreshed.access_token,
      refreshToken: refreshed.refresh_token ?? token.refreshToken,
      expiresAt: Math.floor(Date.now() / 1000) + refreshed.expires_in,
      error: undefined,
    };
  } catch {
    return { ...token, error: 'RefreshAccessTokenError' };
  }
}
