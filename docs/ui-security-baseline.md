# Command Center Security Baseline

The React command center is designed against OWASP ASVS 5.0 controls, but the
repository alone is not a certification or an authorization to operate.

## Implemented controls

- Same-origin architecture; the browser cannot reach PostgreSQL or the Go API
  container directly.
- Opaque, short-lived, HttpOnly, SameSite=Strict sessions. HTTPS deployments
  use a Secure `__Host-` cookie.
- Constant-time credential comparison and CSRF tokens on state-changing calls.
- Strict CSP, clickjacking, MIME sniffing, referrer, resource isolation, and
  browser permissions headers.
- Request body/header limits, HTTP server deadlines, and proxy rate limits.
- Parameterized SQL, JSON schema restrictions, no inline scripts, no source
  maps, and no browser persistence of the bootstrap credential.
- Non-root, read-only UI container with all Linux capabilities removed and
  `no-new-privileges` enabled.
- Required secrets with no Compose fallback credentials.

## Required before a production authorization

- Terminate TLS 1.3 at an approved gateway and set HSTS there. Never expose the
  loopback HTTP development binding to a network.
- Replace the bootstrap login with the organization's OIDC/SAML identity
  provider, phishing-resistant MFA, centralized RBAC, and account lifecycle.
- Replace PostgreSQL plaintext transport with mutually authenticated TLS and a
  least-privilege application role.
- Use SNMPv3 authPriv; the mock network's SNMPv2c `public` community is for
  development only.
- Pin every container by reviewed digest, generate an SBOM, sign artifacts,
  scan images and dependencies, and operate an update/vulnerability process.
- Forward immutable authentication, administrative, and security events to a
  protected audit/SIEM system with synchronized time.
- Complete threat modeling, ASVS verification, penetration testing, backup and
  recovery exercises, key rotation, and the applicable NIST/DISA/FIPS control
  assessment for the deployment environment.
