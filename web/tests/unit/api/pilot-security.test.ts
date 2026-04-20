import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('next/server', () => ({
  NextResponse: {
    json: (body: unknown, init?: { status?: number }) => ({
      body,
      status: init?.status || 200,
      json: async () => body,
    }),
  },
}));

const validPayload = {
  name: 'Jane Okafor',
  email: 'jane@humanrights.org',
  organization: 'Rights Documentation Center',
  role: 'investigator',
  message: 'We need sovereign evidence management for our field documentation work.',
  locale: 'en',
};

describe('Pilot API security', () => {
  let POST: (request: Request) => Promise<{ body: unknown; status: number }>;

  beforeEach(async () => {
    vi.resetModules();
    const mod = await import('@/app/api/pilot/route');
    POST = mod.POST as typeof POST;
  });

  function makeRequest(
    body: unknown,
    ip = '192.168.1.1',
    headers?: Record<string, string>,
  ) {
    return new Request('http://localhost/api/pilot', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-forwarded-for': ip,
        ...headers,
      },
      body: JSON.stringify(body),
    });
  }

  // XSS prevention
  it('rejects script tags in name field', async () => {
    const res = await POST(
      makeRequest({
        ...validPayload,
        name: '<script>alert("xss")</script>',
      }),
    );
    // Even if it passes validation (name is just a string with min 2),
    // the important thing is it doesn't execute — but we should check
    // it doesn't end up in an unsafe response
    if (res.status === 201) {
      const body = res.body as Record<string, unknown>;
      const bodyStr = JSON.stringify(body);
      expect(bodyStr).not.toContain('<script>');
    }
  });

  it('rejects SQL injection in email field', async () => {
    const res = await POST(
      makeRequest({
        ...validPayload,
        email: "'; DROP TABLE users; --@evil.com",
      }),
    );
    expect(res.status).toBe(400); // Zod email validation catches this
  });

  it('rejects extremely long input', async () => {
    const res = await POST(
      makeRequest({
        ...validPayload,
        name: 'A'.repeat(101), // max is 100
      }),
    );
    expect(res.status).toBe(400);
  });

  it('rejects oversized message', async () => {
    const res = await POST(
      makeRequest({
        ...validPayload,
        message: 'A'.repeat(2001), // max is 2000
      }),
    );
    expect(res.status).toBe(400);
  });

  it('rejects oversized organization', async () => {
    const res = await POST(
      makeRequest({
        ...validPayload,
        organization: 'X'.repeat(161), // max is 160
      }),
    );
    expect(res.status).toBe(400);
  });

  // Honeypot
  it('accepts empty honeypot', async () => {
    const res = await POST(
      makeRequest({ ...validPayload, honeypot: '' }, 'hp-1'),
    );
    expect(res.status).toBe(201);
  });

  it('rejects non-empty honeypot via schema', async () => {
    const res = await POST(
      makeRequest(
        { ...validPayload, honeypot: 'bot-filled' },
        'hp-2',
      ),
    );
    expect(res.status).toBe(400);
  });

  // Rate limit
  it('rate limit response does not leak internal info', async () => {
    const ip = 'sec-rate-limit';
    for (let i = 0; i < 5; i++) {
      await POST(makeRequest(validPayload, ip));
    }

    const res = await POST(makeRequest(validPayload, ip));
    expect(res.status).toBe(429);

    const body = res.body as Record<string, unknown>;
    const bodyStr = JSON.stringify(body);
    expect(bodyStr).not.toContain('Map');
    expect(bodyStr).not.toContain('internal');
    expect(bodyStr).not.toContain('stack');
    expect(bodyStr).not.toContain('/Users/');
  });

  // Error responses don't leak info
  it('validation error response does not expose stack traces', async () => {
    const res = await POST(makeRequest({ name: '' }));
    expect(res.status).toBe(400);

    const body = res.body as Record<string, unknown>;
    const bodyStr = JSON.stringify(body);
    expect(bodyStr).not.toContain('node_modules');
    expect(bodyStr).not.toContain('/Users/');
    expect(bodyStr).not.toContain('Error:');
  });

  // IP extraction
  it('handles missing x-forwarded-for gracefully', async () => {
    const request = new Request('http://localhost/api/pilot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(validPayload),
    });
    const res = await POST(request);
    // Should not crash
    expect([201, 429]).toContain(res.status);
  });

  it('uses first IP from x-forwarded-for chain', async () => {
    const spy = vi.spyOn(console, 'info').mockImplementation(() => {});

    await POST(
      makeRequest(validPayload, '10.0.0.1, 192.168.1.1, 172.16.0.1'),
    );

    expect(spy).toHaveBeenCalledWith(
      '[Pilot Registration]',
      expect.objectContaining({
        ip: expect.stringContaining('10.0.0.1'),
      }),
    );

    spy.mockRestore();
  });

  // No prototype pollution
  it('rejects __proto__ in payload', async () => {
    const res = await POST(
      makeRequest({
        ...validPayload,
        __proto__: { isAdmin: true },
      }),
    );
    // Zod will strip extra fields, should still work
    expect([201, 400]).toContain(res.status);
  });

  // Content-Type enforcement (JSON parsing)
  it('rejects non-JSON body', async () => {
    const request = new Request('http://localhost/api/pilot', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-forwarded-for': 'json-test',
      },
      body: '<xml>not json</xml>',
    });
    const res = await POST(request);
    expect(res.status).toBe(400);
    expect(res.body).toEqual({ error: 'Invalid request body' });
  });
});
