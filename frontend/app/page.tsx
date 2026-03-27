'use client';

import { FormEvent, useState } from 'react';
import { trace } from '@opentelemetry/api';

const tracer = trace.getTracer('frontend-ui');

type CreateRepoResponse = {
  owner?: string;
  name?: string;
  trace_id?: string;
  [key: string]: unknown;
};

export default function Page() {
  const [owner, setOwner] = useState('alice');
  const [name, setName] = useState('demo-repo');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<CreateRepoResponse | null>(
    null,
  );
  const [error, setError] = useState('');

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError('');

    await tracer.startActiveSpan(
      'ui.create_repo_submit',
      async (span) => {
        span.setAttribute('repo.owner', owner);
        span.setAttribute('repo.name', name);

        try {
          const resp = await fetch('/api/repos', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ owner, name }),
          });

          const data = await resp.json().catch(() => ({}));
          span.setAttribute('http.status_code', resp.status);

          if (!resp.ok) {
            span.setAttribute('error', true);
            setError(`request failed: ${resp.status}`);
            return;
          }

          setResult(data);
        } catch (err) {
          span.recordException(err as Error);
          span.setAttribute('error', true);
          setError(
            err instanceof Error ? err.message : 'unknown error',
          );
        } finally {
          span.end();
          setLoading(false);
        }
      },
    );
  }

  return (
    <main style={{ padding: 24 }}>
      <h1>OTel demo</h1>

      <form
        onSubmit={onSubmit}
        style={{ display: 'grid', gap: 12, maxWidth: 480 }}
      >
        <input
          value={owner}
          onChange={(e) => setOwner(e.target.value)}
          placeholder="owner"
        />
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="repo name"
        />
        <button type="submit" disabled={loading}>
          {loading ? 'Создаем...' : 'Создать repo'}
        </button>
      </form>

      {error ? <p style={{ color: 'tomato' }}>{error}</p> : null}
      {result ? <pre>{JSON.stringify(result, null, 2)}</pre> : null}
    </main>
  );
}
