import { NextRequest, NextResponse } from 'next/server';

export async function POST(req: NextRequest) {
  try {
    const body = await req.json();
    const apiUrl = process.env.API_INTERNAL_URL || 'http://api:8080';

    const traceparent = req.headers.get('traceparent');
    const tracestate = req.headers.get('tracestate');
    const baggage = req.headers.get('baggage');

    const resp = await fetch(`${apiUrl}/repos`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(traceparent ? { traceparent } : {}),
        ...(tracestate ? { tracestate } : {}),
        ...(baggage ? { baggage } : {}),
      },
      body: JSON.stringify(body),
      cache: 'no-store',
    });

    const text = await resp.text();

    return new NextResponse(text, {
      status: resp.status,
      headers: {
        'Content-Type':
          resp.headers.get('Content-Type') || 'application/json',
      },
    });
  } catch (error) {
    return NextResponse.json(
      {
        error: 'frontend_proxy_failed',
        details:
          error instanceof Error ? error.message : 'unknown error',
      },
      { status: 500 },
    );
  }
}
