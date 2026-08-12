import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './App';
import { connectAnnaRuntime } from './services/annaRuntime';

async function bootstrap() {
  const root = ReactDOM.createRoot(document.getElementById('root') as HTMLElement);
  try {
    const anna = await connectAnnaRuntime();
    root.render(<App anna={anna} />);
  } catch (error) {
    root.render(
      <main className="shell">
        <section className="panel">
          <h1>Mini Notes</h1>
          <div className="error" role="alert">Failed to connect Anna App Runtime: {error instanceof Error ? error.message : String(error)}</div>
        </section>
      </main>,
    );
  }
}

bootstrap();
