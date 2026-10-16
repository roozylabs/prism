import { NextRequest, NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';

function getBackendApiUrl(): string {
  const url = process.env.API_URL || 'http://api:8080';
  return url.replace(/\/+$/, '');
}

async function proxyV1Handler(request: NextRequest, { params }: { params: Promise<{ path?: string[] }> }) {
  const resolvedParams = await params;
  const pathArray = resolvedParams?.path || [];
  const subPath = pathArray.join('/');
  const searchParams = request.nextUrl.search;

  const backendApiUrl = getBackendApiUrl();
  const targetUrl = `${backendApiUrl}/v1/${subPath}${searchParams}`;

  const headers = new Headers(request.headers);
  headers.delete('host');

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
    });

    const responseHeaders = new Headers(backendResponse.headers);
    responseHeaders.delete('content-encoding');
    responseHeaders.delete('content-length');

    const contentType = responseHeaders.get('content-type') || '';
    const isEventStream = contentType.includes('text/event-stream');

    if (isEventStream) {
      responseHeaders.set('Content-Type', 'text/event-stream');
      responseHeaders.set('Cache-Control', 'no-cache, no-transform');
      responseHeaders.set('Connection', 'keep-alive');
      responseHeaders.set('X-Accel-Buffering', 'no');

      return new NextResponse(backendResponse.body, {
        status: backendResponse.status,
        statusText: backendResponse.statusText,
        headers: responseHeaders,
      });
    }

    const responseData = await backendResponse.arrayBuffer();
    return new NextResponse(responseData, {
      status: backendResponse.status,
      statusText: backendResponse.statusText,
      headers: responseHeaders,
    });
  } catch (error: unknown) {
    // The backend address and the connection error stay in the server log;
    // clients get an OpenAI-style error like any other gateway failure.
    console.error('[V1 Proxy Error] Failed to proxy to:', targetUrl, error);
    return NextResponse.json(
      { error: { message: 'The gateway is unavailable. Please try again shortly.', type: 'api_error', code: 'gateway_unavailable' } },
      { status: 502 }
    );
  }
}

export const GET = proxyV1Handler;
export const POST = proxyV1Handler;
export const PUT = proxyV1Handler;
export const PATCH = proxyV1Handler;
export const DELETE = proxyV1Handler;
export const OPTIONS = proxyV1Handler;
