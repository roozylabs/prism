import { NextRequest, NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';

function getBackendApiUrl(): string {
  if (process.env.API_URL) {
    return process.env.API_URL.replace(/\/+$/, '');
  }
  if (process.env.NODE_ENV === 'development') {
    return 'http://localhost:8080';
  }
  return 'http://api:8080';
}

async function proxyHandler(request: NextRequest, { params }: { params: Promise<{ path?: string[] }> }) {
  const resolvedParams = await params;
  const pathArray = resolvedParams?.path || [];
  const subPath = pathArray.join('/');
  const searchParams = request.nextUrl.search;

  const backendApiUrl = getBackendApiUrl();
  
  // Clean proxy: map /api/* directly to backendApiUrl/api/*
  const targetUrl = subPath === 'health'
    ? `${backendApiUrl}/health${searchParams}`
    : `${backendApiUrl}/api/${subPath}${searchParams}`;

  const headers = new Headers(request.headers);
  const host = request.headers.get('host') || 'app.prism.roozylabs.com';
  const proto = request.headers.get('x-forwarded-proto') || (request.url.startsWith('https') ? 'https' : 'http');
  headers.set('x-forwarded-host', host);
  headers.set('x-forwarded-proto', proto);
  headers.delete('host');

  const token = request.cookies.get('auth_token')?.value;
  if (token && !headers.has('authorization')) {
    headers.set('authorization', `Bearer ${token}`);
  }

  let body: ArrayBuffer | undefined = undefined;
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method)) {
    try {
      body = await request.arrayBuffer();
    } catch (_bodyErr) {
      body = undefined;
    }
  }

  try {
    const backendResponse = await fetch(targetUrl, {
      method: request.method,
      headers: headers,
      body: body && body.byteLength > 0 ? body : undefined,
      cache: 'no-store',
      redirect: 'manual',
    });

    const responseHeaders = new Headers(backendResponse.headers);
    responseHeaders.delete('content-encoding');
    responseHeaders.delete('content-length');
    responseHeaders.delete('transfer-encoding');
    responseHeaders.delete('set-cookie');

    const setCookies = typeof backendResponse.headers.getSetCookie === 'function'
      ? backendResponse.headers.getSetCookie()
      : [backendResponse.headers.get('set-cookie')].filter(Boolean) as string[];

    // Transparently forward 3xx redirects (e.g. OAuth to Google/GitHub and session callbacks)
    if (backendResponse.status >= 300 && backendResponse.status < 400) {
      const redirectRes = new NextResponse(null, {
        status: backendResponse.status,
        headers: responseHeaders,
      });
      for (const cookieStr of setCookies) {
        redirectRes.headers.append('set-cookie', cookieStr);
      }
      return redirectRes;
    }

    const contentType = responseHeaders.get('content-type') || '';
    const isEventStream =
      contentType.includes('text/event-stream') ||
      contentType.includes('application/x-ndjson') ||
      subPath === 'sse';

    const isNoBodyStatus = backendResponse.status === 204 || backendResponse.status === 304;

    if (isEventStream) {
      responseHeaders.set('Content-Type', 'text/event-stream');
      responseHeaders.set('Cache-Control', 'no-cache, no-transform');
      responseHeaders.set('Connection', 'keep-alive');
      responseHeaders.set('X-Accel-Buffering', 'no');

      const streamBody = backendResponse.body || new ReadableStream({
        start(controller) {
          controller.close();
        }
      });

      const streamRes = new NextResponse(streamBody, {
        status: backendResponse.status,
        statusText: backendResponse.statusText,
        headers: responseHeaders,
      });
      for (const cookieStr of setCookies) {
        streamRes.headers.append('set-cookie', cookieStr);
      }
      return streamRes;
    }

    if (isNoBodyStatus) {
      const noBodyRes = new NextResponse(null, {
        status: backendResponse.status,
        statusText: backendResponse.statusText,
        headers: responseHeaders,
      });
      for (const cookieStr of setCookies) {
        noBodyRes.headers.append('set-cookie', cookieStr);
      }
      return noBodyRes;
    }

    const responseData = await backendResponse.arrayBuffer();
    const normalRes = new NextResponse(responseData, {
      status: backendResponse.status,
      statusText: backendResponse.statusText,
      headers: responseHeaders,
    });
    for (const cookieStr of setCookies) {
      normalRes.headers.append('set-cookie', cookieStr);
    }
    return normalRes;
  } catch (error: unknown) {
    // The backend address and the connection error stay in the server log;
    // the browser gets the same error shape as any other API failure.
    console.error('[API Proxy Error] Failed to proxy to:', targetUrl, error);
    return NextResponse.json(
      { error: { message: 'The API is unavailable. Please try again shortly.', code: 'API_UNAVAILABLE' } },
      { status: 502 }
    );
  }
}

export const GET = proxyHandler;
export const POST = proxyHandler;
export const PUT = proxyHandler;
export const PATCH = proxyHandler;
export const DELETE = proxyHandler;
export const OPTIONS = proxyHandler;
