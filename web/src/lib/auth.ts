export function getKeycloakConfig() {
  return {
    issuer: process.env.KEYCLOAK_URL
      ? `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}`
      : '',
    clientId: process.env.KEYCLOAK_CLIENT_ID || 'vaultkeeper-web',
  };
}
