import { describe, it, expect, vi, beforeEach } from 'vitest';

// Must mock before importing the route
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

describe('Pilot API Route', () => {
  let POST: (request: Request) => Promise<{ body: unknown; status: number }>;

  beforeEach(async () => {
    vi.resetModules();
    const mod = await import('@/app/api/pilot/route');
    POST = mod.POST as typeof POST;
  });

  function makeRequest(body: unknown, ip = '192.168.1.1') {
    return new Request('http://localhost/api/pilot', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-forwarded-for': ip,
      },
      body: JSON.stringify(body),
    });
  }

  it('returns 201 for valid registration', async () => {
    const res = await POST(makeRequest(validPayload));
    expect(res.status).toBe(201);
    expect(res.body).toEqual({
      success: true,
      message: 'Registration received',
    });
  });

  it('returns 400 for invalid body', async () => {
    const res = await POST(makeRequest({ name: 'x' }));
    expect(res.status).toBe(400);
  });

  it('returns 400 for missing required fields', async () => {
    const res = await POST(
      makeRequest({ name: 'Jane', email: 'not-an-email' }),
    );
    expect(res.status).toBe(400);
  });

  it('validates email format', async () => {
    const res = await POST(
      makeRequest({ ...validPayload, email: 'not-email' }),
    );
    expect(res.status).toBe(400);
  });

  it('validates role enum', async () => {
    const res = await POST(
      makeRequest({ ...validPayload, role: 'hacker' }),
    );
    expect(res.status).toBe(400);
  });

  it('validates message minimum length', async () => {
    const res = await POST(
      makeRequest({ ...validPayload, message: 'too short' }),
    );
    expect(res.status).toBe(400);
  });

  it('validates message maximum length', async () => {
    const res = await POST(
      makeRequest({ ...validPayload, message: 'x'.repeat(2001) }),
    );
    expect(res.status).toBe(400);
  });

  it('returns 400 for malformed JSON', async () => {
    const request = new Request('http://localhost/api/pilot', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-forwarded-for': '10.0.0.1',
      },
      body: 'not json',
    });
    const res = await POST(request);
    expect(res.status).toBe(400);
    expect(res.body).toEqual({ error: 'Invalid request body' });
  });

  it('rate limits after 5 requests from same IP', async () => {
    const ip = '10.10.10.10';
    for (let i = 0; i < 5; i++) {
      const res = await POST(makeRequest(validPayload, ip));
      expect(res.status).toBe(201);
    }

    // 6th request should be rate limited
    const res = await POST(makeRequest(validPayload, ip));
    expect(res.status).toBe(429);
    expect(res.body).toEqual({
      error: 'Too many requests. Please try again later.',
    });
  });

  it('rate limits independently per IP', async () => {
    // Exhaust IP A
    for (let i = 0; i < 5; i++) {
      await POST(makeRequest(validPayload, '10.20.30.40'));
    }

    // IP B should still work
    const res = await POST(makeRequest(validPayload, '10.20.30.41'));
    expect(res.status).toBe(201);
  });

  it('logs message field in console.info', async () => {
    const spy = vi.spyOn(console, 'info').mockImplementation(() => {});

    await POST(makeRequest(validPayload, '172.16.0.1'));

    expect(spy).toHaveBeenCalledWith(
      '[Pilot Registration]',
      expect.objectContaining({
        message: validPayload.message,
        name: validPayload.name,
        email: validPayload.email,
      }),
    );

    spy.mockRestore();
  });

  it('truncates IP in logs', async () => {
    const spy = vi.spyOn(console, 'info').mockImplementation(() => {});

    await POST(makeRequest(validPayload, '192.168.100.200'));

    expect(spy).toHaveBeenCalledWith(
      '[Pilot Registration]',
      expect.objectContaining({
        ip: expect.stringContaining('***'),
      }),
    );

    spy.mockRestore();
  });

  it('accepts all valid role values', async () => {
    const roles = [
      'investigator',
      'prosecutor',
      'defense',
      'analyst',
      'court_admin',
      'other',
    ];

    for (const role of roles) {
      const ip = `role-test-${role}`;
      const res = await POST(
        makeRequest({ ...validPayload, role }, ip),
      );
      expect(res.status).toBe(201);
    }
  });

  it('accepts both en and fr locales', async () => {
    const res1 = await POST(
      makeRequest({ ...validPayload, locale: 'en' }, 'locale-1'),
    );
    expect(res1.status).toBe(201);

    const res2 = await POST(
      makeRequest({ ...validPayload, locale: 'fr' }, 'locale-2'),
    );
    expect(res2.status).toBe(201);
  });

  it('rejects invalid locale', async () => {
    const res = await POST(
      makeRequest({ ...validPayload, locale: 'de' }, 'locale-3'),
    );
    expect(res.status).toBe(400);
  });
});
