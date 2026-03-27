import {
  diag,
  DiagConsoleLogger,
  DiagLogLevel,
  trace,
} from '@opentelemetry/api';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';

declare global {
  interface Window {
    __otelInit?: boolean;
    __otelInitError?: string;
  }
}

if (typeof window !== 'undefined' && !window.__otelInit) {
  try {
    if (process.env.NODE_ENV !== 'production') {
      diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.DEBUG);
    }

    const exporter = new OTLPTraceExporter({
      url: 'http://localhost:4318/v1/traces',
    });

    const provider = new WebTracerProvider({
      resource: resourceFromAttributes({
        [ATTR_SERVICE_NAME]: 'next-frontend-browser',
      }),
      spanProcessors: [
        new BatchSpanProcessor(exporter, {
          scheduledDelayMillis: 2_000,
          maxExportBatchSize: 20,
        }),
      ],
    });

    provider.register();

    registerInstrumentations({
      instrumentations: [
        new DocumentLoadInstrumentation(),
        new FetchInstrumentation({
          ignoreUrls: [/^http:\/\/localhost:4318\/v1\/traces$/],
          propagateTraceHeaderCorsUrls: [
            /^http:\/\/localhost:3001\/api\/repos$/,
            /^http:\/\/localhost:8080\/repos$/,
          ],
        }),
      ],
    });

    trace
      .getTracer('frontend-ui')
      .startSpan('frontend.bootstrap')
      .end();

    window.__otelInit = true;
  } catch (err) {
    window.__otelInitError =
      err instanceof Error ? err.stack || err.message : String(err);
    console.error('OTel browser init failed', err);
  }
}

export function onRouterTransitionStart(url: string) {
  const span = trace
    .getTracer('frontend-ui')
    .startSpan('ui.route_transition');
  span.setAttribute('next.route.target', url);
  span.end();
}
